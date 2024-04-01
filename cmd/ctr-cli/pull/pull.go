package pull

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/containerd/containerd/log"
	"github.com/naoki9911/fuse-diff-containerd/cmd/ctr-cli/load"
	"github.com/naoki9911/fuse-diff-containerd/pkg/benchmark"
	"github.com/naoki9911/fuse-diff-containerd/pkg/image"
	sns "github.com/naoki9911/fuse-diff-containerd/pkg/snapshotter"
	"github.com/naoki9911/fuse-diff-containerd/pkg/update"
	"github.com/naoki9911/fuse-diff-containerd/pkg/utils"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var logger = log.G(context.TODO())

var (
	Flags = []cli.Flag{
		&cli.StringFlag{
			Name:     "image",
			Usage:    "image to be pulled",
			Required: true,
		},
		&cli.BoolFlag{
			Name:     "benchmark",
			Usage:    "enable benchmark",
			Required: false,
		},
		&cli.StringFlag{
			Name:     "host",
			Usage:    "server host",
			Required: true,
		},
	}
)

func Action(c *cli.Context) error {
	logger.Logger.SetLevel(logrus.InfoLevel)
	imageName := c.String("image")
	host := c.String("host")
	benchmark := c.Bool("benchmark")
	logger.WithFields(logrus.Fields{
		"imageName": imageName,
		"host":      host,
	}).Info("starting to pull")

	err := pullImage(c, host, imageName, benchmark)
	if err != nil {
		return err
	}

	logger.Info("pull done")
	return nil
}

func Command() *cli.Command {
	cmd := cli.Command{
		Name:  "pull",
		Usage: "Pull image",
		Action: func(context *cli.Context) error {
			return Action(context)
		},
		Flags: Flags,
	}

	return &cmd
}

func pullImage(c *cli.Context, host string, imageNameWithVersion string, bench bool) error {
	var b *benchmark.Benchmark = nil
	var err error
	if bench {
		b, err = benchmark.NewBenchmark("./benchmark.log")
		if err != nil {
			return err
		}
		defer b.Close()
	}
	start := time.Now()
	snClient, err := sns.NewClient()
	if err != nil {
		return err
	}

	reqImgNames := strings.Split(imageNameWithVersion, ":")
	if len(reqImgNames) != 2 {
		return fmt.Errorf("invalid image name %s", imageNameWithVersion)
	}
	reqImgName := reqImgNames[0]
	reqImgVersion := reqImgNames[1]

	imgStore := snClient.CtrClient.ImageService()
	images, err := imgStore.List(context.TODO())
	if err != nil {
		return err
	}

	localImages := make([]update.Image, 0)
	for _, img := range images {
		targetSns, ok := img.Labels[sns.TargetSnapshotLabel]
		if !ok {
			continue
		}
		if targetSns != "di3fs" {
			continue
		}
		localImgName := img.Labels[sns.SnapshotLabelImageName]
		localImgVersion := img.Labels[sns.SnapshotLabelImageVersion]

		// if the requested image exists local, nothing to do.
		if localImgName == reqImgName && localImgVersion == reqImgVersion {
			logger.Infof("%s is already pulled", imageNameWithVersion)
			return nil
		}

		localImg := update.Image{
			Name:    localImgName,
			Version: localImgVersion,
		}
		localImages = append(localImages, localImg)
	}
	logger.WithField("localImages", localImages).Debug("local images collected")

	reqBody := update.UpdateDataRequest{
		RequestImage: update.Image{
			Name:    reqImgName,
			Version: reqImgVersion,
		},
		LocalImages: localImages,
	}

	reqBodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}
	client := &http.Client{}
	logger.WithField("reqBody", string(reqBodyBytes)).Debug("request update")
	req, err := http.NewRequest("GET", "http://"+host+"/update", bytes.NewBuffer(reqBodyBytes))
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	logger.Debug("received response")

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to pull: server status=%d", resp.StatusCode)
	}
	resJsonLength, err := strconv.Atoi(resp.Header.Get("Update-Response-Length"))
	if err != nil {
		return err
	}
	resJsonBytes := make([]byte, resJsonLength)
	readSize, err := resp.Body.Read(resJsonBytes)
	if err != nil {
		return err
	}
	if resJsonLength != int(readSize) {
		return fmt.Errorf("invalid length response expected=%d actual=%d", resJsonLength, readSize)
	}
	var resJson update.UpdateDataResponse
	err = json.Unmarshal(resJsonBytes, &resJson)
	if err != nil {
		return err
	}
	logger.Infof("recieved response imageName=%s Version=%s baseVersion=%s", resJson.Name, resJson.Version, resJson.BaseVersion)

	header, _, err := image.LoadCdimgHeader(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to load cdimg header: %v", err)
	}
	logger.WithField("dimgSize", header.Head.DimgSize).Debug("got image header")

	dimgPath := filepath.Join(os.TempDir(), utils.GetRandomId("d4c-snapshotter")+".dimg")
	dimgFile, err := os.Create(dimgPath)
	if err != nil {
		return fmt.Errorf("failed to create dimg at %s: %v", dimgPath, err)
	}
	dimgSize, err := io.Copy(dimgFile, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to copy dimg: %v", err)
	}
	if dimgSize != header.Head.DimgSize {
		return fmt.Errorf("invalid dimg (expected=%d actual=%d)", header.Head.DimgSize, dimgSize)
	}
	logger.WithField("dimgPath", dimgPath).Info("dimg saved")

	if b != nil {
		metricDownload := benchmark.Metric{
			TaskName:     "pull-download",
			ElapsedMilli: int(time.Since(start).Milliseconds()),
			Labels: map[string]string{
				"imageName":   resJson.Name,
				"version":     resJson.Version,
				"baseVersion": resJson.BaseVersion,
			},
		}
		metricDownload.AddLabels(utils.ParseLabels(c.StringSlice("labels")))
		err = b.AppendResult(metricDownload)
		if err != nil {
			return err
		}
	}

	err = load.LoadImage(snClient, context.TODO(), reqImgName, reqImgVersion, header, dimgPath)
	if err != nil {
		return fmt.Errorf("failed to load image: %v", err)
	}
	if b != nil {
		metricDownload := benchmark.Metric{
			TaskName:     "pull",
			ElapsedMilli: int(time.Since(start).Milliseconds()),
			Labels: map[string]string{
				"imageName":   resJson.Name,
				"version":     resJson.Version,
				"baseVersion": resJson.BaseVersion,
			},
		}
		metricDownload.AddLabels(utils.ParseLabels(c.StringSlice("labels")))
		err = b.AppendResult(metricDownload)
		if err != nil {
			return err
		}
	}

	return nil
}
