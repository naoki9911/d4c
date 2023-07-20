package pack

import (
	"context"

	"github.com/containerd/containerd/log"
	"github.com/naoki9911/fuse-diff-containerd/pkg/image"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var logger = log.G(context.TODO())

var (
	Flags = []cli.Flag{
		&cli.StringFlag{
			Name:     "manifest",
			Usage:    "path to manifest file",
			Required: true,
		},
		&cli.StringFlag{
			Name:     "config",
			Usage:    "path to config file",
			Required: true,
		},
		&cli.StringFlag{
			Name:     "dimg",
			Usage:    "path to dimg",
			Required: true,
		},
		&cli.StringFlag{
			Name:     "out",
			Usage:    "output file name",
			Required: true,
		},
	}
)

func Action(c *cli.Context) error {
	logger.Logger.SetLevel(logrus.DebugLevel)
	manifestPath := c.String("manifest")
	configPath := c.String("config")
	dimgPath := c.String("dimg")
	outPath := c.String("out")
	logger.WithFields(logrus.Fields{
		"manifestPath": manifestPath,
		"configPath":   configPath,
		"dimg":         dimgPath,
		"outPath":      outPath,
	}).Info("starting to pack")

	err := image.PackCdimg(configPath, dimgPath, outPath)
	if err != nil {
		return err
	}
	logger.Info("pack done")
	return nil
}

func Command() *cli.Command {
	cmd := cli.Command{
		Name:  "pack",
		Usage: "Pack diffs into distributable form",
		Action: func(context *cli.Context) error {
			return Action(context)
		},
		Flags: Flags,
	}

	return &cmd
}
