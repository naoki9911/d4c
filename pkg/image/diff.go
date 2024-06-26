package image

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/naoki9911/fuse-diff-containerd/pkg/benchmark"
	"github.com/naoki9911/fuse-diff-containerd/pkg/bsdiffx"
	"github.com/naoki9911/fuse-diff-containerd/pkg/utils"
)

func getFileSize(path string) (int, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return 0, err
	}

	return int(fileInfo.Size()), nil
}

func readFileAll(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return io.ReadAll(file)
}

func GenerateDiffFromDimg(oldDimgPath, newDimgPath, diffDimgPath string, isBinaryDiff bool, dc DiffConfig, pm *bsdiffx.PluginManager) error {
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

	diffTmpFile, err := os.CreateTemp("", "*")
	if err != nil {
		return err
	}
	defer os.Remove(diffTmpFile.Name())
	defer diffTmpFile.Close()

	err = generateDiffMultithread(oldDimg, newDimg, &oldDimg.DimgHeader().FileEntry, &newDimg.DimgHeader().FileEntry, diffTmpFile, isBinaryDiff, dc, pm)
	if err != nil {
		return err
	}

	header := DimgHeader{
		Id:              newDimg.DimgHeader().Id,
		ParentId:        oldDimg.DimgHeader().Id,
		CompressionMode: dc.CompressionMode,
		FileEntry:       newDimg.header.FileEntry,
	}

	_, err = diffTmpFile.Seek(0, 0)
	if err != nil {
		return err
	}
	err = WriteDimg(diffFile, &header, diffTmpFile)
	if err != nil {
		return fmt.Errorf("failed to write dimg: %v", err)
	}

	return nil
}

func GenerateDiffFromCdimg(oldCdimgPath, newCdimgPath, diffCdimgPath string, isBinaryDiff bool, dc DiffConfig, pm *bsdiffx.PluginManager) error {
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

	diffTmpFile, err := os.CreateTemp("", "*")
	if err != nil {
		return err
	}
	defer os.Remove(diffTmpFile.Name())
	defer diffTmpFile.Close()

	err = generateDiffMultithread(oldDimg, newDimg, &oldDimg.DimgHeader().FileEntry, &newDimg.DimgHeader().FileEntry, diffTmpFile, isBinaryDiff, dc, pm)
	if err != nil {
		return err
	}

	diffDimgOut := bytes.Buffer{}
	header := DimgHeader{
		Id:              newDimg.DimgHeader().Id,
		ParentId:        oldDimg.DimgHeader().Id,
		CompressionMode: dc.CompressionMode,
		FileEntry:       newDimg.header.FileEntry,
	}

	_, err = diffTmpFile.Seek(0, 0)
	if err != nil {
		return err
	}
	err = WriteDimg(&diffDimgOut, &header, diffTmpFile)
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

type DiffConfig struct {
	ThreadNum        int
	ScheduleMode     string
	CompressionMode  bsdiffx.CompressionMode
	BenchmarkPerFile bool
	Benchmarker      *benchmark.Benchmark
	DeltaEncoding    string
}

func (dc *DiffConfig) Validate() error {
	if dc.ThreadNum <= 0 {
		return fmt.Errorf("invalid ThreadNum: %d", dc.ThreadNum)
	}

	if dc.ScheduleMode != DIFF_MULTI_SCHED_NONE && dc.ScheduleMode != DIFF_MULTI_SCHED_SIZE_ORDERED {
		return fmt.Errorf("invalid ScheduleMode: %s", dc.ScheduleMode)
	}

	return nil
}

type diffTaskQueue struct {
	taskChan  chan diffTask
	taskArray []diffTask
	wgQ       sync.WaitGroup
}

func newDiffTaskQueue() *diffTaskQueue {
	dq := &diffTaskQueue{
		wgQ: sync.WaitGroup{},
	}
	dq.wgQ.Add(1)
	return dq
}

func (dq *diffTaskQueue) Enqueue(dt diffTask) {
	if dq.taskChan != nil {
		dq.taskChan <- dt
	} else {
		dq.taskArray = append(dq.taskArray, dt)
	}
}

func (dq *diffTaskQueue) Close() {
	dq.wgQ.Done()
	if dq.taskChan != nil {
		close(dq.taskChan)
	}
}

func generateDiffMultithread(oldDimgFile, newDimgFile *DimgFile, oldEntry, newEntry *FileEntry, diffWriter io.Writer, isBinaryDiff bool, dc DiffConfig, pm *bsdiffx.PluginManager) error {
	diffTasks := make(chan diffTask, 10)
	writeTasks := make(chan diffTask, 10)
	wg := sync.WaitGroup{}

	diffTaskQueue := newDiffTaskQueue()
	if dc.ScheduleMode == DIFF_MULTI_SCHED_NONE {
		diffTaskQueue.taskChan = diffTasks
	} else if dc.ScheduleMode == DIFF_MULTI_SCHED_SIZE_ORDERED {
		diffTaskQueue.taskArray = make([]diffTask, 0)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()

		logger.Info("started diff task enqueu thread")
		err := enqueueDiffTaskToQueue(oldDimgFile, newDimgFile, oldEntry, newEntry, diffTaskQueue)
		if err != nil {
			logger.Errorf("failed to enque: %v", err)
		}
		diffTaskQueue.Close()
		logger.Info("finished diff task enqueu thread")
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		diffTaskQueue.wgQ.Wait()
		if diffTaskQueue.taskArray == nil {
			return
		}
		if dc.ScheduleMode == DIFF_MULTI_SCHED_SIZE_ORDERED {
			// process larger file first
			sort.Slice(diffTaskQueue.taskArray, func(i int, j int) bool {
				return diffTaskQueue.taskArray[i].newEntry.Size > diffTaskQueue.taskArray[j].newEntry.Size
			})
			//logger.Infof("%d %d %d", diffTaskQueue.taskArray[0].newEntry.Size, diffTaskQueue.taskArray[1].newEntry.Size, diffTaskQueue.taskArray[2].newEntry.Size)
			logger.Infof("task was ordered in size")
		}

		for i, t := range diffTaskQueue.taskArray {
			// files to generate diffs first
			if t.oldEntry != nil {
				diffTasks <- diffTaskQueue.taskArray[i]
			}
		}

		for i, t := range diffTaskQueue.taskArray {
			if t.oldEntry == nil {
				diffTasks <- diffTaskQueue.taskArray[i]
			}
		}
		close(diffTasks)
		logger.Info("all task was sent to diff channel")
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Info("started diff write thread")
		diffCount := 0
		newCount := 0
		sameCount := 0
		offset := int64(0)
		for {
			wt, more := <-writeTasks
			if !more {
				break
			}
			len := int64(len(wt.data))
			wt.newEntry.Offset = offset
			_, err := diffWriter.Write(wt.data)
			offset += len
			if err != nil {
				logger.Errorf("failed to write to diffBody: %v", err)
				return
			}
			switch wt.newEntry.Type {
			case FILE_ENTRY_FILE_DIFF:
				diffCount += 1
			case FILE_ENTRY_FILE_NEW:
				newCount += 1
			case FILE_ENTRY_FILE_SAME:
				sameCount += 1
			}
		}

		if dc.BenchmarkPerFile {
			metric := benchmark.Metric{
				TaskName: "diff-per-file-stat",
				Labels: map[string]string{
					"diffCount": strconv.Itoa(diffCount),
					"newCount":  strconv.Itoa(newCount),
					"sameCount": strconv.Itoa(sameCount),
				},
			}
			err := dc.Benchmarker.AppendResult(metric)
			if err != nil {
				panic(err)
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
				//logger.Infof("[thread %d] diffTask %s size=%d", threadId, dt.newEntry.Name, dt.newEntry.Size)

				if dt.oldEntry == nil {
					dt.data = make([]byte, dt.newEntry.CompressedSize)
					_, err := newDimgFile.ReadAt(dt.data, dt.newEntry.Offset)
					if err != nil {
						logger.Errorf("failed to read from newDimgFile at 0x%x: %v", dt.newEntry.Offset, err)
						break
					}
				} else {
					start := time.Now()
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
						var p *bsdiffx.Plugin = nil
						switch dc.DeltaEncoding {
						case "mixed":
							p = pm.GetPluginBySize(dt.newEntry.Size)
						default:
							p = pm.GetPluginByName(dc.DeltaEncoding)
						}
						if p == nil {
							panic(fmt.Sprintf("unknown delta encoding %s", dc.DeltaEncoding))
						}
						// old File may be 0-bytes
						diffWriter := new(bytes.Buffer)
						//fmt.Printf("oldBytes=%d newBytes=%d old=%v new=%v\n", len(oldBytes), len(newBytes), *oldChildEntry, *newChildEntry)
						err = p.Diff(oldBytes, newBytes, diffWriter, dc.CompressionMode)
						if err != nil {
							logger.Errorf("failed to bsdiff.Diff: %v", err)
							break
						}
						dt.newEntry.Type = FILE_ENTRY_FILE_DIFF
						dt.newEntry.CompressedSize = int64(diffWriter.Len())
						dt.data = diffWriter.Bytes()
						dt.newEntry.PluginUuid = p.ID()
					} else {
						dt.newEntry.Type = FILE_ENTRY_FILE_NEW
						dt.data = make([]byte, dt.newEntry.CompressedSize)
						_, err := newDimgFile.ReadAt(dt.data, dt.newEntry.Offset)
						if err != nil {
							logger.Errorf("failed to read from newDimgFile at 0x%x: %v", dt.newEntry.Offset, err)
							break
						}
					}
					elapsed := time.Since(start)
					if dc.BenchmarkPerFile {
						metric := benchmark.Metric{
							TaskName:     "diff-per-file",
							ElapsedMilli: int(elapsed.Milliseconds()),
							Size:         int64(dt.newEntry.Size),
							Labels: map[string]string{
								"type":           EntryTypeToString(dt.newEntry.Type),
								"compressedSize": strconv.Itoa(int(dt.newEntry.CompressedSize)),
							},
						}
						err = dc.Benchmarker.AppendResult(metric)
						if err != nil {
							panic(err)
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

func enqueueDiffTaskToQueue(oldDimgFile, newDimgFile *DimgFile, oldEntry, newEntry *FileEntry, taskQ *diffTaskQueue) error {
	for fName := range newEntry.Childs {
		newChildEntry := newEntry.Childs[fName]
		if newChildEntry.Type == FILE_ENTRY_FILE_SAME ||
			newChildEntry.Type == FILE_ENTRY_FILE_DIFF {
			return fmt.Errorf("invalid dimg")
		}

		if newChildEntry.IsLink() ||
			newChildEntry.Size == 0 {
			continue
		}

		// newly created file or directory
		if oldEntry == nil {
			if newChildEntry.IsDir() {
				err := enqueueDiffTaskToQueue(oldDimgFile, newDimgFile, nil, newChildEntry, taskQ)
				if err != nil {
					return err
				}
			} else {
				taskQ.Enqueue(diffTask{
					oldEntry: nil,
					newEntry: newChildEntry,
				})
			}

			continue
		}

		oldChildEntry := oldEntry.Childs[fName]

		// newly created file or directory including unmatched EntryType
		if oldChildEntry == nil ||
			oldChildEntry.Name != newChildEntry.Name ||
			oldChildEntry.Type != newChildEntry.Type {
			if newChildEntry.IsDir() {
				err := enqueueDiffTaskToQueue(oldDimgFile, newDimgFile, nil, newChildEntry, taskQ)
				if err != nil {
					return err
				}
			} else {
				taskQ.Enqueue(diffTask{
					oldEntry: nil,
					newEntry: newChildEntry,
				})
			}

			continue
		}

		// if both new and old are directory, recursively generate diff
		if newChildEntry.IsDir() {
			err := enqueueDiffTaskToQueue(oldDimgFile, newDimgFile, oldChildEntry, newChildEntry, taskQ)
			if err != nil {
				return err
			}

			continue
		}

		taskQ.Enqueue(diffTask{
			oldEntry: oldChildEntry,
			newEntry: newChildEntry,
		})
	}
	return nil
}
