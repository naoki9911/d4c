package image

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path"

	"github.com/klauspost/compress/zstd"
	"github.com/naoki9911/fuse-diff-containerd/pkg/di3fs"
)

func compressFileWithZstd(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	fileBytes, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}

	res, err := di3fs.CompressWithZstd(fileBytes)
	if err != nil {
		return nil, err
	}

	return res, nil

}

func PackFile(srcFilePath string, out io.Writer) (int64, error) {
	compressed, err := compressFileWithZstd(srcFilePath)
	if err != nil {
		return 0, err
	}
	writtenSize, err := out.Write(compressed)
	if err != nil {
		return 0, err
	}

	return int64(writtenSize), err
}

func packDirImpl(dirPath string, outDirEntry *di3fs.FileEntry, outBody *bytes.Buffer) error {
	logger.Debugf("dirPath:%s\n", dirPath)
	dirEntries, err := os.ReadDir(dirPath)
	if err != nil {
		return err
	}

	childDirs := []fs.DirEntry{}
	isOpaqueDir := false
	for i, entry := range dirEntries {
		fName := entry.Name()
		dirFilePath := path.Join(dirPath, fName)
		logger.Debugf("DIFF %s\n", dirFilePath)

		if entry.IsDir() {
			childDirs = append(childDirs, dirEntries[i])
			continue
		}
		fileInfo, err := entry.Info()
		if err != nil {
			return err
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

		entry := &di3fs.FileEntry{
			Name:   fName,
			Mode:   uint32(fileInfo.Mode()),
			Childs: map[string]*di3fs.FileEntry{},
		}

		if fileInfo.Mode()&os.ModeSymlink == os.ModeSymlink {
			realPath, err := os.Readlink(dirFilePath)
			if err != nil {
				return err
			}
			entry.Type = di3fs.FILE_ENTRY_SYMLINK
			entry.RealPath = realPath
			outDirEntry.Childs[fName] = entry
			continue
		}

		if fileInfo.Mode()&os.ModeCharDevice == os.ModeCharDevice {
			logger.Infof("Ignore char device:%v\n", dirFilePath)
			continue
		}
		if fileInfo.Mode()&os.ModeNamedPipe == os.ModeNamedPipe {
			logger.Infof("Ignore named pipe:%v\n", dirFilePath)
			continue
		}

		err = entry.SetUGID(dirFilePath)
		if err != nil {
			return err
		}

		entry.Size, err = getFileSize(dirFilePath)
		if err != nil {
			return err
		}

		logger.Debugf("NewFile Name:%v\n", dirFilePath)
		entry.Offset = int64(len(outBody.Bytes()))
		writtenSize, err := PackFile(dirFilePath, outBody)
		if err != nil {
			return err
		}
		entry.CompressedSize = writtenSize
		entry.Type = di3fs.FILE_ENTRY_FILE_NEW
		outDirEntry.Childs[fName] = entry
	}

	opaqueFiles := []string{}
	if isOpaqueDir {
		opaqueFiles = append(opaqueFiles, ".wh..wh..opq")
	}
	for _, childDir := range childDirs {
		childDirPath := path.Join(dirPath, childDir.Name())
		entry := &di3fs.FileEntry{
			Name:   childDir.Name(),
			Childs: map[string]*di3fs.FileEntry{},
		}
		err = packDirImpl(childDirPath, entry, outBody)
		if err != nil {
			return err
		}
		outDirEntry.Childs[childDir.Name()] = entry
	}
	fileInfo, err := os.Stat(dirPath)
	if err != nil {
		return err
	}
	outDirEntry.Size = int(fileInfo.Size())
	outDirEntry.Mode = uint32(fileInfo.Mode())
	err = outDirEntry.SetUGID(dirPath)
	if err != nil {
		return err
	}

	outDirEntry.Type = di3fs.FILE_ENTRY_DIR_NEW
	outDirEntry.OaqueFiles = opaqueFiles
	return nil
}

func PackDir(dirPath, outDimgPath string) error {
	entry := &di3fs.FileEntry{
		Name:   "/",
		Childs: map[string]*di3fs.FileEntry{},
	}
	outDimg, err := os.Create(outDimgPath)
	if err != nil {
		return err
	}
	defer outDimg.Close()

	outBody := bytes.Buffer{}
	err = packDirImpl(dirPath, entry, &outBody)
	if err != nil {
		return err
	}

	header := di3fs.ImageHeader{
		BaseId:    "",
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

	_, err = outDimg.Write(append(bs, headerZstdBuffer.Bytes()...))
	if err != nil {
		return err
	}

	_, err = io.Copy(outDimg, &outBody)
	if err != nil {
		return err
	}
	return nil
}
