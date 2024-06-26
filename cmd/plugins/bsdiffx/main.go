package main

import (
	"bytes"
	"io"

	"github.com/google/uuid"
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

func Compare(a, b []byte) bool {
	return bytes.Equal(a, b)
}

func ID() uuid.UUID {
	return uuid.MustParse("2466ff1d-27f2-4cc5-a6ef-bf7de2b7a05f")
}

func init() {
}
