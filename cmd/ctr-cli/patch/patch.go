package patch

import (
	"context"
	"os"
	"time"

	"github.com/containerd/containerd/log"
	"github.com/naoki9911/fuse-diff-containerd/pkg/benchmark"
	"github.com/naoki9911/fuse-diff-containerd/pkg/image"
	"github.com/naoki9911/fuse-diff-containerd/pkg/utils"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var logger = log.G(context.TODO())

func DimgCommand() *cli.Command {
	cmd := cli.Command{
		Name:  "patch",
		Usage: "patch diff dimg to directory",
		Action: func(context *cli.Context) error {
			return dimgAction(context)
		},
		Flags: []cli.Flag{
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
		},
	}

	return &cmd
}

func dimgAction(c *cli.Context) error {
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

	dimgFile, err := image.OpenDimgFile(diffDimg)
	if err != nil {
		panic(err)
	}
	defer dimgFile.Close()
	dimgHeader := dimgFile.Header()

	start := time.Now()
	err = image.ApplyPatch(baseDir, outDir, &dimgHeader.FileEntry, dimgFile, dimgHeader.ParentId == "")
	if err != nil {
		panic(err)
	}
	if b != nil {
		metric := benchmark.Metric{
			TaskName:     "patch",
			ElapsedMilli: int(time.Since(start).Milliseconds()),
			Labels: map[string]string{
				"baseDir":  baseDir,
				"outDir":   outDir,
				"diffDimg": diffDimg,
			},
		}
		metric.AddLabels(utils.ParseLabels(c.StringSlice("labels")))
		err = b.AppendResult(metric)
		if err != nil {
			panic(err)
		}
	}
	logger.Info("patch done")
	return nil
}

func CdimgCommand() *cli.Command {
	cmd := cli.Command{
		Name:  "patch",
		Usage: "patch diff cdimg to directory",
		Action: func(context *cli.Context) error {
			return cdimgAction(context)
		},
		Flags: []cli.Flag{
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
				Name:     "diffCdimg",
				Usage:    "path to diff cdimg",
				Required: true,
			},
			&cli.BoolFlag{
				Name:     "benchmark",
				Usage:    "enable benchmark",
				Value:    false,
				Required: false,
			},
		},
	}

	return &cmd
}

func cdimgAction(c *cli.Context) error {
	logger.Logger.SetLevel(logrus.WarnLevel)
	baseDir := c.String("baseDir")
	outDir := c.String("outDir")
	diffCdimg := c.String("diffCdimg")
	enableBench := c.Bool("benchmark")
	logger.WithFields(logrus.Fields{
		"baseDir":   baseDir,
		"outDir":    outDir,
		"diffCdimg": diffCdimg,
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

	cdimgFile, err := image.OpenCdimgFile(diffCdimg)
	if err != nil {
		panic(err)
	}
	defer cdimgFile.Close()
	dimgFile := cdimgFile.Dimg
	dimgHeader := dimgFile.Header()

	start := time.Now()
	err = image.ApplyPatch(baseDir, outDir, &dimgHeader.FileEntry, dimgFile, dimgHeader.ParentId == "")
	if err != nil {
		panic(err)
	}
	if b != nil {
		metric := benchmark.Metric{
			TaskName:     "patch",
			ElapsedMilli: int(time.Since(start).Milliseconds()),
			Labels: map[string]string{
				"baseDir":   baseDir,
				"outDir":    outDir,
				"diffCdimg": diffCdimg,
			},
		}
		metric.AddLabels(utils.ParseLabels(c.StringSlice("labels")))
		err = b.AppendResult(metric)
		if err != nil {
			panic(err)
		}
	}
	logger.Info("patch done")
	return nil
}
