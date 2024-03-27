package merge

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
			Name:     "lowerDimg",
			Usage:    "path to lower dimg",
			Required: true,
		},
		&cli.StringFlag{
			Name:     "upperDimg",
			Usage:    "path to upper dimg",
			Required: true,
		},
		&cli.StringFlag{
			Name:     "outDimg",
			Usage:    "path to merged dimg",
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
	lowerDimg := c.String("lowerDimg")
	upperDimg := c.String("upperDimg")
	outDimg := c.String("outDimg")
	enableBench := c.Bool("benchmark")
	logger.WithFields(logrus.Fields{
		"lowerDimg": lowerDimg,
		"upperDimg": upperDimg,
		"outDimg":   outDimg,
	}).Info("starting to merge")

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

	mergeFile, err := os.Create(outDimg)
	if err != nil {
		panic(err)
	}
	defer mergeFile.Close()
	err = image.MergeDimg(lowerDimg, upperDimg, mergeFile)
	if err != nil {
		panic(err)
	}

	if b != nil {
		metric := benchmark.Metric{
			TaskName:     "patch",
			ElapsedMilli: int(time.Since(start).Milliseconds()),
			Labels: []string{
				"lowerDimg:" + lowerDimg,
				"upperDimg:" + upperDimg,
				"outDimg:" + outDimg,
			},
		}
		err = b.AppendResult(metric)
		if err != nil {
			panic(err)
		}
	}
	logger.Info("merge done")
	return nil
}

func Command() *cli.Command {
	cmd := cli.Command{
		Name:  "merge",
		Usage: "merge dimgs",
		Action: func(context *cli.Context) error {
			return Action(context)
		},
		Flags: Flags,
	}

	return &cmd
}
