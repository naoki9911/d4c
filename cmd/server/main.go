package main

import (
	"context"
	"flag"

	"github.com/containerd/containerd/log"
	"github.com/naoki9911/fuse-diff-containerd/pkg/bsdiffx"
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

	pm, err := bsdiffx.LoadOrDefaultPlugins("")
	if err != nil {
		logger.Errorf("failed to load plugins: %v", err)
	}

	ds, err := server.NewDiffServer(mc, pm)
	if err != nil {
		logger.Errorf("failed to create DiffServer: %v", err)
	}
	err = ds.ListenAndServe(":8081")
	if err != nil {
		logger.Fatalf("failed to start server: %v", err)
	}
}
