package convert

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/containerd/containerd/log"
	di3fsImage "github.com/naoki9911/fuse-diff-containerd/pkg/image"
	"github.com/naoki9911/fuse-diff-containerd/pkg/utils"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var logger = log.G(context.TODO())

var Flags = []cli.Flag{
	&cli.StringFlag{
		Name:     "image",
		Usage:    "image name to convert",
		Required: true,
	},
	&cli.StringFlag{
		Name:     "output",
		Usage:    "output path",
		Required: true,
	},
	&cli.BoolFlag{
		Name:     "dimg",
		Usage:    "output dimg image (Root required)",
		Required: false,
	},
	&cli.BoolFlag{
		Name:     "cdimg",
		Usage:    "output cdimg image (Root required)",
		Required: false,
	},
	&cli.StringSliceFlag{
		Name:     "excludes",
		Usage:    "path to exclude from image",
		Required: false,
	},
	&cli.IntFlag{
		Name:     "threadNum",
		Usage:    "The number of threads to process",
		Value:    1,
		Required: false,
	},
}

var workDir = filepath.Join(os.TempDir(), "ctr-cli")

type dockerImageManifest struct {
	Config   string   `json:"Config"`
	RepoTags []string `json:"RepoTags"`
	Layers   []string `json:"Layers"`
}

func decodeDockerImageManifest(manifsetPath string) (*dockerImageManifest, error) {
	f, err := os.Open(manifsetPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open %q : %v", manifsetPath, err)
	}
	c, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("failed to read : %v", err)
	}
	var man []dockerImageManifest
	err = json.Unmarshal(c, &man)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal : %v", err)
	}

	if len(man) != 1 {
		logger.Errorf("invalid manifest: %v", man)
		return nil, fmt.Errorf("invalid manifest")
	}

	return &man[0], err
}

func execCmd(name string, args ...string) {
	cmd := exec.Command(name, args...)
	var stderr bytes.Buffer
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		logger.WithFields(logrus.Fields{
			"name": name,
			"args": args,
		}).Fatalf("failed to exec command : %v\n%v", err, stderr.String())
	}
	logger.Debugf("exec done\n%v", stdout.String())
}

func Action(c *cli.Context) error {
	image := c.String("image")
	outputPath := c.String("output")
	outputDimg := c.Bool("dimg")
	outputCdimg := c.Bool("cdimg")
	threadNum := c.Int("threadNum")

	if outputDimg && os.Geteuid() != 0 {
		return fmt.Errorf("root required")
	}

	imageSquashed := image + "-squashed"
	imageSquashedPath := filepath.Join(workDir, imageSquashed+".tar")
	imageDir := filepath.Join(workDir, image)

	os.RemoveAll(workDir)
	err := os.MkdirAll(workDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to mkdir %s: %v", workDir, err)
	}
	err = os.MkdirAll(outputPath, 0755)
	if err != nil {
		return fmt.Errorf("failed to mkdir %s: %v", outputPath, err)
	}

	logger.Infof("pulling image %q", image)
	execCmd("docker", "pull", image)
	logger.Info("pull done")

	logger.Infof("squashing image %q to %q", image, imageSquashed)
	execCmd("docker-squash", "-t", imageSquashed, image)
	logger.Info("squash done")

	logger.Infof("saving image %q to %q", imageSquashed, imageSquashedPath)
	execCmd("docker", "save", imageSquashed, "-o", imageSquashedPath)
	logger.Info("save done")

	logger.Infof("extracting %q to %q", imageSquashedPath, imageDir)
	err = os.MkdirAll(imageDir, 0755)
	if err != nil {
		return err
	}
	execCmd("tar", "-C", imageDir, "-xvf", imageSquashedPath)
	logger.Info("extract done")

	man, err := decodeDockerImageManifest(filepath.Join(imageDir, "manifest.json"))
	if err != nil {
		return err
	}

	if len(man.Layers) != 1 {
		logger.Errorf("unexpected manifest format \n%v", man)
		return fmt.Errorf("unexpected manifset format")
	}

	layerPath := filepath.Join(imageDir, man.Layers[0])
	layerNewPath := filepath.Join(outputPath, "layer.tar")
	logger.Infof("moving %q to %q as base layer", layerPath, layerNewPath)
	execCmd("mv", layerPath, layerNewPath)
	logger.Info("move done")

	configPath := filepath.Join(imageDir, man.Config)
	configNewPath := filepath.Join(outputPath, "config.json")
	logger.Infof("moving %q to %q as config", configPath, configNewPath)
	execCmd("mv", configPath, configNewPath)
	logger.Info("move done")

	configSize, configDigest, err := utils.GetFileSizeAndDigest(configNewPath)
	if err != nil {
		return fmt.Errorf("failed to get size and digest for %q : %v", configNewPath, err)
	}

	layerSize, layerDigest, err := utils.GetFileSizeAndDigest(layerNewPath)
	if err != nil {
		return fmt.Errorf("failed to get size and digest for %q : %v", configNewPath, err)
	}

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
				Size:      layerSize,
				Digest:    *layerDigest,
			},
		},
	}
	manifest.SchemaVersion = 2
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	manifestPath := filepath.Join(outputPath, "manifest.json")
	manifestFile, err := os.Create(manifestPath)
	if err != nil {
		return err
	}
	defer manifestFile.Close()
	_, err = manifestFile.Write(manifestBytes)
	if err != nil {
		return err
	}

	logger.Infof("manifest is written to %q", manifestPath)

	if !(outputDimg || outputCdimg) {
		return nil
	}

	// convert layer.tar to dimg
	tempDir, err := os.MkdirTemp("/tmp/ctr-cli", "*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	logger.Infof("extracting layer(%s) to %s", layerNewPath, tempDir)
	err = exec.Command("tar", "-xf", layerNewPath, "-C", tempDir).Run()
	if err != nil {
		return fmt.Errorf("failed to extract layer(%s) to %s: %v", layerNewPath, tempDir, err)
	}

	excludes := c.StringSlice("excludes")
	for _, exclude := range excludes {
		p := filepath.Join(tempDir, exclude)
		err = os.RemoveAll(p)
		if err != nil {
			return fmt.Errorf("failed to exclude %s (path=%s)", exclude, p)
		}
	}

	tempDiffDir, err := os.MkdirTemp("/tmp/ctr-cli", "*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDiffDir)

	logger.Info("packing dimg")
	dimgPath := filepath.Join(outputPath, "image.dimg")
	err = di3fsImage.PackDir(tempDir, dimgPath, threadNum)
	if err != nil {
		return fmt.Errorf("failed to pack dimg: %v", err)
	}

	logger.Infof("successfully packed dimg image %s to %s", image, dimgPath)
	if outputDimg && !outputCdimg {
		return nil
	}

	logger.Info("packing cdimg")
	cdimgPath := filepath.Join(outputPath, "image.cdimg")
	err = di3fsImage.PackCdimg(configNewPath, dimgPath, cdimgPath)
	if err != nil {
		return fmt.Errorf("failed to pack cdimg: %v", err)
	}

	logger.Infof("successfully packed cdimg image %s to %s", image, cdimgPath)
	return nil
}

func Command() *cli.Command {
	cmd := cli.Command{
		Name:  "convert",
		Usage: "Convert contaier image into squashed image",
		Action: func(context *cli.Context) error {
			return Action(context)
		},
		Flags: Flags,
	}

	return &cmd
}
