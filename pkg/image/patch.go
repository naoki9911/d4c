package image

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"

	"github.com/klauspost/compress/zstd"
	"github.com/naoki9911/fuse-diff-containerd/pkg/bsdiffx"
	cp "github.com/otiai10/copy"
)

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
	err = bsdiffx.Patch(baseFile, newFile, patch)
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
	err = bsdiffx.Patch(baseFile, gzipNewWriter, patch)
	if err != nil {
		return err
	}

	return nil
}

func ApplyPatch(basePath, newPath string, dirEntry *FileEntry, img *DimgFile, isBase bool) error {
	fName := dirEntry.Name
	baseFilePath := path.Join(basePath, fName)
	newFilePath := path.Join(newPath, fName)

	if isBase && !dirEntry.IsDir() && !dirEntry.IsSymlink() && !dirEntry.IsNew() {
		return fmt.Errorf("invalid base image %q", newFilePath)
	}

	if dirEntry.Type == FILE_ENTRY_SYMLINK {
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
		err := os.Mkdir(newFilePath, os.ModePerm)
		if err != nil {
			return err
		}
		for _, c := range dirEntry.Childs {
			err = ApplyPatch(baseFilePath, newFilePath, c, img, isBase)
			if err != nil {
				return err
			}
		}
	} else if dirEntry.Type == FILE_ENTRY_FILE_SAME {
		err := cp.Copy(baseFilePath, newFilePath)
		if err != nil {
			return err
		}
	} else if dirEntry.Type == FILE_ENTRY_FILE_NEW {
		//if strings.Contains(dirEntry.Name, ".wh") {
		//	fmt.Println(newFilePath)
		//}
		logger.Debugf("copy %q from image(offset=%d size=%d)", newFilePath, dirEntry.Offset, dirEntry.CompressedSize)
		patchBytes := make([]byte, dirEntry.CompressedSize)
		_, err := img.ReadAt(patchBytes, dirEntry.Offset)
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

		_, err = io.Copy(newFile, patchReader)
		if err != nil {
			return err
		}
	} else if dirEntry.Type == FILE_ENTRY_FILE_DIFF {
		var patchReader io.Reader
		logger.Debugf("applying diff to %q from image(offset=%d size=%d)", newFilePath, dirEntry.Offset, dirEntry.CompressedSize)
		patchBytes := make([]byte, dirEntry.CompressedSize)
		_, err := img.ReadAt(patchBytes, dirEntry.Offset)
		if err != nil {
			return err
		}
		patchReader = bytes.NewBuffer(patchBytes)
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
