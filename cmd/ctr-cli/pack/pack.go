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
			Usage:    "output cdimg name",
			Required: true,
		},
	}
)

func Action(c *cli.Context) error {
	logger.Logger.SetLevel(logrus.DebugLevel)
	configPath := c.String("config")
	dimgPath := c.String("dimg")
	outPath := c.String("out")
	logger.WithFields(logrus.Fields{
		"configPath": configPath,
		"dimg":       dimgPath,
		"outPath":    outPath,
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

func PackDimgCommand() *cli.Command {
	cmd := &cli.Command{
		Name:  "pack",
		Usage: "pack dir into dimg",
		Action: func(context *cli.Context) error {
			return packDimgAction(context)
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "in",
				Usage:    "path to packed dir",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "out",
				Usage:    "output digm path",
				Required: true,
			},
			&cli.IntFlag{
				Name:     "threadNum",
				Usage:    "the number of thread",
				Required: false,
				Value:    8,
			},
		},
	}
	return cmd
}

func packDimgAction(c *cli.Context) error {
	inPath := c.String("in")
	outPath := c.String("out")
	threadNum := c.Int("threadNum")
	err := image.PackDir(inPath, outPath, threadNum)
	if err != nil {
		return err
	}

	return nil
}
