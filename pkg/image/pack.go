package image

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
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

	res, err := CompressWithZstd(fileBytes)
	if err != nil {
		return nil, err
	}

	return res, nil

}

func packFile(srcFilePath string, out io.Writer) (int64, error) {
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

func packDirImpl(dirPath string, outDirEntry *FileEntry, outBody *bytes.Buffer) error {
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

		entry := &FileEntry{
			Name:   fName,
			Mode:   uint32(fileInfo.Mode()),
			Childs: map[string]*FileEntry{},
		}

		if fileInfo.Mode()&os.ModeSymlink == os.ModeSymlink {
			realPath, err := os.Readlink(dirFilePath)
			if err != nil {
				return err
			}
			entry.Type = FILE_ENTRY_SYMLINK
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
		writtenSize, err := packFile(dirFilePath, outBody)
		if err != nil {
			return err
		}
		entry.CompressedSize = writtenSize
		entry.Type = FILE_ENTRY_FILE_NEW
		outDirEntry.Childs[fName] = entry
	}

	opaqueFiles := []string{}
	if isOpaqueDir {
		opaqueFiles = append(opaqueFiles, ".wh..wh..opq")
	}
	for _, childDir := range childDirs {
		childDirPath := path.Join(dirPath, childDir.Name())
		entry := &FileEntry{
			Name:   childDir.Name(),
			Childs: map[string]*FileEntry{},
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

	outDirEntry.Type = FILE_ENTRY_DIR_NEW
	outDirEntry.OaqueFiles = opaqueFiles
	return nil
}

func PackDir(dirPath, outDimgPath string) error {
	entry := &FileEntry{
		Name:   "/",
		Childs: map[string]*FileEntry{},
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

	header := DimgHeader{
		BaseId:    "",
		FileEntry: *entry,
	}

	err = WriteDimg(outDimg, &header, &outBody)
	if err != nil {
		return fmt.Errorf("faield to write dimg: %v", err)
	}
	return nil
}
