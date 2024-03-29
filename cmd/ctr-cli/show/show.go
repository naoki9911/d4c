package show

import (
	"context"
	"fmt"

	"github.com/containerd/containerd/log"
	"github.com/naoki9911/fuse-diff-containerd/pkg/image"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var logger = log.G(context.TODO())

func DimgCommand() *cli.Command {
	cmd := cli.Command{
		Name:  "show",
		Usage: "show dimg info",
		Action: func(context *cli.Context) error {
			return dimgAction(context)
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "dimg",
				Usage:    "path to dimg",
				Required: true,
			},
		},
	}

	return &cmd
}

func dimgAction(c *cli.Context) error {
	logger.Logger.SetLevel(logrus.WarnLevel)
	dimgPath := c.String("dimg")

	dimgFile, err := image.OpenDimgFile(dimgPath)
	if err != nil {
		return err
	}
	defer dimgFile.Close()
	header := dimgFile.Header()
	fmt.Printf("ID: %s\n", header.Id)
	fmt.Printf("ParentID: %s\n", header.ParentId)
	return nil
}

func CdimgCommand() *cli.Command {
	cmd := cli.Command{
		Name:  "show",
		Usage: "show cdimg info",
		Action: func(context *cli.Context) error {
			return cdimgAction(context)
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "cdimg",
				Usage:    "path to cdimg",
				Required: true,
			},
		},
	}

	return &cmd
}

func cdimgAction(c *cli.Context) error {
	logger.Logger.SetLevel(logrus.WarnLevel)
	cdimgPath := c.String("cdimg")
	cdimgFile, err := image.OpenCdimgFile(cdimgPath)
	if err != nil {
		return err
	}
	defer cdimgFile.Close()
	header := cdimgFile.Header
	head := header.Head

	fmt.Printf("Manifest Digest: %s\n", head.ManifestDigest)
	fmt.Printf("Manifest: %v\n", header.Manifest)
	fmt.Printf("Config: %v\n", header.Config)
	fmt.Printf("DimgDigest: %s\n", header.DimgDigest)
	fmt.Printf("DimgID: %s\n", cdimgFile.Dimg.Header().Id)
	fmt.Printf("DimgParentID: %s\n", cdimgFile.Dimg.Header().ParentId)

	return nil
}
