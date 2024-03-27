package patch

import (
	"context"
	"os"
	"time"

	"github.com/containerd/containerd/log"
	"github.com/naoki9911/fuse-diff-containerd/pkg/benchmark"
	"github.com/naoki9911/fuse-diff-containerd/pkg/image"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var logger = log.G(context.TODO())

var (
	Flags = []cli.Flag{
		&cli.StringFlag{
			Name:     "baseDir",
			Usage:    "path to patch base directory",
			Required: false,
		},
		&cli.StringFlag{
			Name:     "outDir",
			Usage:    "path to output directory",
			Required: true,
		},
		&cli.StringFlag{
			Name:     "diffDimg",
			Usage:    "path to diff dimg",
			Required: true,
		},
		&cli.BoolFlag{
			Name:     "benchmark",
			Usage:    "enable benchmark",
			Value:    false,
			Required: false,
		},
	}
)

func Action(c *cli.Context) error {
	logger.Logger.SetLevel(logrus.WarnLevel)
	baseDir := c.String("baseDir")
	outDir := c.String("outDir")
	diffDimg := c.String("diffDimg")
	enableBench := c.Bool("benchmark")
	logger.WithFields(logrus.Fields{
		"baseDir":  baseDir,
		"outDir":   outDir,
		"diffDimg": diffDimg,
	}).Info("starting to patch")

	os.RemoveAll(outDir)

	var b *benchmark.Benchmark = nil
	var err error
	if enableBench {
		b, err = benchmark.NewBenchmark("./benchmark.log")
		if err != nil {
			return err
		}
		defer b.Close()
	}

	start := time.Now()
	imageFile, err := image.OpenDimgFile(diffDimg)
	if err != nil {
		panic(err)
	}
	imageHeader := imageFile.Header()
	err = image.ApplyPatch(baseDir, outDir, &imageHeader.FileEntry, imageFile, imageHeader.BaseId == "")
	if err != nil {
		panic(err)
	}
	if b != nil {
		metric := benchmark.Metric{
			TaskName:     "patch",
			ElapsedMilli: int(time.Since(start).Milliseconds()),
			Labels: []string{
				"baseDir:" + baseDir,
				"outDir:" + outDir,
				"diffDimg:" + diffDimg,
			},
		}
		err = b.AppendResult(metric)
		if err != nil {
			panic(err)
		}
	}
	logger.Info("patch done")
	return nil
}

func Command() *cli.Command {
	cmd := cli.Command{
		Name:  "patch",
		Usage: "patch diff dimg to directory",
		Action: func(context *cli.Context) error {
			return Action(context)
		},
		Flags: Flags,
	}

	return &cmd
}
