package image

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"

	"github.com/icedream/go-bsdiff"
	"github.com/naoki9911/fuse-diff-containerd/pkg/utils"
)

func getFileSize(path string) (int, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return 0, err
	}

	return int(fileInfo.Size()), nil
}

func GenerateDiffFromDimg(oldDimgPath, newDimgPath, diffDimgPath string, isBinaryDiff bool) error {
	oldDimg, err := OpenDimgFile(oldDimgPath)
	if err != nil {
		return err
	}
	defer oldDimg.Close()

	newDimg, err := OpenDimgFile(newDimgPath)
	if err != nil {
		return err
	}
	defer newDimg.Close()

	diffFile, err := os.Create(diffDimgPath)
	if err != nil {
		return err
	}
	defer diffFile.Close()

	diffOut := bytes.Buffer{}
	_, err = generateDiffFromDimg(oldDimg, newDimg, &oldDimg.Header().FileEntry, &newDimg.Header().FileEntry, &diffOut, isBinaryDiff)
	if err != nil {
		return err
	}

	h := sha256.New()
	baseImg, err := os.Open(oldDimgPath)
	if err != nil {
		panic(err)
	}
	defer baseImg.Close()
	_, err = io.Copy(h, baseImg)
	if err != nil {
		panic(err)
	}
	baseId := fmt.Sprintf("sha256:%x", h.Sum(nil))

	header := DimgHeader{
		BaseId:    baseId,
		FileEntry: newDimg.header.FileEntry,
	}

	err = WriteDimg(diffFile, &header, &diffOut)
	if err != nil {
		return fmt.Errorf("failed to write dimg: %v", err)
	}

	return nil
}

// @return bool: is entirly new ?
func generateDiffFromDimg(oldDimgFile, newDimgFile *DimgFile, oldEntry, newEntry *FileEntry, diffBody *bytes.Buffer, isBinaryDiff bool) (bool, error) {
	entireNew := true

	for fName := range newEntry.Childs {
		newChildEntry := newEntry.Childs[fName]
		if newChildEntry.Type == FILE_ENTRY_FILE_SAME ||
			newChildEntry.Type == FILE_ENTRY_FILE_DIFF {
			return false, fmt.Errorf("invalid dimg")
		}

		if newChildEntry.Type == FILE_ENTRY_OPAQUE ||
			newChildEntry.Type == FILE_ENTRY_SYMLINK ||
			newChildEntry.Size == 0 {
			continue
		}

		// newly created file or directory
		if oldEntry == nil {
			if newChildEntry.IsDir() {
				_, err := generateDiffFromDimg(oldDimgFile, newDimgFile, nil, newChildEntry, diffBody, isBinaryDiff)
				if err != nil {
					return false, err
				}
			} else {
				newBytes := make([]byte, newChildEntry.CompressedSize)
				_, err := newDimgFile.ReadAt(newBytes, newChildEntry.Offset)
				if err != nil {
					return false, err
				}
				newChildEntry.Offset = int64(len(diffBody.Bytes()))
				_, err = diffBody.Write(newBytes)
				if err != nil {
					return false, err
				}
			}

			continue
		}

		oldChildEntry := oldEntry.Childs[fName]

		// newly created file or directory including unmatched EntryType
		if oldChildEntry == nil ||
			oldChildEntry.Name != newChildEntry.Name ||
			oldChildEntry.Type != newChildEntry.Type {
			if newChildEntry.IsDir() {
				_, err := generateDiffFromDimg(oldDimgFile, newDimgFile, nil, newChildEntry, diffBody, isBinaryDiff)
				if err != nil {
					return false, err
				}
			} else {
				newBytes := make([]byte, newChildEntry.CompressedSize)
				_, err := newDimgFile.ReadAt(newBytes, newChildEntry.Offset)
				if err != nil {
					return false, err
				}
				newChildEntry.Offset = int64(len(diffBody.Bytes()))
				_, err = diffBody.Write(newBytes)
				if err != nil {
					return false, err
				}
			}

			continue
		}

		// if both new and old are directory, recursively generate diff
		if newChildEntry.IsDir() {
			new, err := generateDiffFromDimg(oldDimgFile, newDimgFile, oldChildEntry, newChildEntry, diffBody, isBinaryDiff)
			if err != nil {
				return false, err
			}
			if !new {
				entireNew = false
			}

			continue
		}

		newCompressedBytes := make([]byte, newChildEntry.CompressedSize)
		_, err := newDimgFile.ReadAt(newCompressedBytes, newChildEntry.Offset)
		if err != nil {
			return false, err
		}
		newBytes, err := utils.DecompressWithZstd(newCompressedBytes)
		if err != nil {
			return false, err
		}

		oldCompressedBytes := make([]byte, oldChildEntry.CompressedSize)
		_, err = oldDimgFile.ReadAt(oldCompressedBytes, oldChildEntry.Offset)
		if err != nil {
			return false, err
		}
		oldBytes, err := utils.DecompressWithZstd(oldCompressedBytes)
		if err != nil {
			return false, err
		}
		isSame := bytes.Equal(newBytes, oldBytes)
		if isSame {
			entireNew = false
			newChildEntry.Type = FILE_ENTRY_FILE_SAME
			continue
		}

		// old File may be 0-bytes
		if len(oldBytes) > 0 && isBinaryDiff {
			entireNew = false
			diffWriter := new(bytes.Buffer)
			//fmt.Printf("oldBytes=%d newBytes=%d old=%v new=%v\n", len(oldBytes), len(newBytes), *oldChildEntry, *newChildEntry)
			err = bsdiff.Diff(bytes.NewBuffer(oldBytes), bytes.NewBuffer(newBytes), diffWriter)
			if err != nil {
				return false, err
			}
			newChildEntry.Offset = int64(len(diffBody.Bytes()))
			newChildEntry.CompressedSize = int64(len(diffWriter.Bytes()))
			_, err = diffBody.Write(diffWriter.Bytes())
			if err != nil {
				return false, err
			}
			newChildEntry.Type = FILE_ENTRY_FILE_DIFF
		} else {
			newBytes := make([]byte, newChildEntry.CompressedSize)
			_, err := newDimgFile.ReadAt(newBytes, newChildEntry.Offset)
			if err != nil {
				return false, err
			}
			newChildEntry.Offset = int64(len(diffBody.Bytes()))
			_, err = diffBody.Write(newBytes)
			if err != nil {
				return false, err
			}
			newChildEntry.Type = FILE_ENTRY_FILE_NEW
		}
	}
	if newEntry.IsDir() {
		if entireNew {
			newEntry.Type = FILE_ENTRY_DIR_NEW
		} else {
			newEntry.Type = FILE_ENTRY_DIR
		}
	}
	return entireNew, nil
}
