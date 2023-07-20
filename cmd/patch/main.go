package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/containerd/containerd/log"
	"github.com/icedream/go-bsdiff"
	"github.com/klauspost/compress/zstd"
	"github.com/naoki9911/fuse-diff-containerd/pkg/benchmark"
	"github.com/naoki9911/fuse-diff-containerd/pkg/di3fs"
	cp "github.com/otiai10/copy"
	"github.com/sirupsen/logrus"
)

var logger = log.G(context.TODO())

func applyFilePatch(baseFilePath, newFilePath string, patch io.Reader) error {
	//fmt.Println(newFilePath)
	baseFile, err := os.Open(baseFilePath)
	if err != nil {
		return err
	}
	defer baseFile.Close()
	newFile, err := os.Create(newFilePath)
	if err != nil {
		return err
	}
	defer newFile.Close()
	err = bsdiff.Patch(baseFile, newFile, patch)
	if err != nil {
		return err
	}

	return nil
}

func applyFilePatchForGz(baseFilePath, newFilePath string, patch io.Reader) error {
	baseFile, err := os.Open(baseFilePath)
	if err != nil {
		return err
	}
	defer baseFile.Close()
	newFile, err := os.Create(newFilePath)
	if err != nil {
		return err
	}
	defer newFile.Close()

	gzipNewWriter := gzip.NewWriter(newFile)
	defer gzipNewWriter.Close()
	err = bsdiff.Patch(baseFile, gzipNewWriter, patch)
	if err != nil {
		return err
	}

	return nil
}

func applyPatch(basePath, newPath, patchPath string, dirEntry di3fs.FileEntry, image *os.File, imageStartOffset int64, isBase bool) error {
	fName := dirEntry.Name
	isImage := image != nil
	baseFilePath := path.Join(basePath, fName)
	newFilePath := path.Join(newPath, fName)
	patchFilePath := path.Join(patchPath, dirEntry.DiffName)

	if isBase && !dirEntry.IsDir() && !dirEntry.IsSymlink() && !dirEntry.IsNew() {
		return fmt.Errorf("invalid base image %q", newFilePath)
	}

	if dirEntry.Type == di3fs.FILE_ENTRY_SYMLINK {
		prevDir, err := filepath.Abs(".")
		if err != nil {
			return err
		}

		err = os.Chdir(newPath)
		if err != nil {
			return err
		}

		err = os.Symlink(dirEntry.RealPath, fName)
		if err != nil {
			return err
		}

		err = os.Chdir(prevDir)
		if err != nil {
			return err
		}
	} else if dirEntry.IsDir() {
		patchFilePath = path.Join(patchPath, dirEntry.Name)
		err := os.Mkdir(newFilePath, os.ModePerm)
		if err != nil {
			return err
		}
		for _, c := range dirEntry.Childs {
			err = applyPatch(baseFilePath, newFilePath, patchFilePath, c, image, imageStartOffset, isBase)
			if err != nil {
				return err
			}
		}
	} else if dirEntry.Type == di3fs.FILE_ENTRY_FILE_SAME {
		err := cp.Copy(baseFilePath, newFilePath)
		if err != nil {
			return err
		}
	} else if dirEntry.Type == di3fs.FILE_ENTRY_FILE_NEW {
		//if strings.Contains(dirEntry.Name, ".wh") {
		//	fmt.Println(newFilePath)
		//}
		if isImage {
			logger.Debugf("copy %q from image(offset=%d size=%d)", newFilePath, dirEntry.Offset, dirEntry.CompressedSize)
			patchBytes := make([]byte, dirEntry.CompressedSize)
			_, err := image.ReadAt(patchBytes, dirEntry.Offset+imageStartOffset)
			if err != nil {
				return err
			}
			patchBuf := bytes.NewBuffer(patchBytes)
			patchReader, err := zstd.NewReader(patchBuf)
			if err != nil {
				return err
			}
			defer patchReader.Close()

			newFile, err := os.Create(newFilePath)
			if err != nil {
				return err
			}
			defer newFile.Close()

			io.Copy(newFile, patchReader)
		} else {
			err := cp.Copy(patchFilePath, newFilePath)
			if err != nil {
				return err
			}
		}
	} else if dirEntry.Type == di3fs.FILE_ENTRY_FILE_DIFF {
		var patchReader io.Reader
		if isImage {
			logger.Debugf("applying diff to %q from image(offset=%d size=%d)", newFilePath, dirEntry.Offset, dirEntry.CompressedSize)
			patchBytes := make([]byte, dirEntry.CompressedSize)
			_, err := image.ReadAt(patchBytes, dirEntry.Offset+imageStartOffset)
			if err != nil {
				return err
			}
			patchReader = bytes.NewBuffer(patchBytes)
		} else {
			patchFile, err := os.Open(patchFilePath)
			if err != nil {
				return err
			}
			defer patchFile.Close()
			patchReader = patchFile
		}
		if dirEntry.UncompressedGz {
			err := applyFilePatchForGz(baseFilePath, newFilePath, patchReader)
			if err != nil {
				return err
			}
		} else {
			err := applyFilePatch(baseFilePath, newFilePath, patchReader)
			if err != nil {
				return err
			}
		}
	} else {
		return fmt.Errorf("unexpected error type=%v", dirEntry.Type)
	}

	for _, o := range dirEntry.OaqueFiles {
		f, err := os.Create(path.Join(newFilePath, o))
		if err != nil {
			return err
		}
		err = f.Chmod(os.FileMode(0755))
		if err != nil {
			return err
		}
		f.Close()
	}
	return nil
}

func main() {
	logger.Logger.SetLevel(logrus.WarnLevel)
	if len(os.Args) < 5 {
		fmt.Println("diff dir base-dir new-dir [patch-dir|patch-img] json-file")
		fmt.Println("diff dimg base-dir new-dir patch-img [benchmark]")
		os.Exit(1)
	}
	mode := os.Args[1]
	baseDir := os.Args[2]
	newDir := os.Args[3]
	patchDir := os.Args[4]

	os.RemoveAll(newDir)

	//entry.print("", false)
	if mode == "dimg" {
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
		imageHeader, imageFile, curOffset, err := di3fs.LoadImage(patchDir)
		if err != nil {
			panic(err)
		}
		err = applyPatch(baseDir, newDir, patchDir, imageHeader.FileEntry, imageFile, curOffset, imageHeader.BaseId == "")
		if err != nil {
			panic(err)
		}
		if b != nil {
			metric := benchmark.Metric{
				TaskName:     "patch",
				ElapsedMilli: int(time.Since(start).Milliseconds()),
				Labels: []string{
					"base:" + baseDir,
					"new:" + newDir,
					mode,
				},
			}
			err = b.AppendResult(metric)
			if err != nil {
				panic(err)
			}
		}
	} else {
		entry := di3fs.FileEntry{}
		jsonPath := os.Args[5]
		jsonRaw, err := os.ReadFile(jsonPath)
		if err != nil {
			panic(err)
		}
		err = json.Unmarshal(jsonRaw, &entry)
		if err != nil {
			panic(err)
		}
		err = applyPatch(baseDir, newDir, patchDir, entry, nil, 0, false)
		if err != nil {
			panic(err)
		}
	}
}
