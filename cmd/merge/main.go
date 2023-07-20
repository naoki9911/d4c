package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/jinzhu/copier"
	"github.com/naoki9911/fuse-diff-containerd/pkg/benchmark"
	"github.com/naoki9911/fuse-diff-containerd/pkg/di3fs"
	log "github.com/sirupsen/logrus"
)

func main() {
	log.SetLevel(log.WarnLevel)
	if len(os.Args) < 5 {
		fmt.Println("merge dir lower-diff lower-json upper-diff upper-json merged-diff merged-json")
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
		err = di3fs.MergeDimg(lowerDimg, upperDimg, mergeFile)
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
		lowerDiff := os.Args[2]
		lowerJson := os.Args[3]
		upperDiff := os.Args[4]
		upperJson := os.Args[5]
		mergedDiff := os.Args[6]
		mergedJson := os.Args[7]

		lowerJsonRaw, err := os.ReadFile(lowerJson)
		if err != nil {
			panic(err)
		}
		lowerEntry := di3fs.FileEntry{}
		err = json.Unmarshal(lowerJsonRaw, &lowerEntry)
		if err != nil {
			panic(err)
		}

		upperJsonRaw, err := os.ReadFile(upperJson)
		if err != nil {
			panic(err)
		}
		upperEntry := di3fs.FileEntry{}
		err = json.Unmarshal(upperJsonRaw, &upperEntry)
		if err != nil {
			panic(err)
		}
		mergedEntry := di3fs.NewFileEntry()
		err = copier.Copy(mergedEntry, upperEntry)
		if err != nil {
			panic(err)
		}

		err = di3fs.MergeDiff(lowerDiff, upperDiff, mergedDiff, &lowerEntry, &upperEntry, mergedEntry)
		if err != nil {
			panic(err)
		}

		//entry.print("", true)
		entryJson, err := json.MarshalIndent(mergedEntry, "", "  ")
		if err != nil {
			panic(err)
		}
		jsonFile, err := os.Create(mergedJson)
		if err != nil {
			panic(err)
		}
		defer jsonFile.Close()
		jsonFile.Write(entryJson)
	}
}
