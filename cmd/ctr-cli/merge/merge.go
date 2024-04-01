package merge

import (
	"context"
	"os"
	"strconv"
	"time"

	"github.com/containerd/containerd/log"
	"github.com/naoki9911/fuse-diff-containerd/pkg/benchmark"
	"github.com/naoki9911/fuse-diff-containerd/pkg/bsdiffx"
	"github.com/naoki9911/fuse-diff-containerd/pkg/image"
	"github.com/naoki9911/fuse-diff-containerd/pkg/utils"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var logger = log.G(context.TODO())

func DimgCommand() *cli.Command {
	cmd := cli.Command{
		Name:  "merge",
		Usage: "merge dimgs",
		Action: func(context *cli.Context) error {
			return dimgAction(context)
		},
		Flags: []cli.Flag{
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
			&cli.BoolFlag{
				Name:     "benchmarkPerFile",
				Usage:    "enable benchmark for files",
				Value:    false,
				Required: false,
			},
			&cli.IntFlag{
				Name:     "threadNum",
				Usage:    "The number of threads to process",
				Value:    1,
				Required: false,
			},
		},
	}

	return &cmd
}

func dimgAction(c *cli.Context) error {
	lowerDimg := c.String("lowerDimg")
	upperDimg := c.String("upperDimg")
	outDimg := c.String("outDimg")
	enableBench := c.Bool("benchmark")
	enableBenchPerFile := c.Bool("benchmarkPerFile")
	threadNum := c.Int("threadNum")
	logger.WithFields(logrus.Fields{
		"lowerDimg": lowerDimg,
		"upperDimg": upperDimg,
		"outDimg":   outDimg,
	}).Info("starting to merge")

	var b *benchmark.Benchmark = nil
	var err error
	if enableBench || enableBenchPerFile {
		b, err = benchmark.NewBenchmark("./benchmark.log")
		if err != nil {
			return err
		}
		defer b.Close()
		b.SetDefaultLabels(utils.ParseLabels(c.StringSlice("labels")))
	}

	start := time.Now()

	mergeFile, err := os.Create(outDimg)
	if err != nil {
		panic(err)
	}
	defer mergeFile.Close()
	mergeConfig := image.MergeConfig{
		ThreadNum:        threadNum,
		BenchmarkPerFile: enableBenchPerFile,
		Benchmarker:      b,
	}
	header, err := image.MergeDimg(lowerDimg, upperDimg, mergeFile, mergeConfig)
	if err != nil {
		panic(err)
	}

	elapsed := time.Since(start)
	stat, err := os.Stat(outDimg)
	if err != nil {
		panic(err)
	}

	if b != nil {
		metric := benchmark.Metric{
			TaskName:     "merge",
			ElapsedMilli: int(elapsed.Milliseconds()),
			Size:         stat.Size(),
			Labels: map[string]string{
				"lowerDimg":       lowerDimg,
				"upperDimg":       upperDimg,
				"outDimg":         outDimg,
				"threadNum":       strconv.Itoa(threadNum),
				"compressionMode": bsdiffx.CompressionModeToString(header.CompressionMode),
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

func CdimgCommand() *cli.Command {
	cmd := cli.Command{
		Name:  "merge",
		Usage: "merge cdimgs",
		Action: func(context *cli.Context) error {
			return cdimgAction(context)
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "lowerCdimg",
				Usage:    "path to lower cdimg",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "upperCdimg",
				Usage:    "path to upper cdimg",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "outCdimg",
				Usage:    "path to merged cdimg",
				Required: true,
			},
			&cli.BoolFlag{
				Name:     "benchmark",
				Usage:    "enable benchmark",
				Value:    false,
				Required: false,
			},
			&cli.BoolFlag{
				Name:     "benchmarkPerFile",
				Usage:    "enable benchmark for files",
				Value:    false,
				Required: false,
			},
			&cli.IntFlag{
				Name:     "threadNum",
				Usage:    "The number of threads to process",
				Value:    1,
				Required: false,
			},
		},
	}

	return &cmd
}

func cdimgAction(c *cli.Context) error {
	lowerCdimg := c.String("lowerCdimg")
	upperCdimg := c.String("upperCdimg")
	outCdimg := c.String("outCdimg")
	enableBench := c.Bool("benchmark")
	enableBenchPerFile := c.Bool("benchmarkPerFile")
	threadNum := c.Int("threadNum")
	logger.WithFields(logrus.Fields{
		"lowerCdimg": lowerCdimg,
		"upperCdimg": upperCdimg,
		"outCdimg":   outCdimg,
	}).Info("starting to merge")

	var b *benchmark.Benchmark = nil
	var err error
	if enableBench || enableBenchPerFile {
		b, err = benchmark.NewBenchmark("./benchmark.log")
		if err != nil {
			return err
		}
		defer b.Close()
		b.SetDefaultLabels(utils.ParseLabels(c.StringSlice("labels")))
	}

	start := time.Now()

	mergeFile, err := os.Create(outCdimg)
	if err != nil {
		panic(err)
	}
	defer mergeFile.Close()
	mergeConfig := image.MergeConfig{
		ThreadNum:        threadNum,
		BenchmarkPerFile: enableBenchPerFile,
		Benchmarker:      b,
	}
	header, err := image.MergeCdimg(lowerCdimg, upperCdimg, mergeFile, mergeConfig)
	if err != nil {
		panic(err)
	}

	elapsed := time.Since(start)
	stat, err := os.Stat(outCdimg)
	if err != nil {
		panic(err)
	}

	if b != nil {
		metric := benchmark.Metric{
			TaskName:     "merge",
			ElapsedMilli: int(elapsed.Milliseconds()),
			Size:         stat.Size(),
			Labels: map[string]string{
				"lowerCdimg":      lowerCdimg,
				"upperCdimg":      upperCdimg,
				"outCdimg":        outCdimg,
				"threadNum":       strconv.Itoa(threadNum),
				"compressionMode": bsdiffx.CompressionModeToString(header.CompressionMode),
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
