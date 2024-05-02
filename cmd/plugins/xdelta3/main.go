package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/dsnet/compress/bzip2"
	"github.com/google/uuid"
	"github.com/naoki9911/fuse-diff-containerd/pkg/bsdiffx"
	"github.com/nine-lives-later/go-xdelta"
)

// install xdelta3 https://github.com/wdhongtw/xdelta

func Info() string {
	return "plugin for gz compressed files"
}

func Diff(oldBytes, newBytes []byte, patchWriter io.Writer, mode bsdiffx.CompressionMode) error {
	writer, err := bzip2.NewWriter(patchWriter, nil)
	if err != nil {
		return err
	}
	defer writer.Close()

	options := xdelta.EncoderOptions{
		FileID:      "myfile.ext",
		FromFile:    bytes.NewReader(oldBytes),
		ToFile:      bytes.NewReader(newBytes),
		PatchFile:   writer,
		BlockSizeKB: 4,
	}

	enc, err := xdelta.NewEncoder(options)
	if err != nil {
		return err
	}
	defer enc.Close()

	// create the patch
	err = enc.Process(context.TODO())
	if err != nil {
		return err
	}
	return nil
}

func Patch(oldBytes []byte, patchReader io.Reader) ([]byte, error) {
	reader, err := bzip2.NewReader(patchReader, nil)
	if err != nil {
		return nil, err
	}

	new := bytes.NewBuffer(nil)
	options := xdelta.DecoderOptions{
		FileID:      "myfile.ext",
		FromFile:    bytes.NewReader(oldBytes),
		ToFile:      new,
		PatchFile:   reader,
		BlockSizeKB: 4,
	}

	dec, err := xdelta.NewDecoder(options)
	if err != nil {
		return nil, fmt.Errorf("failed to new decoder: %v", err)
	}
	defer dec.Close()

	err = dec.Process(context.TODO())
	if err != nil {
		return nil, fmt.Errorf("failed to process decoder: %v", err)
	}
	return new.Bytes(), nil
}

func Merge(lowerDiff, upperDiff io.Reader, mergedDiff io.Writer) error {
	lowerReader, err := bzip2.NewReader(lowerDiff, nil)
	if err != nil {
		return err
	}
	upperReader, err := bzip2.NewReader(upperDiff, nil)
	if err != nil {
		return err
	}

	lowerFile, err := os.CreateTemp("", "*")
	if err != nil {
		return err
	}
	lowerFileName := lowerFile.Name()
	defer os.Remove(lowerFileName)
	defer lowerFile.Close()

	_, err = io.Copy(lowerFile, lowerReader)
	if err != nil {
		return err
	}
	err = lowerFile.Close()
	if err != nil {
		return err
	}

	upperFile, err := os.CreateTemp("", "*")
	if err != nil {
		return err
	}
	upperFileName := upperFile.Name()
	defer os.Remove(upperFileName)
	defer upperFile.Close()

	_, err = io.Copy(upperFile, upperReader)
	if err != nil {
		return err
	}
	err = upperFile.Close()
	if err != nil {
		return err
	}

	u, err := uuid.NewRandom()
	if err != nil {
		return err
	}
	mergedFileName := filepath.Join(os.TempDir(), u.String())
	defer os.Remove(mergedFileName)

	cmd := exec.Command("xdelta3", "merge", "-m", lowerFileName, upperFileName, mergedFileName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return err
	}

	mergedFile, err := os.Open(mergedFileName)
	if err != nil {
		return err
	}

	bzip2Writer, err := bzip2.NewWriter(mergedDiff, nil)
	if err != nil {
		return err
	}
	defer bzip2Writer.Close()

	_, err = io.Copy(bzip2Writer, mergedFile)
	if err != nil {
		return err
	}

	return nil
}

func Compare(a, b []byte) bool {
	return bytes.Equal(a, b)
}

func ID() uuid.UUID {
	return uuid.MustParse("c4e21629-5937-49d2-9646-b93df8e04b5d")
}

func init() {
}
