package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/containerd/containerd/log"
	"github.com/naoki9911/fuse-diff-containerd/pkg/benchmark"
	"github.com/naoki9911/fuse-diff-containerd/pkg/image"
	"github.com/sirupsen/logrus"
)

var logger = log.G(context.TODO())

func main() {
	logger.Logger.SetLevel(logrus.WarnLevel)
	if len(os.Args) < 6 {
		fmt.Println("diff base-dir new-dir output-dir json-file mode [benchmark]")
		fmt.Println("diff dimg base-dimg new-dimg output-dimg mode [benchmark]")
		os.Exit(1)
	}

	benchmarkEnabled := false
	if len(os.Args) == 7 {
		benchmarkEnabled = os.Args[6] == "benchmark"
	}
	var b *benchmark.Benchmark = nil
	var err error
	if benchmarkEnabled {
		b, err = benchmark.NewBenchmark("./benchmark.log")
		if err != nil {
			panic(err)
		}
		defer b.Close()
	}
	var base string
	var new string
	var mode string
	start := time.Now()
	if os.Args[1] == "dimg" {
		baseDimgPath := os.Args[2]
		newDimgPath := os.Args[3]
		outputDimgPath := os.Args[4]
		mode = os.Args[5]
		if mode != "binary-diff" && mode != "file-diff" {
			fmt.Println("mode is \"binary-diff\" or \"file-diff\"")
			os.Exit(1)
		}
		base = baseDimgPath
		new = newDimgPath

		err := image.GenerateDiffFromDimg(baseDimgPath, newDimgPath, outputDimgPath, mode == "binary-diff")
		if err != nil {
			panic(err)
		}
	} else {
		baseDir := os.Args[1]
		newDir := os.Args[2]
		outputDir := os.Args[3]
		jsonPath := os.Args[4]
		mode = os.Args[5]
		if mode != "binary-diff" && mode != "file-diff" {
			fmt.Println("mode is \"binary-diff\" or \"file-diff\"")
			os.Exit(1)
		}
		base = baseDir
		new = newDir

		os.RemoveAll(outputDir)
		os.RemoveAll(jsonPath)

		entry, err := image.GenerateDiffFromDir(baseDir, newDir, outputDir, mode == "binary-diff", baseDir != "")
		if err != nil {
			panic(err)
		}

		//entry.print("", true)
		entryJson, err := json.MarshalIndent(entry, "", "  ")
		if err != nil {
			panic(err)
		}
		jsonFile, err := os.Create(jsonPath)
		if err != nil {
			panic(err)
		}
		defer jsonFile.Close()
		jsonFile.Write(entryJson)
	}

	if benchmarkEnabled {
		elapsedMilli := time.Since(start).Milliseconds()
		metric := benchmark.Metric{
			TaskName:     "diff",
			ElapsedMilli: int(elapsedMilli),
			Labels: []string{
				"base:" + base,
				"new:" + new,
				mode,
			},
		}
		err = b.AppendResult(metric)
		if err != nil {
			panic(err)
		}
	}
}
