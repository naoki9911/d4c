package convert2

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/containerd/containerd/log"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/naoki9911/fuse-diff-containerd/pkg/image"
	"github.com/naoki9911/fuse-diff-containerd/pkg/oci"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var logger = log.G(context.TODO())

func Command() *cli.Command {
	cmd := cli.Command{
		Name:  "convert2",
		Usage: "Convert contaier image into squashed image",
		Action: func(context *cli.Context) error {
			return action(context)
		},
		Flags: Flags,
	}

	return &cmd
}

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
	&cli.StringFlag{
		Name:     "os",
		Usage:    "Platform.OS",
		Value:    "linux",
		Required: false,
	},
	&cli.StringFlag{
		Name:     "arch",
		Usage:    "Platform.Architecture",
		Value:    "amd64",
		Required: false,
	},
	&cli.BoolFlag{
		Name:     "dimg",
		Usage:    "output dimg image (Root required)",
		Value:    true,
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
	&cli.BoolFlag{
		Name:  "load",
		Usage: "Load image tarball",
		Value: false,
	},
}

func action(c *cli.Context) error {
	outputPath := c.String("output")
	img := c.String("image")
	OS := c.String("os")
	arch := c.String("arch")
	outputCdimg := c.Bool("cdimg")
	threadNum := c.Int("threadNum")
	puller, err := oci.NewPuller()
	if err != nil {
		return fmt.Errorf("failed to create puller: %v", err)
	}
	var layer v1.Layer
	var config *v1.ConfigFile
	if c.Bool("load") {
		logger.WithFields(logrus.Fields{"image": img, "os": OS, "arch": arch}).Info("started to load image")
		layer, config, err = puller.Load(img, OS, arch)
		if err != nil {
			return fmt.Errorf("failed to load image %s: %v", img, err)
		}
	} else {
		logger.WithFields(logrus.Fields{"image": img, "os": OS, "arch": arch}).Info("started to pull image")
		layer, config, err = puller.Pull(img, OS, arch)
		if err != nil {
			return fmt.Errorf("failed to pull image %s: %v", img, err)
		}
	}

	uncompLayer, err := layer.Uncompressed()
	if err != nil {
		return fmt.Errorf("failed to get uncompressed layer: %v", err)
	}
	defer uncompLayer.Close()
	logger.Infof("pull done")
	err = os.MkdirAll(outputPath, 0755)
	if err != nil {
		return fmt.Errorf("failed to create output dir: %v", err)
	}

	layerPath := filepath.Join(outputPath, "layer.tar")
	layerFile, err := os.Create(layerPath)
	if err != nil {
		return fmt.Errorf("failed to create layer file %s: %v", layerPath, err)
	}
	defer layerFile.Close()

	_, err = io.Copy(layerFile, uncompLayer)
	if err != nil {
		return fmt.Errorf("failed to copy layer: %v", err)
	}

	configPath := filepath.Join(outputPath, "config.json")
	configFile, err := os.Create(configPath)
	if err != nil {
		return fmt.Errorf("failed to create config file: %v", err)
	}
	defer configFile.Close()
	configBytes, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %v", err)
	}
	_, err = configFile.Write(configBytes)
	if err != nil {
		return fmt.Errorf("failed to write config: %v", err)
	}
	err = configFile.Sync()
	if err != nil {
		return fmt.Errorf("failed to sync config file: %v", err)
	}

	dimgPath := filepath.Join(outputPath, "image.dimg")
	err = image.PackLayer(layer, dimgPath, threadNum)
	if err != nil {
		return fmt.Errorf("failed to pack layer: %v", err)
	}

	if outputCdimg {
		err = image.PackCdimg(configPath, dimgPath, filepath.Join(outputPath, "image.cdimg"))
		if err != nil {
			return fmt.Errorf("failed to pack cdimg: %v", err)
		}
	}

	return nil
}
