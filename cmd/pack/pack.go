package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/containerd/containerd/log"
	"github.com/naoki9911/fuse-diff-containerd/pkg/di3fs"
	"github.com/naoki9911/fuse-diff-containerd/pkg/image"
	"github.com/sirupsen/logrus"
)

var logger = log.G(context.TODO())

func main() {
	logger.Logger.SetLevel(logrus.WarnLevel)
	if len(os.Args) < 5 {
		fmt.Println("diff diff-dir json-file baseDImg ouput")
		os.Exit(1)
	}
	diffDir := os.Args[1]
	jsonPath := os.Args[2]
	baseDImg := os.Args[3]
	outputPath := os.Args[4]

	jsonRaw, err := os.ReadFile(jsonPath)
	if err != nil {
		panic(err)
	}
	logger.WithFields(logrus.Fields{"diffDir": diffDir, "diffJsonPath": jsonPath, "baseDImg": baseDImg, "outputPath": outputPath}).Info("start packing")
	entry := &di3fs.FileEntry{}
	json.Unmarshal(jsonRaw, entry)
	err = image.PackDimg(logger, diffDir, entry, baseDImg, outputPath)
	if err != nil {
		logger.Fatal("failed to pack dimg")
	}
	logger.Info("pack done")
}
