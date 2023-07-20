package image

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"

	"github.com/icedream/go-bsdiff"
	"github.com/klauspost/compress/zstd"
	"github.com/naoki9911/fuse-diff-containerd/pkg/di3fs"
	"github.com/naoki9911/fuse-diff-containerd/pkg/utils"
	cp "github.com/otiai10/copy"
)

func copyFile(srcFile, dstFile string) error {
	src, err := os.Open(srcFile)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create(dstFile)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	if err != nil {
		return err
	}

	return nil
}

func getFileSize(path string) (int, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return 0, err
	}

	return int(fileInfo.Size()), nil
}

func generateFileHash(filePath string) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	h := sha256.New()
	_, err = io.Copy(h, file)
	if err != nil {
		return nil, err
	}

	return h.Sum(nil), nil
}

// return true if files are same.
func compareFile(fileAPath, fileBPath string) (bool, error) {
	fileAHash, err := generateFileHash(fileAPath)
	if err != nil {
		return false, err
	}
	fileBHash, err := generateFileHash(fileBPath)
	if err != nil {
		return false, err
	}

	for i := range fileAHash {
		if fileAHash[i] != fileBHash[i] {
			return false, nil
		}
	}

	return true, nil
}

func generateFileDiff(baseFilePath, newFilePath, outFilePath string) error {
	baseFile, err := os.Open(baseFilePath)
	if err != nil {
		return err
	}
	defer baseFile.Close()
	newFile, err := os.Open(newFilePath)
	if err != nil {
		return err
	}
	defer newFile.Close()
	outFile, err := os.Create(outFilePath)
	if err != nil {
		return err
	}
	defer outFile.Close()
	err = bsdiff.Diff(baseFile, newFile, outFile)
	if err != nil {
		return err
	}

	return nil
}

// @return bool[0]: is new same as base?
// @return bool[1]: is entirly new ?
// @return bool[2]: is the dir containing opaque dir ?
func generateDiffFromDirImpl(basePath, newPath, outPath string, dirEntry *di3fs.FileEntry, isBinaryDiff, baseExists bool) (bool, bool, []string, error) {
	entireSame := true
	entireNew := true
	baseOk := baseExists

	baseFiles := map[string]fs.DirEntry{}
	if _, err := os.Stat(basePath); err == nil && baseOk {
		baseEntries, _ := os.ReadDir(basePath)
		for i := range baseEntries {
			baseFiles[baseEntries[i].Name()] = baseEntries[i]
		}
	} else {
		baseOk = false
		entireSame = false
	}

	logger.Debugf("newPath:%s\n", outPath)
	newEntries, err := os.ReadDir(newPath)
	if err != nil {
		return false, false, nil, err
	}

	if _, err := os.Stat(outPath); err != nil {
		err = os.Mkdir(outPath, os.ModePerm)
		if err != nil {
			return false, false, nil, err
		}
	}

	childDirs := []fs.DirEntry{}
	isOpaqueDir := false
	for i, entry := range newEntries {
		fName := entry.Name()
		baseFilePath := path.Join(basePath, fName)
		newFilePath := path.Join(newPath, fName)
		outFilePath := path.Join(outPath, fName)
		logger.Debugf("DIFF %s\n", newFilePath)

		if entry.IsDir() {
			childDirs = append(childDirs, newEntries[i])
			continue
		}
		fileInfo, err := entry.Info()
		if err != nil {
			return false, false, nil, err
		}

		// opaque directory
		// https://www.madebymikal.com/interpreting-whiteout-files-in-docker-image-layers/
		// AUFS provided an “opaque directory” that ensured that the directory remained, but all of its previous content was hidden.
		// TODO check filemode
		// https://www.kernel.org/doc/html/latest/filesystems/overlayfs.html#whiteouts-and-opaque-directories
		if fName == ".wh..wh..opq" {
			isOpaqueDir = true
			continue
		}

		entry := di3fs.FileEntry{
			Name:   fName,
			Mode:   uint32(fileInfo.Mode()),
			Childs: []di3fs.FileEntry{},
		}

		if fileInfo.Mode()&os.ModeSymlink == os.ModeSymlink {
			realPath, err := os.Readlink(newFilePath)
			if err != nil {
				return false, false, nil, err
			}
			entireSame = false
			entry.Type = di3fs.FILE_ENTRY_SYMLINK
			entry.RealPath = realPath
			dirEntry.Childs = append(dirEntry.Childs, entry)
			continue
		}

		if fileInfo.Mode()&os.ModeCharDevice == os.ModeCharDevice {
			logger.Infof("Ignore char device:%v\n", newFilePath)
			continue
		}
		if fileInfo.Mode()&os.ModeNamedPipe == os.ModeNamedPipe {
			logger.Infof("Ignore named pipe:%v\n", newFilePath)
			continue
		}

		err = entry.SetUGID(newFilePath)
		if err != nil {
			return false, false, nil, err
		}

		entry.Size, err = getFileSize(newFilePath)
		if err != nil {
			return false, false, nil, err
		}
		// newFile is not dir, but baseFile is dir
		// newFile must be newly created.
		if baseFile, ok := baseFiles[fName]; !ok || baseFile.IsDir() {
			logger.Debugf("NewFile Name:%v\n", newFilePath)
			err = cp.Copy(newFilePath, outFilePath)
			if err != nil {
				return false, false, nil, err
			}
			entry.Type = di3fs.FILE_ENTRY_FILE_NEW
			entry.DiffName = fName
			entireSame = false
		} else {
			isSame, err := compareFile(baseFilePath, newFilePath)
			if err != nil {
				return false, false, nil, err
			}
			if isSame {
				entireNew = false
				entry.Type = di3fs.FILE_ENTRY_FILE_SAME
				logger.Debugf("NotUpdatedFile Name:%v\n", fName)
			} else {
				baseStat, err := os.Stat(baseFilePath)
				if err != nil {
					return false, false, nil, err
				}
				entireSame = false
				if baseStat.Size() == 0 || !isBinaryDiff {
					err = copyFile(newFilePath, outFilePath)
					if err != nil {
						return false, false, nil, err
					}
					entry.Type = di3fs.FILE_ENTRY_FILE_NEW
					entry.DiffName = fName
				} else {
					entireNew = false
					entry.Type = di3fs.FILE_ENTRY_FILE_DIFF
					outFilePath += ".diff"
					entry.DiffName = path.Base(outFilePath)
					logger.Debugf("UpdatedFile Name:%v\n", fName)
					err = generateFileDiff(baseFilePath, newFilePath, outFilePath)
					if err != nil {
						return false, false, nil, err
					}
					logger.Debugf("CreatedDiffFile %v\n", outFilePath)
				}
			}

		}
		dirEntry.Childs = append(dirEntry.Childs, entry)
	}

	opaqueFiles := []string{}
	if isOpaqueDir {
		opaqueFiles = append(opaqueFiles, ".wh..wh..opq")
	}
	for _, childDir := range childDirs {
		childBasePath := path.Join(basePath, childDir.Name())
		childNewPath := path.Join(newPath, childDir.Name())
		childOutPath := path.Join(outPath, childDir.Name())
		entry := di3fs.FileEntry{
			Name:   childDir.Name(),
			Childs: []di3fs.FileEntry{},
		}
		same, new, opaque, err := generateDiffFromDirImpl(childBasePath, childNewPath, childOutPath, &entry, isBinaryDiff, baseOk)
		if err != nil {
			return false, false, nil, err
		}
		if !same {
			entireSame = false
		}
		if !new {
			entireNew = false
		}

		for _, o := range opaque {
			opaqueFiles = append(opaqueFiles, path.Join(childDir.Name(), o))
		}
		dirEntry.Childs = append(dirEntry.Childs, entry)
	}
	fileInfo, err := os.Stat(newPath)
	if err != nil {
		return false, false, nil, nil
	}
	dirEntry.Size = int(fileInfo.Size())
	dirEntry.Mode = uint32(fileInfo.Mode())
	err = dirEntry.SetUGID(newPath)
	if err != nil {
		return false, false, nil, err
	}

	if !entireSame && entireNew {
		dirEntry.Type = di3fs.FILE_ENTRY_DIR_NEW
	} else {
		dirEntry.Type = di3fs.FILE_ENTRY_DIR
	}
	dirEntry.OaqueFiles = opaqueFiles
	return entireSame, entireNew, nil, nil
}

func GenerateDiffFromDir(basePath, newPath, outPath string, isBinaryDiff, baseExists bool) (*di3fs.FileEntry, error) {
	entry := &di3fs.FileEntry{
		Name: "/",
	}
	_, _, _, err := generateDiffFromDirImpl(basePath, newPath, outPath, entry, isBinaryDiff, baseExists)
	return entry, err
}

func GenerateDiffFromDimg(oldDimgPath, newDimgPath, parentDimgPath, diffDimgPath string, isBinaryDiff bool) error {
	oldImg, oldImgFile, oldOffset, err := di3fs.LoadImage(oldDimgPath)
	if err != nil {
		return err
	}
	defer oldImgFile.Close()

	newImg, newImgFile, newOffset, err := di3fs.LoadImage(newDimgPath)
	if err != nil {
		return err
	}
	defer newImgFile.Close()

	diffFile, err := os.Create(diffDimgPath)
	if err != nil {
		return err
	}
	defer diffFile.Close()

	diffOut := bytes.Buffer{}
	_, err = generateDiffFromDimg(oldImgFile, oldOffset, &oldImg.FileEntry, newImgFile, newOffset, &newImg.FileEntry, &diffOut, isBinaryDiff)
	if err != nil {
		return err
	}
	baseId := ""
	if parentDimgPath != "" {
		h := sha256.New()
		baseImg, err := os.Open(parentDimgPath)
		if err != nil {
			panic(err)
		}
		_, err = io.Copy(h, baseImg)
		if err != nil {
			panic(err)
		}
		baseId = fmt.Sprintf("sha256:%x", h.Sum(nil))
	}
	header := di3fs.ImageHeader{
		BaseId:    baseId,
		FileEntry: newImg.FileEntry,
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
	// [ length of compressed image header (4bit)]
	// [ compressed image header ]
	// [ content body ]

	_, err = diffFile.Write(append(bs, headerZstdBuffer.Bytes()...))
	if err != nil {
		return err
	}

	_, err = io.Copy(diffFile, &diffOut)
	if err != nil {
		return err
	}

	return nil
}

// @return bool: is entirly new ?
func generateDiffFromDimg(oldImgFile *os.File, oldOffset int64, oldEntry *di3fs.FileEntry, newImgFile *os.File, newOffset int64, newEntry *di3fs.FileEntry, diffBody *bytes.Buffer, isBinaryDiff bool) (bool, error) {
	entireNew := true

	for i := range newEntry.Childs {
		newChildEntry := &newEntry.Childs[i]
		if newChildEntry.Type == di3fs.FILE_ENTRY_FILE_SAME ||
			newChildEntry.Type == di3fs.FILE_ENTRY_FILE_DIFF {
			return false, fmt.Errorf("invalid dimg")
		}

		if newChildEntry.Type == di3fs.FILE_ENTRY_OPAQUE ||
			newChildEntry.Type == di3fs.FILE_ENTRY_SYMLINK ||
			newChildEntry.Size == 0 {
			continue
		}

		// newly created file or directory
		if oldEntry == nil {
			if newChildEntry.IsDir() {
				_, err := generateDiffFromDimg(oldImgFile, oldOffset, nil, newImgFile, newOffset, &newEntry.Childs[i], diffBody, isBinaryDiff)
				if err != nil {
					return false, err
				}
			} else {
				newBytes := make([]byte, newChildEntry.CompressedSize)
				_, err := newImgFile.ReadAt(newBytes, newOffset+newChildEntry.Offset)
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

		oldChildIdx := 0
		for oldChildIdx < len(oldEntry.Childs) && oldEntry.Childs[oldChildIdx].Name != newChildEntry.Name {
			oldChildIdx += 1
		}
		oldChildEntry := oldEntry.Childs[oldChildIdx]

		// newly created file or directory including unmatched EntryType
		if oldChildEntry.Name != newChildEntry.Name ||
			oldChildEntry.Type != newChildEntry.Type {
			if newChildEntry.IsDir() {
				_, err := generateDiffFromDimg(oldImgFile, oldOffset, nil, newImgFile, newOffset, &newEntry.Childs[i], diffBody, isBinaryDiff)
				if err != nil {
					return false, err
				}
			} else {
				newBytes := make([]byte, newChildEntry.CompressedSize)
				_, err := newImgFile.ReadAt(newBytes, newOffset+newChildEntry.Offset)
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
			new, err := generateDiffFromDimg(oldImgFile, oldOffset, &oldEntry.Childs[oldChildIdx], newImgFile, newOffset, &newEntry.Childs[i], diffBody, isBinaryDiff)
			if err != nil {
				return false, err
			}
			if !new {
				entireNew = false
			}

			continue
		}

		newCompressedBytes := make([]byte, newChildEntry.CompressedSize)
		_, err := newImgFile.ReadAt(newCompressedBytes, newOffset+newChildEntry.Offset)
		if err != nil {
			return false, err
		}
		newBytes, err := utils.DecompressWithZstd(newCompressedBytes)
		if err != nil {
			return false, err
		}

		oldCompressedBytes := make([]byte, oldChildEntry.CompressedSize)
		_, err = oldImgFile.ReadAt(oldCompressedBytes, oldOffset+oldChildEntry.Offset)
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
			newChildEntry.Type = di3fs.FILE_ENTRY_FILE_SAME
			continue
		}

		if isBinaryDiff {
			entireNew = false
			diffWriter := new(bytes.Buffer)
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
			newChildEntry.Type = di3fs.FILE_ENTRY_FILE_DIFF
		} else {
			newBytes := make([]byte, newChildEntry.CompressedSize)
			_, err := newImgFile.ReadAt(newBytes, newOffset+newChildEntry.Offset)
			if err != nil {
				return false, err
			}
			newChildEntry.Offset = int64(len(diffBody.Bytes()))
			_, err = diffBody.Write(newBytes)
			if err != nil {
				return false, err
			}
			newChildEntry.Type = di3fs.FILE_ENTRY_FILE_NEW
		}
	}
	if newEntry.IsDir() && entireNew {
		newEntry.Type = di3fs.FILE_ENTRY_DIR_NEW
	}
	return entireNew, nil
}
