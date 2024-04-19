package image

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"

	"github.com/klauspost/compress/zstd"
	"github.com/naoki9911/fuse-diff-containerd/pkg/bsdiffx"
	cp "github.com/otiai10/copy"
)

func ApplyFilePatch(baseFilePath, newFilePath string, patch io.Reader, p *bsdiffx.Plugin) error {
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

	baseBytes, err := io.ReadAll(baseFile)
	if err != nil {
		return err
	}
	newBytes, err := p.Patch(baseBytes, patch)
	if err != nil {
		return err
	}

	_, err = newFile.Write(newBytes)
	if err != nil {
		return err
	}

	return nil
}

//func applyFilePatchForGz(baseFilePath, newFilePath string, patch io.Reader) error {
//	baseFile, err := os.Open(baseFilePath)
//	if err != nil {
//		return err
//	}
//	defer baseFile.Close()
//	newFile, err := os.Create(newFilePath)
//	if err != nil {
//		return err
//	}
//	defer newFile.Close()
//
//	gzipNewWriter := gzip.NewWriter(newFile)
//	defer gzipNewWriter.Close()
//	err = bsdiffx.Patch(baseFile, gzipNewWriter, patch)
//	if err != nil {
//		return err
//	}
//
//	return nil
//}

func ApplyPatch(basePath, newPath string, dirEntry *FileEntry, img *DimgFile, isBase bool, pm *bsdiffx.PluginManager) error {
	hardlinks, err := applyPatchImpl(basePath, newPath, dirEntry, img, isBase, pm)
	if err != nil {
		return fmt.Errorf("failed to apply patch: %v", err)
	}

	for _, h := range hardlinks {
		targetPath := filepath.Join(newPath, h.link)
		err = os.Link(targetPath, h.path)
		if err != nil {
			return fmt.Errorf("failed to create hardlink from %s to %s", h.path, targetPath)
		}
	}

	return nil
}

type hardlinkEntry struct {
	path string
	link string
}

func applyPatchImpl(basePath, newPath string, dirEntry *FileEntry, img *DimgFile, isBase bool, pm *bsdiffx.PluginManager) ([]*hardlinkEntry, error) {
	fName := dirEntry.Name
	baseFilePath := path.Join(basePath, fName)
	newFilePath := path.Join(newPath, fName)
	hardlinks := []*hardlinkEntry{}

	if isBase && dirEntry.IsBaseRequired() {
		return nil, fmt.Errorf("invalid base image %q", newFilePath)
	}

	if dirEntry.Type == FILE_ENTRY_SYMLINK {
		prevDir, err := filepath.Abs(".")
		if err != nil {
			return nil, err
		}

		err = os.Chdir(newPath)
		if err != nil {
			return nil, err
		}

		err = os.Symlink(dirEntry.RealPath, fName)
		if err != nil {
			return nil, err
		}

		err = os.Chdir(prevDir)
		if err != nil {
			return nil, err
		}
	} else if dirEntry.Type == FILE_ENTRY_HARDLINK {
		hardlinks = append(hardlinks, &hardlinkEntry{
			path: newFilePath,
			link: dirEntry.RealPath,
		})
	} else if dirEntry.IsDir() {
		err := os.Mkdir(newFilePath, os.ModePerm)
		if err != nil {
			return nil, err
		}
		for _, c := range dirEntry.Childs {
			h, err := applyPatchImpl(baseFilePath, newFilePath, c, img, isBase, pm)
			if err != nil {
				return nil, err
			}
			hardlinks = append(hardlinks, h...)
		}
	} else if dirEntry.Type == FILE_ENTRY_FILE_SAME {
		err := cp.Copy(baseFilePath, newFilePath)
		if err != nil {
			return nil, err
		}
	} else if dirEntry.Type == FILE_ENTRY_FILE_NEW {
		//if strings.Contains(dirEntry.Name, ".wh") {
		//	fmt.Println(newFilePath)
		//}
		logger.Debugf("copy %q from image(offset=%d size=%d)", newFilePath, dirEntry.Offset, dirEntry.CompressedSize)
		patchBytes := make([]byte, dirEntry.CompressedSize)
		_, err := img.ReadAt(patchBytes, dirEntry.Offset)
		if err != nil {
			return nil, err
		}
		patchBuf := bytes.NewBuffer(patchBytes)
		patchReader, err := zstd.NewReader(patchBuf)
		if err != nil {
			return nil, err
		}
		defer patchReader.Close()

		newFile, err := os.Create(newFilePath)
		if err != nil {
			return nil, err
		}
		defer newFile.Close()

		_, err = io.Copy(newFile, patchReader)
		if err != nil {
			return nil, err
		}
	} else if dirEntry.Type == FILE_ENTRY_FILE_DIFF {
		p := pm.GetPluginByExt(filepath.Ext(dirEntry.Name))
		var patchReader io.Reader
		logger.Debugf("applying diff to %q from image(offset=%d size=%d)", newFilePath, dirEntry.Offset, dirEntry.CompressedSize)
		patchBytes := make([]byte, dirEntry.CompressedSize)
		_, err := img.ReadAt(patchBytes, dirEntry.Offset)
		if err != nil {
			return nil, err
		}
		patchReader = bytes.NewBuffer(patchBytes)
		err = ApplyFilePatch(baseFilePath, newFilePath, patchReader, p)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("unexpected error type=%v", dirEntry.Type)
	}

	data := []byte{}
	if dirEntry.IsFile() {
		f, err := os.Open(newFilePath)
		if err != nil {
			return nil, err
		}
		defer f.Close()

		data, err = io.ReadAll(f)
		if err != nil {
			return nil, err
		}
	}
	err := dirEntry.Verify(data)
	if err != nil {
		return nil, fmt.Errorf("failed to verify %s(%d, %d): %v", newFilePath, dirEntry.Type, dirEntry.Size, err)
	}

	return hardlinks, nil
}
