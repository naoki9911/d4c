package diff

import (
	"context"
	"os"
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

const ModeDiffBinary = "binary-diff"
const ModeDiffFile = "file-diff"

func DimgCommand() *cli.Command {
	cmd := cli.Command{
		Name:  "diff",
		Usage: "generate diff dimg between dimgs",
		Action: func(context *cli.Context) error {
			return dimgAction(context)
		},
		Flags: []cli.Flag{
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
				Usage:    "diff generating mdoe",
				Required: false,
				Value:    ModeDiffBinary,
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
			&cli.StringFlag{
				Name:     "threadSchedMode",
				Usage:    "Multithread scheduling mode",
				Value:    "none",
				Required: false,
			},
			&cli.StringFlag{
				Name:     "compressionMode",
				Usage:    "Mode to compress diffs",
				Value:    "bzip2",
				Required: false,
			},
		},
	}

	return &cmd
}

func dimgAction(c *cli.Context) error {
	oldDimg := c.String("oldDimg")
	newDimg := c.String("newDimg")
	outDimg := c.String("outDimg")
	mode := c.String("mode")
	threadNum := c.Int("threadNum")
	threadSchedMode := c.String("threadSchedMode")
	enableBench := c.Bool("benchmark")
	enableBenchPerFile := c.Bool("benchmarkPerFile")
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
	if enableBench || enableBenchPerFile {
		b, err = benchmark.NewBenchmark("./benchmark.log")
		if err != nil {
			return err
		}
		defer b.Close()
		b.SetDefaultLabels(utils.ParseLabels(c.StringSlice("labels")))
	}

	start := time.Now()

	compressionMode := c.String("compressionMode")
	compMode, err := bsdiffx.GetCompressMode(compressionMode)
	if err != nil {
		return err
	}

	dc := image.DiffConfig{
		ThreadNum:        threadNum,
		ScheduleMode:     threadSchedMode,
		CompressionMode:  compMode,
		BenchmarkPerFile: enableBenchPerFile,
		Benchmarker:      b,
	}
	err = image.GenerateDiffFromDimg(oldDimg, newDimg, outDimg, mode == ModeDiffBinary, dc)
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
			TaskName:     "diff",
			ElapsedMilli: int(elapsed.Milliseconds()),
			Size:         stat.Size(),
			Labels: map[string]string{
				"oldDimg": oldDimg,
				"newDimg": newDimg,
				"outDimg": outDimg,
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

func CdimgCommand() *cli.Command {
	cmd := cli.Command{
		Name:  "diff",
		Usage: "generate diff cdimg between dimgs",
		Action: func(context *cli.Context) error {
			return cdimgAction(context)
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "oldCdimg",
				Usage:    "path to old base dimg",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "newCdimg",
				Usage:    "path to new base dimg",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "outCdimg",
				Usage:    "path to merged dimg",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "mode",
				Usage:    "diff generating mdoe",
				Required: false,
				Value:    ModeDiffBinary,
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
			&cli.StringFlag{
				Name:     "threadSchedMode",
				Usage:    "Multithread scheduling mode",
				Value:    "none",
				Required: false,
			},
			&cli.StringFlag{
				Name:     "compressionMode",
				Usage:    "Mode to compress diffs",
				Value:    "bzip2",
				Required: false,
			},
		},
	}

	return &cmd
}

func cdimgAction(c *cli.Context) error {
	oldCdimg := c.String("oldCdimg")
	newCdimg := c.String("newCdimg")
	outCdimg := c.String("outCdimg")
	mode := c.String("mode")
	enableBench := c.Bool("benchmark")
	enableBenchPerFile := c.Bool("benchmarkPerFile")
	threadNum := c.Int("threadNum")
	threadSchedMode := c.String("threadSchedMode")
	logger.WithFields(logrus.Fields{
		"oldCdimg": oldCdimg,
		"newCdimg": newCdimg,
		"outCdimg": outCdimg,
		"mode":     mode,
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
		b.SetDefaultLabels(utils.ParseLabels(c.StringSlice("labels")))
	}

	start := time.Now()

	compressionMode := c.String("compressionMode")
	compMode, err := bsdiffx.GetCompressMode(compressionMode)
	if err != nil {
		return err
	}

	dc := image.DiffConfig{
		ThreadNum:        threadNum,
		ScheduleMode:     threadSchedMode,
		CompressionMode:  compMode,
		BenchmarkPerFile: enableBenchPerFile,
		Benchmarker:      b,
	}
	err = image.GenerateDiffFromCdimg(oldCdimg, newCdimg, outCdimg, mode == ModeDiffBinary, dc)
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
			TaskName:     "diff",
			ElapsedMilli: int(elapsed.Milliseconds()),
			Size:         stat.Size(),
			Labels: map[string]string{
				"oldCdimg": oldCdimg,
				"newCdimg": newCdimg,
				"outCdimg": outCdimg,
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
