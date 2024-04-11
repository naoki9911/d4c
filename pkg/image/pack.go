package image

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sync"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
)

type packTask struct {
	entry *FileEntry
	data  *bytes.Buffer
}

func packDirImplMultithread(dirPath string, layer v1.Layer, outDirEntry *FileEntry, outBody *bytes.Buffer, threadNum int) error {
	compressTasks := make(chan packTask, 1000)
	writeTasks := make(chan packTask, 1000)
	wg := sync.WaitGroup{}

	if layer != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			logger.Info("started pack enqueu thread")
			err := enqueuePackTaskToChannelFromLayer(layer, outDirEntry, compressTasks)
			if err != nil {
				logger.Errorf("failed to enque: %v", err)
			}
			close(compressTasks)
			logger.Info("finished pack enqueu thread")
		}()
	} else {
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
	}

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
				outBuffer, err := compressWithZstdIo(ct.data)
				if err != nil {
					logger.Errorf("failed to pack file %s: %v", ct.entry.Name, err)
					break
				}
				ct.entry.CompressedSize = int64(outBuffer.Len())
				ct.entry.Type = FILE_ENTRY_FILE_NEW
				ct.data = outBuffer
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

		// ignore 'char', 'fifo', 'socket' and whiteout file
		if fileInfo.Mode()&os.ModeCharDevice == os.ModeCharDevice ||
			fileInfo.Mode()&os.ModeNamedPipe == os.ModeNamedPipe ||
			fileInfo.Mode()&os.ModeSocket == os.ModeSocket ||
			fName == ".wh..wh..opq" {
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

		err = entry.SetUGID(dirFilePath)
		if err != nil {
			return err
		}

		entry.Size, err = getFileSize(dirFilePath)
		if err != nil {
			return err
		}

		fileBody, err := readFileAll(dirFilePath)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %v", dirFilePath, err)
		}

		parentEntry.Childs[fName] = entry
		taskChan <- packTask{
			entry: entry,
			data:  bytes.NewBuffer(fileBody),
		}
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
	return nil
}

func enqueuePackTaskToChannelFromLayer(layer v1.Layer, rootEntry *FileEntry, taskChan chan packTask) error {
	uncomp, err := layer.Uncompressed()
	if err != nil {
		return fmt.Errorf("failed to get uncompressed layer: %v", err)
	}
	tarReader := tar.NewReader(uncomp)

	// key is directory path
	files := map[string]*FileEntry{}
	rootEntry.Type = FILE_ENTRY_DIR_NEW
	files["."] = rootEntry

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read next header: %v", err)
		}

		dirname := filepath.Dir(header.Name)
		basename := filepath.Base(header.Name)
		logger.WithFields(logrus.Fields{"dirname": dirname, "basename": basename}).Debugf("pack %s", header.Name)

		dirEntry, ok := files[dirname]
		if !ok {
			return fmt.Errorf("directory %s not found", dirname)
		}
		entry := &FileEntry{
			Name: basename,
			Mode: uint32(header.Mode),
			Size: int(header.Size),
			UID:  uint32(header.Uid),
			GID:  uint32(header.Gid),
		}
		switch header.Typeflag {
		case tar.TypeDir:
			entry.Type = FILE_ENTRY_DIR_NEW
			entry.Childs = map[string]*FileEntry{}
			dirEntry.Childs[basename] = entry
			files[header.Name] = entry
		case tar.TypeReg:
			entry.Type = FILE_ENTRY_FILE_NEW
			if entry.Size != 0 {
				data := bytes.Buffer{}
				_, err = io.Copy(&data, tarReader)
				if err != nil {
					return fmt.Errorf("failed to copy %s: %v", header.Name, err)
				}
				taskChan <- packTask{
					entry: entry,
					data:  &data,
				}
			}
			dirEntry.Childs[basename] = entry
			files[header.Name] = entry
		case tar.TypeSymlink:
			entry.Type = FILE_ENTRY_SYMLINK
			entry.RealPath = header.Linkname
			dirEntry.Childs[basename] = entry
			files[header.Name] = entry
		case tar.TypeLink:
			entry.Type = FILE_ENTRY_HARDLINK
			entry.RealPath = header.Linkname
			dirEntry.Childs[basename] = entry
			files[header.Name] = entry
		case tar.TypeBlock, tar.TypeChar, tar.TypeFifo:
			continue
		default:
			return fmt.Errorf("file %s has unexpected type flag: %d", header.Name, header.Typeflag)
		}
	}
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
	err = packDirImplMultithread(dirPath, nil, entry, &outBody, threadNum)
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

func PackLayer(layer v1.Layer, outDimgPath string, threadNum int) error {
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
	err = packDirImplMultithread("", layer, entry, &outBody, threadNum)
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
