package image

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"sync"

	"github.com/opencontainers/go-digest"
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

type packTask struct {
	entry *FileEntry
	path  string
	data  *bytes.Buffer
}

func packDirImplMultithread(dirPath string, outDirEntry *FileEntry, outBody *bytes.Buffer, threadNum int) error {
	compressTasks := make(chan packTask, 1000)
	writeTasks := make(chan packTask, 1000)
	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Info("started pack enqueu thread")
		err := enqueuePackTaskToChannel(dirPath, outDirEntry, compressTasks)
		if err != nil {
			logger.Errorf("failed to enque: %v", err)
		}
		close(compressTasks)
		logger.Info("finished pack enqueu thread")
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Info("started pack write thread")
		for {
			wt, more := <-writeTasks
			if !more {
				break
			}
			wt.entry.Offset = int64(outBody.Len())
			_, err := io.Copy(outBody, wt.data)
			if err != nil {
				logger.Errorf("failed to copy to outBody: %v", err)
				return
			}
		}
		logger.Info("finished pack write thread")
	}()

	compWg := sync.WaitGroup{}
	for i := 0; i < threadNum; i++ {
		wg.Add(1)
		compWg.Add(1)
		go func(threadId int) {
			logger.Infof("started pack compress thread idx=%d", threadId)
			defer wg.Done()
			defer compWg.Done()
			for {
				ct, more := <-compressTasks
				if !more {
					break
				}
				outBuffer := bytes.Buffer{}
				writtenSize, err := packFile(ct.path, &outBuffer)
				if err != nil {
					logger.Errorf("failed to pack file %s: %v", ct.path, err)
					break
				}
				ct.entry.CompressedSize = writtenSize
				ct.entry.Type = FILE_ENTRY_FILE_NEW
				ct.data = &outBuffer
				writeTasks <- ct
			}
			logger.Infof("finished pack compress thread idx=%d", threadId)
		}(i)
	}

	go func() {
		compWg.Wait()
		close(writeTasks)
		logger.Infof("all compression tasks finished")
	}()

	wg.Wait()
	return nil
}

func enqueuePackTaskToChannel(dirPath string, parentEntry *FileEntry, taskChan chan packTask) error {
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
			parentEntry.Childs[fName] = entry
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

		parentEntry.Childs[fName] = entry
		taskChan <- packTask{
			entry: entry,
			path:  dirFilePath,
			data:  nil,
		}
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
		err = enqueuePackTaskToChannel(childDirPath, entry, taskChan)
		if err != nil {
			return err
		}
		parentEntry.Childs[childDir.Name()] = entry
	}
	fileInfo, err := os.Stat(dirPath)
	if err != nil {
		return err
	}
	parentEntry.Size = int(fileInfo.Size())
	parentEntry.Mode = uint32(fileInfo.Mode())
	err = parentEntry.SetUGID(dirPath)
	if err != nil {
		return err
	}

	parentEntry.Type = FILE_ENTRY_DIR_NEW
	parentEntry.OaqueFiles = opaqueFiles
	return nil
}

func PackDir(dirPath, outDimgPath string, threadNum int) error {
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
	err = packDirImplMultithread(dirPath, entry, &outBody, threadNum)
	if err != nil {
		return err
	}
	//jsonBytes, _ := json.MarshalIndent(entry, " ", " ")
	//fmt.Println(string(jsonBytes))
	bodyDigest := digest.FromBytes(outBody.Bytes())

	header := DimgHeader{
		Id:        bodyDigest,
		ParentId:  digest.Digest(""),
		FileEntry: *entry,
	}

	err = WriteDimg(outDimg, &header, &outBody)
	if err != nil {
		return fmt.Errorf("faield to write dimg: %v", err)
	}
	return nil
}
