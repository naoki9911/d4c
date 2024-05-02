package image

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/naoki9911/fuse-diff-containerd/pkg/benchmark"
	"github.com/naoki9911/fuse-diff-containerd/pkg/bsdiffx"
	"github.com/naoki9911/fuse-diff-containerd/pkg/utils"
	log "github.com/sirupsen/logrus"
)

var ErrUnexpected = fmt.Errorf("unexpected error")

type mergeTask struct {
	lowerEntry *FileEntry
	upperEntry *FileEntry
	data       []byte
}

func mergeDiffDimgMultihread(lowerImgFile, upperImgFile *DimgFile, mergeOut *bytes.Buffer, mc MergeConfig, pm *bsdiffx.PluginManager) (*FileEntry, error) {
	lowerEntry := &lowerImgFile.DimgHeader().FileEntry
	upperEntry := &upperImgFile.DimgHeader().FileEntry

	mergeTasks := make(chan mergeTask, 1000)
	writeTasks := make(chan mergeTask, 1000)
	wg := sync.WaitGroup{}

	var gErr error
	ctx, cancel := context.WithCancel(context.Background())

	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Info("started merge task enqueue thread")
		err := enqueueMergeTaskToQueue(lowerEntry, upperEntry, mergeTasks)
		if err != nil {
			gErr = fmt.Errorf("failed to enqueue: %v", err)
			cancel()
			logger.Errorf("merge task enqueu thread: %v", gErr)
		}
		close(mergeTasks)
		logger.Info("finished mege task enqueue thread")
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Info("started merge write thread")
		cont := true
		for cont {
			select {
			case <-ctx.Done():
				logger.Infof("merge write thread canceled")
				cont = false
			case mt, more := <-writeTasks:
				if !more {
					cont = false
					break
				}
				mt.upperEntry.Offset = int64(mergeOut.Len())
				_, err := mergeOut.Write(mt.data)
				if err != nil {
					gErr = fmt.Errorf("failed to write to mergeOut: %v", err)
					cancel()
					logger.Errorf("merge write thread: %v", gErr)
					return
				}
			}
		}
		logger.Info("finished merge write thread")
	}()

	mergeWg := sync.WaitGroup{}
	for i := 0; i < mc.ThreadNum; i++ {
		wg.Add(1)
		mergeWg.Add(1)
		go func(threadId int) {
			logger.Infof("started merge thread idx=%d", threadId)
			defer wg.Done()
			defer mergeWg.Done()
			cont := true
			for cont {
				select {
				case <-ctx.Done():
					logger.Infof("merge thread canceled")
					cont = false
				case mt, more := <-mergeTasks:
					if !more {
						cont = false
						break
					}
					start := time.Now()
					mode := ""
					if mt.lowerEntry != nil && mt.upperEntry != nil {
						p := pm.GetPluginByUuid(mt.upperEntry.PluginUuid)
						if mt.lowerEntry.Type == FILE_ENTRY_FILE_NEW && mt.upperEntry.Type == FILE_ENTRY_FILE_DIFF {
							lowerBytes := make([]byte, mt.lowerEntry.CompressedSize)
							upperBytes := make([]byte, mt.upperEntry.CompressedSize)
							_, err := lowerImgFile.ReadAt(lowerBytes, mt.lowerEntry.Offset)
							if err != nil {
								gErr = fmt.Errorf("failed to read from lowerImg: %v", err)
								cancel()
								logger.Errorf("merge thread: %v", gErr)
								return
							}
							baseBytes, err := utils.DecompressWithZstd(lowerBytes)
							if err != nil {
								gErr = fmt.Errorf("failed to decompress lowerImg: %v", err)
								cancel()
								logger.Errorf("merge thread: %v", gErr)
								return
							}

							_, err = upperImgFile.ReadAt(upperBytes, mt.upperEntry.Offset)
							if err != nil {
								gErr = fmt.Errorf("failed to read from upperImg: %v", err)
								cancel()
								logger.Errorf("merge thread: %v", gErr)
								return
							}
							mergeBytes, err := p.Patch(baseBytes, bytes.NewBuffer(upperBytes))
							if err != nil {
								gErr = fmt.Errorf("failed to patch: %v", err)
								cancel()
								logger.Errorf("merge thread: %v", gErr)
								return
							}

							mergeCompressed, err := CompressWithZstd(mergeBytes)
							if err != nil {
								gErr = fmt.Errorf("failed to compresse merged bytes: %v", err)
								cancel()
								logger.Errorf("merge thread: %v", gErr)
								return
							}
							mt.upperEntry.Type = FILE_ENTRY_FILE_NEW
							mt.upperEntry.CompressedSize = int64(len(mergeCompressed))
							mt.data = mergeCompressed
							mode = "apply"
						} else if mt.lowerEntry.Type == FILE_ENTRY_FILE_DIFF && mt.upperEntry.Type == FILE_ENTRY_FILE_DIFF {
							if mt.lowerEntry.PluginUuid != mt.upperEntry.PluginUuid {
								panic(fmt.Sprintf("unmatched plugin lower %s upper %s", mt.lowerEntry.PluginUuid, mt.upperEntry.PluginUuid))
							}
							lowerBytes := make([]byte, mt.lowerEntry.CompressedSize)
							upperBytes := make([]byte, mt.upperEntry.CompressedSize)
							_, err := lowerImgFile.ReadAt(lowerBytes, mt.lowerEntry.Offset)
							if err != nil {
								gErr = fmt.Errorf("failed to read from lowerImg: %v", err)
								cancel()
								logger.Errorf("merge thread: %v", gErr)
								return
							}
							_, err = upperImgFile.ReadAt(upperBytes, mt.upperEntry.Offset)
							if err != nil {
								gErr = fmt.Errorf("failed to read from upperImg: %v", err)
								cancel()
								logger.Errorf("merge thread: %v", gErr)
								return
							}
							mergeBytes := bytes.NewBuffer(nil)
							err = p.Merge(bytes.NewBuffer(lowerBytes), bytes.NewBuffer(upperBytes), mergeBytes)
							if err != nil {
								gErr = fmt.Errorf("failed to merge diffs: %v", err)
								cancel()
								logger.Errorf("merge thread: %v", gErr)
								return
							}
							mt.upperEntry.CompressedSize = int64(mergeBytes.Len())
							mt.data = mergeBytes.Bytes()
							mode = "merge"
						} else {
							gErr = fmt.Errorf("unexpected types lower=%v upper=%v", mt.lowerEntry.Type, mt.upperEntry.Type)
							cancel()
							logger.Errorf("merge thread: %v", gErr)
							return
						}
					} else if mt.lowerEntry != nil {
						lowerBytes := make([]byte, mt.lowerEntry.CompressedSize)
						_, err := lowerImgFile.ReadAt(lowerBytes, mt.lowerEntry.Offset)
						if err != nil {
							gErr = fmt.Errorf("failed to read from lowerImg: %v", err)
							cancel()
							logger.Errorf("merge thread: %v", gErr)
							return
						}
						mt.upperEntry = mt.lowerEntry
						mt.data = lowerBytes
						mode = "copy-lower"
					} else if mt.upperEntry != nil {
						upperBytes := make([]byte, mt.upperEntry.CompressedSize)
						_, err := upperImgFile.ReadAt(upperBytes, mt.upperEntry.Offset)
						if err != nil {
							gErr = fmt.Errorf("failed to read from upperImg: %v", err)
							cancel()
							gErr = fmt.Errorf("merge thread: %v", gErr)
							return
						}
						mt.data = upperBytes
						mode = "copy-upper"
					}
					elapsed := time.Since(start)
					if mc.BenchmarkPerFile {
						metric := benchmark.Metric{
							TaskName:     "merge-per-file",
							ElapsedMilli: int(elapsed.Milliseconds()),
							Size:         int64(mt.upperEntry.Size),
							Labels: map[string]string{
								"mergeMode":      mode,
								"compressedSize": strconv.Itoa(int(mt.upperEntry.CompressedSize)),
							},
						}
						err := mc.Benchmarker.AppendResult(metric)
						if err != nil {
							panic(err)
						}
					}
					writeTasks <- mt
				}
			}
			logger.Infof("finished merge thread idx=%d", threadId)
		}(i)
	}

	go func() {
		mergeWg.Wait()
		close(writeTasks)
		logger.Info("all merge tasks finished")
	}()
	wg.Wait()

	logger.Info("started to update dir entry")
	updateDirFileEntry(upperEntry)
	logger.Info("finished to update dir entry")

	if gErr != nil {
		return nil, gErr
	}
	return upperEntry, nil
}

// upperEntry is updated to merged FileEntry
func enqueueMergeTaskToQueue(lowerEntry, upperEntry *FileEntry, taskChan chan mergeTask) error {
	for upperfName := range upperEntry.Childs {
		upperChild := upperEntry.Childs[upperfName]
		switch upperChild.Type {
		case FILE_ENTRY_DIR_NEW, FILE_ENTRY_FILE_NEW, FILE_ENTRY_SYMLINK, FILE_ENTRY_HARDLINK:
			log.Debugf("upperChild is new")
			if upperChild.IsDir() {
				err := enqueueMergeTaskToQueue(nil, upperChild, taskChan)
				if err != nil {
					return err
				}
			} else if upperChild.HasBody() {
				taskChan <- mergeTask{
					lowerEntry: nil,
					upperEntry: upperChild,
				}
			}
		default:
			lowerChild, ok := lowerEntry.Childs[upperfName]
			if !ok {
				for k, v := range upperChild.Childs {
					fmt.Printf("- %s %v\n", k, EntryTypeToString(v.Type))
				}
				return fmt.Errorf("upperChild is %s but lowerChild(%s) not found: %v", EntryTypeToString(upperChild.Type), upperfName, upperChild.Childs)
			}

			// When the lower has SYMLINK, the upper must have 'New' entries
			// Such files must be processed above case.
			if lowerChild.IsLink() {
				return fmt.Errorf("lowerChild is symlink or hardlink")
			}

			switch upperChild.Type {
			case FILE_ENTRY_DIR:
				if lowerChild.IsDir() {
					err := enqueueMergeTaskToQueue(lowerChild, upperChild, taskChan)
					if err != nil {
						return err
					}
				} else {
					return fmt.Errorf("lowerChild is not directory")
				}
			case FILE_ENTRY_FILE_SAME:
				// lower must have FILE_NEW or FILE_DIFF
				if lowerChild.HasBody() {
					// upperChild's metadata can be updated
					lowerChild.Mode = upperChild.Mode
					lowerChild.UID = upperChild.UID
					lowerChild.GID = upperChild.GID
					lowerChild.Digest = upperChild.Digest
					upperEntry.Childs[upperfName] = lowerChild
					taskChan <- mergeTask{
						lowerEntry: lowerChild,
						upperEntry: nil,
					}
				} else if lowerChild.Type == FILE_ENTRY_FILE_SAME {
					// this branch is ignored
				} else {
					return fmt.Errorf("upperChild is FILE_SAME but lowerChild does not have body")
				}
			case FILE_ENTRY_FILE_DIFF:
				if lowerChild.Type == FILE_ENTRY_FILE_SAME {
					taskChan <- mergeTask{
						lowerEntry: nil,
						upperEntry: upperChild,
					}
				} else if lowerChild.HasBody() {
					taskChan <- mergeTask{
						lowerEntry: lowerChild,
						upperEntry: upperChild,
					}
				} else {
					return fmt.Errorf("upperChild is FILE_DIFF but lowerChild does not have body")
				}
			}
		}
	}

	return nil
}

type MergeConfig struct {
	ThreadNum              int
	MergeDimgConcurrentNum int
	BenchmarkPerFile       bool
	Benchmarker            *benchmark.Benchmark
}

func MergeDimg(lowerDimg, upperDimg string, merged io.Writer, mc MergeConfig, pm *bsdiffx.PluginManager) (*DimgHeader, error) {
	lowerImgFile, err := OpenDimgFile(lowerDimg)
	if err != nil {
		panic(err)
	}
	defer lowerImgFile.Close()
	upperImgFile, err := OpenDimgFile(upperDimg)
	if err != nil {
		panic(err)
	}
	defer upperImgFile.Close()
	tmp := bytes.Buffer{}
	mergedEntry, err := mergeDiffDimgMultihread(lowerImgFile, upperImgFile, &tmp, mc, pm)
	if err != nil {
		panic(err)
	}

	header := DimgHeader{
		Id:              upperImgFile.DimgHeader().Id,
		ParentId:        lowerImgFile.DimgHeader().ParentId,
		CompressionMode: lowerImgFile.DimgHeader().CompressionMode,
		FileEntry:       *mergedEntry,
	}

	err = WriteDimg(merged, &header, &tmp)
	if err != nil {
		return nil, fmt.Errorf("failed to write to dimg: %v", err)
	}
	return &header, nil
}

func MergeCdimg(lowerCdimg, upperCdimg string, merged io.Writer, mc MergeConfig, pm *bsdiffx.PluginManager) (*DimgHeader, error) {
	lowerCdimgFile, err := OpenCdimgFile(lowerCdimg)
	if err != nil {
		panic(err)
	}
	defer lowerCdimgFile.Close()
	lowerDimg := lowerCdimgFile.Dimg

	upperCdimgFile, err := OpenCdimgFile(upperCdimg)
	if err != nil {
		panic(err)
	}
	defer upperCdimgFile.Close()
	upperDimg := upperCdimgFile.Dimg

	tmp := bytes.Buffer{}
	mergedEntry, err := mergeDiffDimgMultihread(lowerDimg, upperDimg, &tmp, mc, pm)
	if err != nil {
		panic(err)
	}

	header := DimgHeader{
		Id:              upperDimg.DimgHeader().Id,
		ParentId:        lowerDimg.DimgHeader().ParentId,
		CompressionMode: upperDimg.DimgHeader().CompressionMode,
		FileEntry:       *mergedEntry,
	}

	mergedDimg := bytes.Buffer{}
	err = WriteDimg(&mergedDimg, &header, &tmp)
	if err != nil {
		return nil, fmt.Errorf("failed to write to dimg: %v", err)
	}

	err = WriteCdimgHeader(bytes.NewBuffer(upperCdimgFile.Header.ConfigBytes), &header, int64(mergedDimg.Len()), merged)
	if err != nil {
		return nil, fmt.Errorf("failed to cdimg header: %v", err)
	}
	_, err = io.Copy(merged, &mergedDimg)
	if err != nil {
		return nil, fmt.Errorf("failed to write dimg: %v", err)
	}
	return &header, nil
}

func MergeDimgsWithLinear(dimgs []*DimgEntry, tmpDir string, mc MergeConfig, isCdimg bool, pm *bsdiffx.PluginManager) (*DimgEntry, error) {
	lowerDimg := dimgs[len(dimgs)-1]
	for idx := len(dimgs) - 2; idx >= 0; idx-- {
		upperDimg := dimgs[idx]
		var mergedDimgPath string
		if isCdimg {
			mergedDimgPath = filepath.Join(tmpDir, utils.GetRandomId("merge")+".cdimg")
		} else {
			mergedDimgPath = filepath.Join(tmpDir, utils.GetRandomId("merge")+".dimg")
		}
		mergedFile, err := os.Create(mergedDimgPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create temporary dimg %s: %v", mergedDimgPath, err)
		}
		defer mergedFile.Close()

		logger.Infof("merge %s and %s", lowerDimg.Digest(), upperDimg.Digest())
		var header *DimgHeader
		if isCdimg {
			header, err = MergeCdimg(lowerDimg.Path, upperDimg.Path, mergedFile, mc, pm)
		} else {
			header, err = MergeDimg(lowerDimg.Path, upperDimg.Path, mergedFile, mc, pm)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to merge dimgs: %v", err)
		}
		lowerDimg.DimgHeader = *header
		lowerDimg.Path = mergedDimgPath
	}

	return lowerDimg, nil
}

type mergeDimgTask struct {
	lowerMergeTask *mergeDimgTask
	upperMergeTask *mergeDimgTask

	dimg *DimgEntry

	done chan error
}

func MergeDimgsWithBisectMultithread(dimgs []*DimgEntry, tmpDir string, mc MergeConfig, isCdimg bool, pm *bsdiffx.PluginManager) (*DimgEntry, error) {
	mergeTask := buildMergeDimgTask(dimgs)
	if mergeTask == nil {
		return nil, fmt.Errorf("mergeTasks is nil")
	}

	threadLimit := make(chan struct{}, mc.MergeDimgConcurrentNum)
	err := runMergeDimgTask(mergeTask, tmpDir, threadLimit, mc, isCdimg, pm)
	if err != nil {
		return nil, err
	}

	if mergeErr := <-mergeTask.done; mergeErr != nil {
		return nil, fmt.Errorf("failed to merge: %v", mergeErr)
	}

	return mergeTask.dimg, nil
}

func buildMergeDimgTask(dimgs []*DimgEntry) *mergeDimgTask {
	dimgsLen := len(dimgs)
	switch dimgsLen {
	case 0:
		return nil
	case 1:
		return &mergeDimgTask{
			dimg: dimgs[0],
			done: make(chan error, 1),
		}
	default:
		return &mergeDimgTask{
			upperMergeTask: buildMergeDimgTask(dimgs[0 : dimgsLen/2]),
			lowerMergeTask: buildMergeDimgTask(dimgs[dimgsLen/2:]),
			done:           make(chan error, 1),
		}
	}
}

func runMergeDimgTask(task *mergeDimgTask, tmpDir string, threadLimit chan struct{}, mc MergeConfig, isCdimg bool, pm *bsdiffx.PluginManager) error {
	if task.upperMergeTask == nil && task.lowerMergeTask == nil {
		task.done <- nil
		return nil
	}

	if task.lowerMergeTask != nil {
		err := runMergeDimgTask(task.lowerMergeTask, tmpDir, threadLimit, mc, isCdimg, pm)
		if err != nil {
			return err
		}
	}

	if task.upperMergeTask != nil {
		err := runMergeDimgTask(task.upperMergeTask, tmpDir, threadLimit, mc, isCdimg, pm)
		if err != nil {
			return err
		}
	}

	if lowerErr := <-task.lowerMergeTask.done; lowerErr != nil {
		return fmt.Errorf("lowerMergeTask has error: %v", lowerErr)
	}
	if upperErr := <-task.upperMergeTask.done; upperErr != nil {
		return fmt.Errorf("upperMergeTask has error: %v", upperErr)
	}

	threadLimit <- struct{}{}
	go func() {
		upperDimg := task.upperMergeTask.dimg
		lowerDimg := task.lowerMergeTask.dimg

		var mergedDimgPath string
		if isCdimg {
			mergedDimgPath = filepath.Join(tmpDir, utils.GetRandomId("merge")+".cdimg")
		} else {
			mergedDimgPath = filepath.Join(tmpDir, utils.GetRandomId("merge")+".dimg")
		}
		mergedFile, err := os.Create(mergedDimgPath)
		if err != nil {
			task.done <- fmt.Errorf("failed to create temporary dimg %s: %v", mergedDimgPath, err)
			return
		}
		defer mergedFile.Close()

		logger.Infof("merge %s and %s", lowerDimg.Digest(), upperDimg.Digest())
		var header *DimgHeader
		if isCdimg {
			header, err = MergeCdimg(lowerDimg.Path, upperDimg.Path, mergedFile, mc, pm)
		} else {
			header, err = MergeDimg(lowerDimg.Path, upperDimg.Path, mergedFile, mc, pm)
		}
		if err != nil {
			task.done <- fmt.Errorf("failed to merge: %v", err)
			return
		}

		task.dimg = upperDimg
		task.dimg.DimgHeader = *header
		task.dimg.Path = mergedDimgPath
		task.done <- nil

		<-threadLimit
	}()

	return nil
}
