package merge

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
				Required: false,
			},
			&cli.StringFlag{
				Name:     "upperDimg",
				Usage:    "path to upper dimg",
				Required: false,
			},
			&cli.StringFlag{
				Name:     "outDimg",
				Usage:    "path to merged dimg",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "dimgs",
				Usage:    "path to merged dimgs. path is ordered in upper to lower form (upperN, upperN-1, upperN-2, .. upper0)",
				Value:    "",
				Required: false,
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
			&cli.IntFlag{
				Name:     "mergeDimgConcurrentNum",
				Usage:    "The nubmer of merged concurrently",
				Value:    1,
				Required: false,
			},
			&cli.StringFlag{
				Name:     "mergeMode",
				Usage:    "The mode to merge (linear, bisect)",
				Value:    "linear",
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
	mergeDimgConcurrentNum := c.Int("mergeDimgConcurrentNum")
	mergeMode := c.String("mergeMode")
	dimgs := c.String("dimgs")
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

	mergeConfig := image.MergeConfig{
		ThreadNum:              threadNum,
		MergeDimgConcurrentNum: mergeDimgConcurrentNum,
		BenchmarkPerFile:       enableBenchPerFile,
		Benchmarker:            b,
	}
	var header *image.DimgHeader
	start := time.Now()

	if dimgs != "" {
		dimgEntry := []*image.DimgEntry{}
		for _, path := range strings.Split(dimgs, ",") {
			dimgFile, err := image.OpenDimgFile(path)
			if err != nil {
				return fmt.Errorf("failed to open %s: %v", path, err)
			}
			defer dimgFile.Close()
			dimgEntry = append(dimgEntry, &image.DimgEntry{
				DimgHeader: *dimgFile.Header(),
				Path:       path,
			})
		}
		start = time.Now()
		tmpDir := filepath.Join("/tmp/d4c", utils.GetRandomId("merge-tmp"))
		err = os.MkdirAll(tmpDir, 0755)
		if err != nil {
			return fmt.Errorf("failed to create %s: %v", tmpDir, err)
		}
		defer os.RemoveAll(tmpDir)
		var mergedDimg *image.DimgEntry
		switch mergeMode {
		case "linear":
			mergedDimg, err = image.MergeDimgsWithLinear(dimgEntry, tmpDir, mergeConfig, false)
		case "bisect":
			mergedDimg, err = image.MergeDimgsWithBisectMultithread(dimgEntry, tmpDir, mergeConfig, false)
		default:
			return fmt.Errorf("invalid mergeMode %s (only 'linear' or 'bisect' are allowed)", mergeMode)
		}
		if err != nil {
			return fmt.Errorf("failed to merge: %v", err)
		}
		err = os.Rename(mergedDimg.Path, outDimg)
		if err != nil {
			return fmt.Errorf("failed to rename %s to %s: %v", mergedDimg.Path, outDimg, err)
		}
	} else {
		mergeFile, err := os.Create(outDimg)
		if err != nil {
			panic(err)
		}
		defer mergeFile.Close()
		header, err = image.MergeDimg(lowerDimg, upperDimg, mergeFile, mergeConfig)
		if err != nil {
			panic(err)
		}
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
				"lowerDimg":              lowerDimg,
				"upperDimg":              upperDimg,
				"outDimg":                outDimg,
				"threadNum":              strconv.Itoa(threadNum),
				"compressionMode":        bsdiffx.CompressionModeToString(header.CompressionMode),
				"mergeDimgConcurrentNum": strconv.Itoa(mergeDimgConcurrentNum),
				"mergeMode":              mergeMode,
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
				Required: false,
			},
			&cli.StringFlag{
				Name:     "upperCdimg",
				Usage:    "path to upper cdimg",
				Required: false,
			},
			&cli.StringFlag{
				Name:     "outCdimg",
				Usage:    "path to merged cdimg",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "cdimgs",
				Usage:    "path to merged cdimgs. path is ordered in upper to lower form (upperN, upperN-1, upperN-2, .. upper0)",
				Value:    "",
				Required: false,
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
			&cli.IntFlag{
				Name:     "mergeDimgConcurrentNum",
				Usage:    "The nubmer of merged concurrently",
				Value:    1,
				Required: false,
			},
			&cli.StringFlag{
				Name:     "mergeMode",
				Usage:    "The mode to merge (linear, bisect)",
				Value:    "linear",
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
	mergeDimgConcurrentNum := c.Int("mergeDimgConcurrentNum")
	mergeMode := c.String("mergeMode")
	cdimgs := c.String("cdimgs")
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

	mergeConfig := image.MergeConfig{
		ThreadNum:              threadNum,
		MergeDimgConcurrentNum: mergeDimgConcurrentNum,
		BenchmarkPerFile:       enableBenchPerFile,
		Benchmarker:            b,
	}
	var header *image.DimgHeader
	start := time.Now()

	if cdimgs != "" {
		dimgEntry := []*image.DimgEntry{}
		for _, path := range strings.Split(cdimgs, ",") {
			cdimgFile, err := image.OpenCdimgFile(path)
			if err != nil {
				return fmt.Errorf("failed to open %s: %v", path, err)
			}
			defer cdimgFile.Close()
			dimgEntry = append(dimgEntry, &image.DimgEntry{
				DimgHeader: *cdimgFile.Dimg.Header(),
				Path:       path,
			})
		}
		start = time.Now()
		tmpDir := filepath.Join("/tmp/d4c", utils.GetRandomId("merge-tmp"))
		err = os.MkdirAll(tmpDir, 0755)
		if err != nil {
			return fmt.Errorf("failed to create %s: %v", tmpDir, err)
		}
		defer os.RemoveAll(tmpDir)
		var mergedDimg *image.DimgEntry
		switch mergeMode {
		case "linear":
			mergedDimg, err = image.MergeDimgsWithLinear(dimgEntry, tmpDir, mergeConfig, true)
		case "bisect":
			mergedDimg, err = image.MergeDimgsWithBisectMultithread(dimgEntry, tmpDir, mergeConfig, true)
		default:
			return fmt.Errorf("invalid mergeMode %s (only 'linear' or 'bisect' are allowed)", mergeMode)
		}
		if err != nil {
			return fmt.Errorf("failed to merge: %v", err)
		}
		err = os.Rename(mergedDimg.Path, outCdimg)
		if err != nil {
			return fmt.Errorf("failed to rename %s to %s: %v", mergedDimg.Path, outCdimg, err)
		}
	} else {
		mergeFile, err := os.Create(outCdimg)
		if err != nil {
			panic(err)
		}
		defer mergeFile.Close()
		header, err = image.MergeCdimg(lowerCdimg, upperCdimg, mergeFile, mergeConfig)
		if err != nil {
			panic(err)
		}
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
				"lowerCdimg":             lowerCdimg,
				"upperCdimg":             upperCdimg,
				"outCdimg":               outCdimg,
				"threadNum":              strconv.Itoa(threadNum),
				"compressionMode":        bsdiffx.CompressionModeToString(header.CompressionMode),
				"mergeDimgConcurrentNum": strconv.Itoa(mergeDimgConcurrentNum),
				"mergeMode":              mergeMode,
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
