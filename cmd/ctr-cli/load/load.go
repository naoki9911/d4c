package load

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/log"
	"github.com/naoki9911/fuse-diff-containerd/pkg/image"
	sns "github.com/naoki9911/fuse-diff-containerd/pkg/snapshotter"
	"github.com/naoki9911/fuse-diff-containerd/pkg/utils"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var Flags = []cli.Flag{
	&cli.StringFlag{
		Name:     "image",
		Usage:    "image name to be loaded",
		Required: true,
	},
	&cli.StringFlag{
		Name:     "cdimg",
		Usage:    "path to cdimg to be loaded",
		Required: true,
	},
}

func LoadImage(snClient *sns.Client, ctx context.Context, imageName, imageVersion string, imageHeader *image.CdimgHeader, dimgPath string) error {
	cs := snClient.CtrClient.ContentStore()

	configSize, configDigest, err := utils.GetSizeAndDigest(imageHeader.ConfigBytes)
	if err != nil {
		return err
	}

	dimgId := imageHeader.Config.RootFS.DiffIDs[0]

	manifest := v1.Manifest{
		MediaType: v1.MediaTypeImageManifest,
		Config: v1.Descriptor{
			MediaType: v1.MediaTypeImageConfig,
			Size:      configSize,
			Digest:    *configDigest,
		},
		Layers: []v1.Descriptor{
			{
				MediaType: v1.MediaTypeImageLayer,
				Size:      imageHeader.Head.DimgSize,
				Digest:    dimgId,
			},
		},
	}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %v", err)
	}
	_, manifestDigest, err := utils.GetSizeAndDigest(manifestBytes)
	if err != nil {
		return fmt.Errorf("failed to get digest for manifest: %v", err)
	}

	err = cs.Delete(ctx, *manifestDigest)
	if err != nil {
		log.G(ctx).Infof("%s is already removed: %v", manifestDigest, err)
	}

	err = content.WriteBlob(ctx, cs, manifestDigest.Hex(), bytes.NewReader(manifestBytes),
		v1.Descriptor{
			Size:   int64(len(manifestBytes)),
			Digest: *manifestDigest,
		},
		content.WithLabels(map[string]string{
			sns.NerverGC:         "hoghoge",
			sns.ImageLabelPuller: "di3fs",
			//fmt.Sprintf("%s.config", sns.ContentLabelContainerdGC): imageHeader.Manifest.Config.Digest.String(),
			//fmt.Sprintf("%s.di3fs", ContentLabelContainerdGC):  dId.String(),
		}),
	)
	if err != nil {
		return err
	}
	log.G(ctx).Debug("load manifest done")

	err = content.WriteBlob(
		ctx, cs, configDigest.Hex(), bytes.NewReader(imageHeader.ConfigBytes),
		v1.Descriptor{
			Size:   int64(len(imageHeader.ConfigBytes)),
			Digest: *configDigest,
		},
		content.WithLabels(map[string]string{
			sns.NerverGC: "hoghoge",
		}),
	)
	if err != nil {
		return err
	}
	log.G(ctx).Debug("load config done")

	// register image
	is := snClient.CtrClient.ImageService()
	_, err = is.Create(ctx, images.Image{
		Name: imageName + ":" + imageVersion,
		Target: v1.Descriptor{
			MediaType: sns.ImageMediaTypeManifestV2,
			Digest:    *manifestDigest,
			Size:      int64(len(manifestBytes)),
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Labels: map[string]string{
			sns.TargetSnapshotLabel:       "di3fs",
			sns.SnapshotLabelImageName:    imageName,
			sns.SnapshotLabelImageVersion: imageVersion,
		},
	})
	if err != nil {
		return err
	}

	// now ready to create snapshot
	err = sns.CreateSnapshot(ctx, snClient.SnClient, *manifestDigest, dimgId, imageName+":"+imageVersion, dimgPath)
	if err != nil {
		return err
	}

	log.G(ctx).WithFields(logrus.Fields{
		"header":   imageHeader,
		"manifest": manifest,
		"config":   imageHeader.Config,
	}).Debugf("image loaded")

	return nil
}

func Load(ctx context.Context, imgNameWithVersion, imgPath string) error {
	snClient, err := sns.NewClient()
	if err != nil {
		return err
	}

	imgNames := strings.Split(imgNameWithVersion, ":")
	if len(imgNames) != 2 {
		return fmt.Errorf("invalid image name %s", imgNameWithVersion)
	}
	imgName := imgNames[0]
	imgVersion := imgNames[1]
	log.G(ctx).WithFields(logrus.Fields{"imageName": imgName, "imageVersion": imgVersion}).Infof("loading image from %q", imgPath)
	// load image
	image, err := image.OpenCdimgFile(imgPath)
	if err != nil {
		return err
	}
	defer image.Close()
	log.G(ctx).Info("loaded image")

	// extract dimg
	dimgPath := filepath.Join(os.TempDir(), utils.GetRandomId("d4c-snapshotter")+".dimg")
	dimgFile, err := os.Create(dimgPath)
	if err != nil {
		return err
	}
	defer dimgFile.Close()
	err = image.WriteDimg(dimgFile)
	if err != nil {
		return err
	}
	// LaodImage use written dimg. so close here.
	dimgFile.Close()

	err = LoadImage(snClient, ctx, imgName, imgVersion, image.Header, dimgPath)
	if err != nil {
		return err
	}

	return nil
}

func Action(c *cli.Context) error {
	imgName := c.String("image")
	imgPath := c.String("cdimg")
	err := Load(context.TODO(), imgName, imgPath)
	if err != nil {
		return err
	}
	return nil
}

func Command() *cli.Command {
	cmd := cli.Command{
		Name:  "load",
		Usage: "Load dimg",
		Action: func(context *cli.Context) error {
			return Action(context)
		},
		Flags: Flags,
	}

	return &cmd
}
