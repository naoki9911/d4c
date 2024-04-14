package main

import (
	"io"

	"github.com/naoki9911/fuse-diff-containerd/pkg/bsdiffx"
)

func Info() string {
	return "plugin for gz compressed files"
}

func Diff(oldBytes, newBytes []byte, patchWriter io.Writer, mode bsdiffx.CompressionMode) error {
	return bsdiffx.Diff(oldBytes, newBytes, patchWriter, mode)
}

func Patch(oldBytes []byte, patchReader io.Reader) ([]byte, error) {
	return bsdiffx.Patch(oldBytes, patchReader)
}

func Merge(lowerDiff, upperDiff io.Reader, mergedDiff io.Writer) error {
	return bsdiffx.DeltaMergingBytes(lowerDiff, upperDiff, mergedDiff)
}

func init() {
}
