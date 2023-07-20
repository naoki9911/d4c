package image

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/klauspost/compress/zstd"
	"github.com/naoki9911/fuse-diff-containerd/pkg/di3fs"
	"github.com/sirupsen/logrus"
)

// @return int64: written size
func packDimgContent(logger *logrus.Entry, diffDir string, entry *di3fs.FileEntry, offset int64, out io.Writer) (int64, error) {
	if entry.IsSame() {
		// no need to copy this
		return 0, nil
	}

	if entry.IsSymlink() {
		// nothing to do
		return 0, nil
	}

	if entry.IsDir() {
		curOffset := offset
		for idx := range entry.Childs {
			childDir := &entry.Childs[idx]
			written, err := packDimgContent(logger, filepath.Join(diffDir, entry.Name), childDir, curOffset, out)
			if err != nil {
				return 0, err
			}
			curOffset += written
		}
		return curOffset - offset, nil
	}

	var writtenSize int64
	var err error
	if entry.IsNew() {
		// comress from diffDir
		filePath := filepath.Join(diffDir, entry.Name)
		logger.Debugf("packing %q", filePath)
		writtenSize, err = di3fs.PackFile(filePath, out)
		if err != nil {
			return 0, err
		}
	} else {
		// read and write file
		diffFilePath := filepath.Join(diffDir, entry.DiffName)
		logger.Debugf("packing %q", diffFilePath)
		diffFile, err := os.Open(diffFilePath)
		if err != nil {
			return 0, err
		}
		defer diffFile.Close()
		writtenSize, err = io.Copy(out, diffFile)
		if err != nil {
			return 0, err
		}
	}
	logger.Debugf("pack done(offset=%d size=%d)", offset, writtenSize)

	entry.Offset = offset
	entry.CompressedSize = writtenSize
	entry.DiffName = ""
	return writtenSize, nil
}

func PackDimg(logger *logrus.Entry, diffDir string, entry *di3fs.FileEntry, baseDImgPath string, outputPath string) error {
	outFile, err := os.Create(outputPath)
	if err != nil {
		panic(err)
	}
	defer outFile.Close()

	//entry.print("", false)
	var tmpBuf bytes.Buffer

	baseId := ""
	if baseDImgPath != "" {
		h := sha256.New()
		baseImg, err := os.Open(baseDImgPath)
		if err != nil {
			panic(err)
		}
		_, err = io.Copy(h, baseImg)
		if err != nil {
			panic(err)
		}
		baseId = fmt.Sprintf("sha256:%x", h.Sum(nil))
	}

	_, err = packDimgContent(logger, diffDir, entry, 0, &tmpBuf)
	if err != nil {
		panic(err)
	}
	header := di3fs.ImageHeader{
		BaseId:    baseId,
		FileEntry: *entry,
	}

	jsonBytes, err := json.Marshal(header)
	if err != nil {
		panic(err)
	}

	// encode header
	var headerZstdBuffer bytes.Buffer
	headerZstd, err := zstd.NewWriter(&headerZstdBuffer)
	if err != nil {
		panic(err)
	}
	_, err = headerZstd.Write(jsonBytes)
	if err != nil {
		panic(err)
	}
	err = headerZstd.Close()
	if err != nil {
		panic(err)
	}

	bs := make([]byte, 4)
	binary.LittleEndian.PutUint32(bs, uint32(len(headerZstdBuffer.Bytes())))

	// Image format
	// [ length of compressed image header (4bytes)]
	// [ compressed image header ]
	// [ content body ]

	_, err = outFile.Write(append(bs, headerZstdBuffer.Bytes()...))
	if err != nil {
		return err
	}

	_, err = io.Copy(outFile, &tmpBuf)
	if err != nil {
		return err
	}

	return nil
}
