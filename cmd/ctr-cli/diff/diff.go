package diff

import (
	"context"
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
			Name:     "oldDimg",
			Usage:    "path to old base dimg",
			Required: true,
		},
		&cli.StringFlag{
			Name:     "newDimg",
			Usage:    "path to new base dimg",
			Required: true,
		},
		&cli.StringFlag{
			Name:     "outDimg",
			Usage:    "path to merged dimg",
			Required: true,
		},
		&cli.StringFlag{
			Name:     "mode",
			Usage:    "path to merged dimg",
			Required: false,
			Value:    ModeDiffBinary,
		},
		&cli.BoolFlag{
			Name:     "benchmark",
			Usage:    "enable benchmark",
			Value:    false,
			Required: false,
		},
	}
)

const ModeDiffBinary = "binary-diff"
const ModeDiffFile = "file-diff"

func Action(c *cli.Context) error {
	logger.Logger.SetLevel(logrus.WarnLevel)
	oldDimg := c.String("oldDimg")
	newDimg := c.String("newDimg")
	outDimg := c.String("outDimg")
	mode := c.String("mode")
	enableBench := c.Bool("benchmark")
	logger.WithFields(logrus.Fields{
		"oldDimg": oldDimg,
		"newDimg": newDimg,
		"outDimg": outDimg,
		"mode":    mode,
	}).Info("starting to diff")

	if mode != ModeDiffBinary && mode != ModeDiffFile {
		logger.Fatalf("mode '%s' does not exist. only 'binary-diff' or 'file-diff' is allowed", mode)
	}

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

	err = image.GenerateDiffFromDimg(oldDimg, newDimg, outDimg, mode == ModeDiffBinary)
	if err != nil {
		panic(err)
	}

	if b != nil {
		metric := benchmark.Metric{
			TaskName:     "patch",
			ElapsedMilli: int(time.Since(start).Milliseconds()),
			Labels: []string{
				"oldDimg" + oldDimg,
				"newDimg" + newDimg,
				"outDimg" + outDimg,
				"mode" + mode,
			},
		}
		err = b.AppendResult(metric)
		if err != nil {
			panic(err)
		}
	}
	logger.Info("diff done")
	return nil
}

func Command() *cli.Command {
	cmd := cli.Command{
		Name:  "diff",
		Usage: "generate diff dimg between dimgs",
		Action: func(context *cli.Context) error {
			return Action(context)
		},
		Flags: Flags,
	}

	return &cmd
}
