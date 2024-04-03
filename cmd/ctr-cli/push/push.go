package push

import (
	"context"
	"fmt"
	"strings"

	"github.com/containerd/containerd/log"
	"github.com/naoki9911/fuse-diff-containerd/pkg/server"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var logger = log.G(context.TODO())

func Command() *cli.Command {
	cmd := cli.Command{
		Name:  "push",
		Usage: "Push image to the server",
		Action: func(context *cli.Context) error {
			return Action(context)
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "serverHost",
				Usage:    "host name for the diff server",
				Value:    "localhost:8081",
				Required: false,
			},
			&cli.StringFlag{
				Name:     "cdimg",
				Usage:    "path to cdimg to push",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "imageTag",
				Usage:    "Tag for the image (e.g. nginx:1.23.1)",
				Required: false,
			},
		},
	}

	return &cmd
}

func Action(c *cli.Context) error {
	logger.Logger.SetLevel(logrus.InfoLevel)
	serverHost := c.String("serverHost")
	cdimg := c.String("cdimg")
	imageTag := c.String("imageTag")
	logger.WithFields(logrus.Fields{
		"serverHost": serverHost,
		"cdimg":      cdimg,
		"imageTag":   imageTag,
	}).Info("starting to push")

	var imgTag *server.ImageTag = nil
	if imageTag != "" {
		ss := strings.SplitN(imageTag, ":", 2)
		switch len(ss) {
		case 1:
			imgTag = &server.ImageTag{
				Name: ss[0],
			}
		case 2:
			imgTag = &server.ImageTag{
				Name:    ss[0],
				Version: ss[1],
			}
		default:
			return fmt.Errorf("invalid format imageTag %s", imageTag)
		}
	}

	client := server.NewDiffClient(serverHost)
	err := client.PushImage(cdimg, imgTag)
	if err != nil {
		return fmt.Errorf("failed to push image: %v", err)
	}

	logger.Info("push done")
	return nil
}
