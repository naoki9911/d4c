package main

import (
	"fmt"
	"os"
	"time"

	"github.com/naoki9911/fuse-diff-containerd/pkg/benchmark"
	"github.com/naoki9911/fuse-diff-containerd/pkg/image"
	log "github.com/sirupsen/logrus"
)

func main() {
	log.SetLevel(log.WarnLevel)
	if len(os.Args) < 5 {
		fmt.Println("merge dimg lower-dimg upper-dimg merged-dimg benchmark")
		os.Exit(1)
	}
	isImage := os.Args[1] == "dimg"
	if isImage {
		lowerDimg := os.Args[2]
		upperDimg := os.Args[3]
		mergedDimg := os.Args[4]
		var b *benchmark.Benchmark = nil
		var err error
		if len(os.Args) > 5 && os.Args[5] == "benchmark" {
			b, err = benchmark.NewBenchmark("./benchmark.log")
			if err != nil {
				panic(err)
			}
			defer b.Close()
		}

		start := time.Now()

		mergeFile, err := os.Create(mergedDimg)
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
				TaskName:     "merge",
				ElapsedMilli: int(time.Since(start).Milliseconds()),
				Labels: []string{
					"lower:" + lowerDimg,
					"upper:" + upperDimg,
					"dimg",
				},
			}
			err = b.AppendResult(metric)
			if err != nil {
				panic(err)
			}
		}
	} else {
		fmt.Println("only 'dimg' mode is allowed")
		os.Exit(1)
	}
}
