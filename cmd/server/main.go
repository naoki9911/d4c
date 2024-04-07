package main

import (
	"context"
	"flag"

	"github.com/containerd/containerd/log"
	"github.com/naoki9911/fuse-diff-containerd/pkg/image"
	"github.com/naoki9911/fuse-diff-containerd/pkg/server"
)

var logger = log.G(context.TODO())

func main() {
	threadNum := flag.Int("threadNum", 1, "Te number of threads to merge diffs")
	flag.Parse()
	mc := image.MergeConfig{
		ThreadNum:              *threadNum,
		MergeDimgConcurrentNum: 4,
	}
	ds, err := server.NewDiffServer(mc)
	if err != nil {
		logger.Errorf("failed to create DiffServer: %v", err)
	}
	err = ds.ListenAndServe(":8081")
	if err != nil {
		logger.Fatalf("failed to start server: %v", err)
	}
}
