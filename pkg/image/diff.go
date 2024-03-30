package image

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sync"

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

func GenerateDiffFromDimg(oldDimgPath, newDimgPath, diffDimgPath string, isBinaryDiff bool, dc DiffMultihreadConfig) error {
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
	err = generateDiffMultithread(oldDimg, newDimg, &oldDimg.Header().FileEntry, &newDimg.Header().FileEntry, &diffOut, isBinaryDiff, dc)
	if err != nil {
		return err
	}

	header := DimgHeader{
		Id:        newDimg.Header().Id,
		ParentId:  oldDimg.Header().Id,
		FileEntry: newDimg.header.FileEntry,
	}

	err = WriteDimg(diffFile, &header, &diffOut)
	if err != nil {
		return fmt.Errorf("failed to write dimg: %v", err)
	}

	return nil
}

func GenerateDiffFromCdimg(oldCdimgPath, newCdimgPath, diffCdimgPath string, isBinaryDiff bool, dc DiffMultihreadConfig) error {
	oldCdimg, err := OpenCdimgFile(oldCdimgPath)
	if err != nil {
		return err
	}
	defer oldCdimg.Close()
	oldDimg := oldCdimg.Dimg

	newCdimg, err := OpenCdimgFile(newCdimgPath)
	if err != nil {
		return err
	}
	defer newCdimg.Close()
	newDimg := newCdimg.Dimg

	diffCdimg, err := os.Create(diffCdimgPath)
	if err != nil {
		return err
	}
	defer diffCdimg.Close()

	diffOut := bytes.Buffer{}
	err = generateDiffMultithread(oldDimg, newDimg, &oldDimg.Header().FileEntry, &newDimg.Header().FileEntry, &diffOut, isBinaryDiff, dc)
	if err != nil {
		return err
	}

	diffDimgOut := bytes.Buffer{}
	header := DimgHeader{
		Id:        newDimg.Header().Id,
		ParentId:  oldDimg.Header().Id,
		FileEntry: newDimg.header.FileEntry,
	}
	err = WriteDimg(&diffDimgOut, &header, &diffOut)
	if err != nil {
		return fmt.Errorf("failed to write dimg: %v", err)
	}

	err = WriteCdimgHeader(bytes.NewBuffer(newCdimg.Header.ConfigBytes), &header, int64(diffDimgOut.Len()), diffCdimg)
	if err != nil {
		return fmt.Errorf("failed to cdimg header: %v", err)
	}
	_, err = io.Copy(diffCdimg, &diffDimgOut)
	if err != nil {
		return fmt.Errorf("failed to write dimg: %v", err)
	}

	return nil
}

type diffTask struct {
	oldEntry *FileEntry
	newEntry *FileEntry
	data     []byte
}

const (
	DIFF_MULTI_SCHED_NONE         = "none"
	DIFF_MULTI_SCHED_SIZE_ORDERED = "size-ordered"
)

type DiffMultihreadConfig struct {
	ThreadNum    int
	ScheduleMode string
}

func (dc *DiffMultihreadConfig) Validate() error {
	if dc.ThreadNum <= 0 {
		return fmt.Errorf("invalid ThreadNum: %d", dc.ThreadNum)
	}

	if dc.ScheduleMode != DIFF_MULTI_SCHED_NONE && dc.ScheduleMode != DIFF_MULTI_SCHED_SIZE_ORDERED {
		return fmt.Errorf("invalid ScheduleMode: %s", dc.ScheduleMode)
	}
	return nil
}

func generateDiffMultithread(oldDimgFile, newDimgFile *DimgFile, oldEntry, newEntry *FileEntry, diffBody *bytes.Buffer, isBinaryDiff bool, dc DiffMultihreadConfig) error {
	diffTasks := make(chan diffTask, 1000)
	writeTasks := make(chan diffTask, 1000)
	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Info("started diff task enqueu thread")
		err := enqueueDiffTaskToChannel(oldDimgFile, newDimgFile, oldEntry, newEntry, diffTasks)
		if err != nil {
			logger.Errorf("failed to enque: %v", err)
		}
		close(diffTasks)
		logger.Info("finished diff task enqueu thread")
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Info("started diff write thread")
		for {
			wt, more := <-writeTasks
			if !more {
				break
			}
			wt.newEntry.Offset = int64(diffBody.Len())
			_, err := diffBody.Write(wt.data)
			if err != nil {
				logger.Errorf("failed to write to diffBody: %v", err)
				return
			}
		}
		logger.Info("finished diff write thread")
	}()

	compWg := sync.WaitGroup{}
	for i := 0; i < dc.ThreadNum; i++ {
		wg.Add(1)
		compWg.Add(1)
		go func(threadId int) {
			logger.Infof("started diff thread idx=%d", threadId)
			defer wg.Done()
			defer compWg.Done()
			for {
				dt, more := <-diffTasks
				if !more {
					break
				}

				if dt.oldEntry == nil {
					dt.data = make([]byte, dt.newEntry.CompressedSize)
					_, err := newDimgFile.ReadAt(dt.data, dt.newEntry.Offset)
					if err != nil {
						logger.Errorf("failed to read from newDimgFile at 0x%x: %v", dt.newEntry.Offset, err)
						break
					}
				} else {
					newCompressedBytes := make([]byte, dt.newEntry.CompressedSize)
					_, err := newDimgFile.ReadAt(newCompressedBytes, dt.newEntry.Offset)
					if err != nil {
						logger.Errorf("failed to read from newDimgFile at 0x%x: %v", dt.newEntry.Offset, err)
						break
					}
					newBytes, err := utils.DecompressWithZstd(newCompressedBytes)
					if err != nil {
						logger.Errorf("failed to decompress newBytes: %v", err)
						break
					}

					oldCompressedBytes := make([]byte, dt.oldEntry.CompressedSize)
					_, err = oldDimgFile.ReadAt(oldCompressedBytes, dt.oldEntry.Offset)
					if err != nil {
						logger.Errorf("failed to read from oldDimgFile at 0x%x: %v", dt.oldEntry.Offset, err)
						break
					}
					oldBytes, err := utils.DecompressWithZstd(oldCompressedBytes)
					if err != nil {
						logger.Errorf("failed to decompress oldBytes: %v", err)
						break
					}
					isSame := bytes.Equal(newBytes, oldBytes)
					if isSame {
						dt.newEntry.Type = FILE_ENTRY_FILE_SAME
						dt.newEntry.CompressedSize = 0
						continue
					}
					if len(oldBytes) > 0 && isBinaryDiff {
						// old File may be 0-bytes
						diffWriter := new(bytes.Buffer)
						//fmt.Printf("oldBytes=%d newBytes=%d old=%v new=%v\n", len(oldBytes), len(newBytes), *oldChildEntry, *newChildEntry)
						err = bsdiff.Diff(bytes.NewBuffer(oldBytes), bytes.NewBuffer(newBytes), diffWriter)
						if err != nil {
							logger.Errorf("failed to bsdiff.Diff: %v", err)
							break
						}
						dt.newEntry.Type = FILE_ENTRY_FILE_DIFF
						dt.newEntry.CompressedSize = int64(diffWriter.Len())
						dt.data = diffWriter.Bytes()
					} else {
						dt.newEntry.Type = FILE_ENTRY_FILE_NEW
						dt.data = make([]byte, dt.newEntry.CompressedSize)
						_, err := newDimgFile.ReadAt(dt.data, dt.newEntry.Offset)
						if err != nil {
							logger.Errorf("failed to read from newDimgFile at 0x%x: %v", dt.newEntry.Offset, err)
							break
						}
					}
				}
				writeTasks <- dt
			}
			logger.Infof("finished diff thread idx=%d", threadId)
		}(i)
	}

	go func() {
		compWg.Wait()
		close(writeTasks)
		logger.Infof("all diff tasks finished")
	}()

	wg.Wait()

	logger.Info("started to update dir entry")
	updateDirFileEntry(newEntry)
	logger.Info("finished to update dir entry")
	return nil
}

// updates FileEntry.Type to FILE_ENTRY_DIR or FILE_ENTRY_DIR_NEW
func updateDirFileEntry(entry *FileEntry) {
	if !entry.IsDir() {
		return
	}

	entireNew := true
	for _, childEntry := range entry.Childs {
		if childEntry.IsDir() {
			updateDirFileEntry(childEntry)
		}

		if !childEntry.IsNew() {
			entireNew = false
		}
	}

	if entireNew {
		entry.Type = FILE_ENTRY_DIR_NEW
	} else {
		entry.Type = FILE_ENTRY_DIR
	}
}

func enqueueDiffTaskToChannel(oldDimgFile, newDimgFile *DimgFile, oldEntry, newEntry *FileEntry, taskChan chan diffTask) error {
	for fName := range newEntry.Childs {
		newChildEntry := newEntry.Childs[fName]
		if newChildEntry.Type == FILE_ENTRY_FILE_SAME ||
			newChildEntry.Type == FILE_ENTRY_FILE_DIFF {
			return fmt.Errorf("invalid dimg")
		}

		if newChildEntry.Type == FILE_ENTRY_OPAQUE ||
			newChildEntry.Type == FILE_ENTRY_SYMLINK ||
			newChildEntry.Size == 0 {
			continue
		}

		// newly created file or directory
		if oldEntry == nil {
			if newChildEntry.IsDir() {
				err := enqueueDiffTaskToChannel(oldDimgFile, newDimgFile, nil, newChildEntry, taskChan)
				if err != nil {
					return err
				}
			} else {
				taskChan <- diffTask{
					oldEntry: nil,
					newEntry: newChildEntry,
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
				err := enqueueDiffTaskToChannel(oldDimgFile, newDimgFile, nil, newChildEntry, taskChan)
				if err != nil {
					return err
				}
			} else {
				taskChan <- diffTask{
					oldEntry: nil,
					newEntry: newChildEntry,
				}
			}

			continue
		}

		// if both new and old are directory, recursively generate diff
		if newChildEntry.IsDir() {
			err := enqueueDiffTaskToChannel(oldDimgFile, newDimgFile, oldChildEntry, newChildEntry, taskChan)
			if err != nil {
				return err
			}

			continue
		}

		taskChan <- diffTask{
			oldEntry: oldChildEntry,
			newEntry: newChildEntry,
		}
	}
	return nil
}
