package image

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/klauspost/compress/zstd"
	"github.com/naoki9911/fuse-diff-containerd/pkg/bsdiffx"
	log "github.com/sirupsen/logrus"
)

type DiffBlock = struct {
	oldPos      int64
	newPos      int64
	addBytes    []byte
	insertBytes []byte
}

func NewDiffBlock(oldPos, newPos int64) DiffBlock {
	res := DiffBlock{}
	res.oldPos = oldPos
	res.newPos = newPos
	res.addBytes = make([]byte, 0)
	res.insertBytes = make([]byte, 0)
	return res
}

func readPatch(r io.Reader) ([]DiffBlock, uint64, bsdiffx.CompressionMode, error) {
	reader, newLen, compMode, err := bsdiffx.ReadPatch(r)
	if err != nil {
		return nil, 0, 0, err
	}

	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, 0, 0, err
	}

	lowerBlocks, err := readContent(newLen, bytes.NewReader(content))
	if err != nil {
		return nil, 0, 0, err
	}

	return lowerBlocks, newLen, compMode, nil
}

func writePatch(w io.Writer, size uint64, blocks []DiffBlock, mode bsdiffx.CompressionMode) error {
	writer, err := bsdiffx.WritePatch(w, size, mode)
	if err != nil {
		return err
	}
	defer writer.Close()

	for i, b := range blocks {
		ctrl0 := int64(len(b.addBytes))
		err = writeInt64(writer, ctrl0)
		if err != nil {
			return err
		}

		ctrl1 := int64(len(b.insertBytes))
		err = writeInt64(writer, ctrl1)
		if err != nil {
			return err
		}

		ctrl2 := int64(0)
		if i != len(blocks)-1 {
			ctrl2 = blocks[i+1].oldPos - blocks[i].oldPos - ctrl0
		}
		err = writeInt64(writer, ctrl2)
		if err != nil {
			return err
		}

		_, err = writer.Write(b.addBytes)
		if err != nil {
			return err
		}

		_, err = writer.Write(b.insertBytes)
		if err != nil {
			return err
		}

	}

	return nil
}

func readInt64(reader io.Reader) (int64, error) {
	buf := make([]byte, 8)
	readSize, err := reader.Read(buf)
	if err != nil {
		return 0, err
	}
	if readSize != 8 {
		return 0, fmt.Errorf("invalid size")
	}

	isNegative := (buf[7]&0x80 > 0)
	buf[7] = buf[7] & 0x7F
	res := binary.LittleEndian.Uint64(buf)
	if isNegative {
		return -int64(res), nil
	} else {
		return int64(res), nil
	}
}

func writeInt64(writer io.Writer, len int64) error {
	buf := make([]byte, 8)
	if len < 0 {
		binary.LittleEndian.PutUint64(buf, uint64(-len))
		buf[7] |= 0x80
	} else {
		binary.LittleEndian.PutUint64(buf, uint64(len))
	}

	_, err := writer.Write(buf)
	if err != nil {
		return err
	}

	return err
}

func readContent(newSize uint64, reader io.Reader) ([]DiffBlock, error) {
	newPos := int64(0)
	oldPos := int64(0)

	blocks := []DiffBlock{}
	for newPos < int64(newSize) {
		ctrl0, err := readInt64(reader)
		if err != nil {
			return nil, err
		}
		if ctrl0 < 0 {
			return nil, fmt.Errorf("ctrl0 negative")
		}
		ctrl1, err := readInt64(reader)
		if err != nil {
			return nil, err
		}
		ctrl2, err := readInt64(reader)
		if err != nil {
			return nil, err
		}

		if uint64(newPos+ctrl0) > newSize {
			return nil, fmt.Errorf("newPos + ctrl0 exceeds newSize")
		}
		//fmt.Printf("newPos=%d oldPos=%d\n", newPos, oldPos)
		//fmt.Printf("ctrl0=%d ctrl1=%d ctrl2=%d\n", ctrl0, ctrl1, ctrl2)

		diff := make([]byte, ctrl0)
		diffSize, err := reader.Read(diff)
		if err != nil {
			return nil, err
		}
		if int(ctrl0) != diffSize {
			return nil, fmt.Errorf("invalid size expected=%d actual=%d", ctrl0, diffSize)
		}

		block := DiffBlock{
			oldPos:   oldPos,
			newPos:   newPos,
			addBytes: diff,
		}

		newPos += ctrl0
		oldPos += ctrl0

		insert := make([]byte, ctrl1)
		if ctrl1 != 0 {
			insertSize, err := reader.Read(insert)
			if err != nil {
				return nil, err
			}
			if int(ctrl1) != insertSize {
				return nil, fmt.Errorf("invalid size expected=%d actual=%d", ctrl1, insertSize)
			}
		}

		block.insertBytes = insert
		blocks = append(blocks, block)

		newPos += ctrl1
		oldPos += ctrl2
	}

	return blocks, nil
}

func min(a, b int) int {
	if a < b {
		return a
	} else {
		return b
	}
}

func getBlock(newPos int64, blocks []DiffBlock) *DiffBlock {
	start_idx := int(0)
	end_idx := int(len(blocks))
	for {
		idx := int((start_idx + end_idx) / 2)
		b := blocks[idx]
		//log.Tracef("start_idx=%d end_idx=%d", start_idx, end_idx)
		//log.Tracef("b start=%d end=%d req=%d", b.newPos, b.newPos+int64(len(b.addBytes)+len(b.insertBytes)), newPos)
		if newPos >= b.newPos && newPos < b.newPos+int64(len(b.addBytes))+int64(len(b.insertBytes)) {
			return &b
		}
		if start_idx == end_idx {
			return nil
		}
		if newPos < b.newPos {
			end_idx = idx
		} else {
			start_idx = idx
		}
	}
}

func mergeBlocks(lower, upper []DiffBlock, base, updated *os.File) ([]DiffBlock, error) {
	var merged = []DiffBlock{}
	lowerLastBlock := lower[len(lower)-1]
	lowerSize := lowerLastBlock.newPos + int64(len(lowerLastBlock.addBytes)) + int64(len(lowerLastBlock.insertBytes))
	log.Tracef("lowerSize=%d", lowerSize)

	for _, upperBlock := range upper {
		upperInsertPos := upperBlock.newPos + int64(len(upperBlock.addBytes))

		cur := int64(0)
		// state = 0: new block
		// state = 1: already add
		// state = 2: already insert
		state := 0
		mergeBlock := NewDiffBlock(0, upperBlock.newPos)
		for cur < int64(len(upperBlock.addBytes))+int64(len(upperBlock.insertBytes)) {
			log.Tracef("upperOldPos=%d upperNewPos=%d", upperBlock.oldPos+cur, upperBlock.newPos+cur)
			if upperBlock.oldPos+cur >= lowerSize {
				if upperBlock.newPos+cur < upperInsertPos {
					return nil, fmt.Errorf("overlapped offset")
				}
				upperInsertBytesBegin := upperBlock.newPos + cur - upperInsertPos
				insertLen := len(upperBlock.insertBytes[upperInsertBytesBegin:])
				insertBytes := make([]byte, insertLen)
				copy(insertBytes, upperBlock.insertBytes[upperInsertBytesBegin:])

				if state == 0 {
					log.Panicf("HOGEHOGE")
				}
				state = 2
				mergeBlock.insertBytes = append(mergeBlock.insertBytes, insertBytes...)

				cur += int64(insertLen)
				continue
			}
			lowerBlock := getBlock(upperBlock.oldPos+cur, lower)
			if lowerBlock == nil {
				return nil, fmt.Errorf("invalid lower blocks")
			}
			if state == 1 {
				merged = append(merged, mergeBlock)
				mergeBlock = NewDiffBlock(0, 0)
				state = 0
			}
			log.Tracef("Got lowerBlock")
			lowerInsertPos := lowerBlock.newPos + int64(len(lowerBlock.addBytes))

			log.Tracef("lowerBlock oldPos=%d newPos=%d add=%d insert=%d\n", lowerBlock.oldPos, lowerBlock.newPos, len(lowerBlock.addBytes), len(lowerBlock.insertBytes))
			log.Tracef("upperBlock oldPos=%d newPos=%d add=%d insert=%d\n", upperBlock.oldPos, upperBlock.newPos, len(upperBlock.addBytes), len(upperBlock.insertBytes))
			for cur < int64(len(upperBlock.addBytes))+int64(len(upperBlock.insertBytes)) &&
				upperBlock.oldPos+cur < lowerBlock.newPos+int64(len(lowerBlock.addBytes))+int64(len(lowerBlock.insertBytes)) {
				if upperBlock.oldPos+cur < lowerInsertPos {
					// lower ADD
					lowerAddBytesBegin := int(upperBlock.oldPos + cur - lowerBlock.newPos)
					lowerAddRestBytesLen := len(lowerBlock.addBytes[lowerAddBytesBegin:])

					if upperBlock.newPos+cur < upperInsertPos {
						// upper ADD
						//upperAddBytesBegin := int(upperNewPos - upperBlock.newPos)
						upperAddRestBytesLen := len(upperBlock.addBytes[cur:])

						addLen := min(lowerAddRestBytesLen, upperAddRestBytesLen)
						log.Tracef("lower=ADD upper=ADD len=%d\n", addLen)
						if state == 2 {
							merged = append(merged, mergeBlock)
							mergeBlock = NewDiffBlock(0, 0)
							state = 0
						}

						if state == 0 {
							mergeBlock.oldPos = lowerBlock.oldPos + int64(lowerAddBytesBegin)
							mergeBlock.newPos = upperBlock.newPos + cur
						}

						addBytes := make([]byte, addLen)
						for i := 0; i < addLen; i++ {
							addBytes[i] = lowerBlock.addBytes[lowerAddBytesBegin+i] + upperBlock.addBytes[cur+int64(i)]
						}
						mergeBlock.addBytes = append(mergeBlock.addBytes, addBytes...)
						cur += int64(addLen)

						state = 1
					} else {
						log.Tracef("lower=ADD upper=INSERT\n")
						// upper INSERT
						upperInsertBytesBegin := cur - int64(len(upperBlock.addBytes))
						upperInsertRestBytesLen := len(upperBlock.insertBytes[upperInsertBytesBegin:])

						insertLen := min(lowerAddRestBytesLen, upperInsertRestBytesLen)
						insertBytes := make([]byte, insertLen)

						if state == 0 {
							mergeBlock.newPos = upperBlock.newPos + cur
						}

						copy(insertBytes, upperBlock.insertBytes[upperInsertBytesBegin:])
						mergeBlock.insertBytes = append(mergeBlock.insertBytes, insertBytes...)
						state = 2

						cur += int64(insertLen)
					}
				} else {
					// lower INSERT
					lowerInsertBytesBegin := int(upperBlock.oldPos + cur - lowerInsertPos)
					lowerInsertRestBytesLen := len(lowerBlock.insertBytes[lowerInsertBytesBegin:])

					if state == 0 {
						mergeBlock.newPos = upperBlock.newPos + cur
					}
					if upperBlock.newPos+cur < upperInsertPos {
						log.Tracef("lower=INSERT upper=ADD\n")
						// upper ADD
						upperAddRestBytesLen := len(upperBlock.addBytes[cur:])
						insertLen := min(lowerInsertRestBytesLen, upperAddRestBytesLen)

						insertBytes := make([]byte, insertLen)
						for i := 0; i < insertLen; i++ {
							insertBytes[i] = lowerBlock.insertBytes[lowerInsertBytesBegin+i] + upperBlock.addBytes[cur+int64(i)]
						}
						mergeBlock.insertBytes = append(mergeBlock.insertBytes, insertBytes...)
						state = 2

						cur += int64(insertLen)
					} else {
						// upper INSERT
						upperInsertBytesBegin := cur - int64(len(upperBlock.addBytes))
						upperInsertRestBytesLen := len(upperBlock.insertBytes[upperInsertBytesBegin:])

						insertLen := min(lowerInsertRestBytesLen, upperInsertRestBytesLen)
						insertBytes := make([]byte, insertLen)

						log.Tracef("lower=INSERT upper=INSERT Len=%d\n", insertLen)
						copy(insertBytes, upperBlock.insertBytes[upperInsertBytesBegin:])
						mergeBlock.insertBytes = append(mergeBlock.insertBytes, insertBytes...)
						state = 2

						cur += int64(insertLen)
					}
				}
			}
			if base != nil && updated != nil {
				baseAddBytes := make([]byte, len(mergeBlock.addBytes))
				patchedBytes := make([]byte, len(mergeBlock.addBytes)+len(mergeBlock.insertBytes))
				updatedBytes := make([]byte, len(mergeBlock.addBytes)+len(mergeBlock.insertBytes))
				_, err := base.ReadAt(baseAddBytes, mergeBlock.oldPos)
				if err != nil {
					return nil, err
				}
				_, err = updated.ReadAt(updatedBytes, mergeBlock.newPos)
				if err != nil {
					return nil, err
				}

				for i := 0; i < len(baseAddBytes); i++ {
					patchedBytes[i] = baseAddBytes[i] + mergeBlock.addBytes[i]
				}
				for i := 0; i < len(mergeBlock.insertBytes); i++ {
					patchedBytes[i+len(mergeBlock.addBytes)] = mergeBlock.insertBytes[i]
				}
				if !bytes.Equal(updatedBytes, patchedBytes) {
					fmt.Printf("ans=%v\n", updatedBytes)
					fmt.Printf("patched=%v\n", patchedBytes)
					log.Panicf("Coruppted! oldPos=%d newPos=%d len=%d", mergeBlock.oldPos, mergeBlock.newPos, len(updatedBytes))
				} else {
					fmt.Printf("VERIFY OK\n")
				}
			}

			//if !merging {
			//	merged = append(merged, mergeBlock)
			//} else {
			//	merged[len(merged)-1] = mergeBlock
			//}
		}
		if state == 1 || state == 2 {
			merged = append(merged, mergeBlock)
		}
		//fmt.Println(merged)
	}

	return merged, nil
}

func deltaMergingBytes(lowerDiff, upperDiff io.Reader, mergedDiff io.Writer) error {
	lowerBlocks, _, _, err := readPatch(lowerDiff)
	if err != nil {
		return err
	}
	upperBlocks, newLen, compMode, err := readPatch(upperDiff)
	if err != nil {
		return err
	}

	mergedBlocks, err := mergeBlocks(lowerBlocks, upperBlocks, nil, nil)
	if err != nil {
		return err
	}

	err = writePatch(mergedDiff, newLen, mergedBlocks, compMode)
	if err != nil {
		return err
	}

	return nil
}

var ErrUnexpected = fmt.Errorf("unexpected error")

type mergeTask struct {
	lowerEntry *FileEntry
	upperEntry *FileEntry
	data       []byte
}

func mergeDiffDimgMultihread(lowerImgFile, upperImgFile *DimgFile, mergeOut *bytes.Buffer, threadNum int) (*FileEntry, error) {
	lowerEntry := &lowerImgFile.Header().FileEntry
	upperEntry := &upperImgFile.Header().FileEntry

	mergeTasks := make(chan mergeTask, 1000)
	writeTasks := make(chan mergeTask, 1000)
	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()

		logger.Info("started merge task enqueue thread")
		err := enqueueMergeTaskToQueue(lowerEntry, upperEntry, mergeTasks)
		if err != nil {
			logger.Errorf("failed to enqueue: %v", err)
		}
		close(mergeTasks)
		logger.Info("finished mege task enqueue thread")
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Info("started merge write thread")
		for {
			mt, more := <-writeTasks
			if !more {
				break
			}
			mt.upperEntry.Offset = int64(mergeOut.Len())
			_, err := mergeOut.Write(mt.data)
			if err != nil {
				logger.Errorf("failed to write to mergeOut: %v", err)
				return
			}
		}
		logger.Info("finished merge write thread")
	}()

	mergeWg := sync.WaitGroup{}
	for i := 0; i < threadNum; i++ {
		wg.Add(1)
		mergeWg.Add(1)
		go func(threadId int) {
			logger.Infof("started merge thread idx=%d", threadId)
			defer wg.Done()
			defer mergeWg.Done()
			for {
				mt, more := <-mergeTasks
				if !more {
					break
				}
				if mt.lowerEntry != nil && mt.upperEntry != nil {
					if mt.lowerEntry.Type == FILE_ENTRY_FILE_NEW && mt.upperEntry.Type == FILE_ENTRY_FILE_DIFF {
						lowerBytes := make([]byte, mt.lowerEntry.CompressedSize)
						upperBytes := make([]byte, mt.upperEntry.CompressedSize)
						_, err := lowerImgFile.ReadAt(lowerBytes, mt.lowerEntry.Offset)
						if err != nil {
							logger.Errorf("failed to read from lowerImg: %v", err)
							return
						}
						baseReader, err := zstd.NewReader(bytes.NewBuffer(lowerBytes))
						if err != nil {
							logger.Errorf("failed to read lowerImg via zstdReader: %v", err)
							return
						}
						defer baseReader.Close()

						_, err = upperImgFile.ReadAt(upperBytes, mt.upperEntry.Offset)
						if err != nil {
							logger.Errorf("failed to read from upperImg: %v", err)
							return
						}
						mergeBytes := bytes.NewBuffer(nil)
						err = bsdiffx.Patch(baseReader, mergeBytes, bytes.NewBuffer(upperBytes))
						if err != nil {
							logger.Errorf("failed to patch: %v", err)
							return
						}

						mergeCompressed, err := CompressWithZstd(mergeBytes.Bytes())
						if err != nil {
							logger.Errorf("failed to compresse merged bytes: %v", err)
							return
						}
						mt.upperEntry.Type = FILE_ENTRY_FILE_NEW
						mt.upperEntry.CompressedSize = int64(len(mergeCompressed))
						mt.data = mergeCompressed
					} else if mt.lowerEntry.Type == FILE_ENTRY_FILE_DIFF && mt.upperEntry.Type == FILE_ENTRY_FILE_DIFF {
						lowerBytes := make([]byte, mt.lowerEntry.CompressedSize)
						upperBytes := make([]byte, mt.upperEntry.CompressedSize)
						_, err := lowerImgFile.ReadAt(lowerBytes, mt.lowerEntry.Offset)
						if err != nil {
							logger.Errorf("failed to read from lowerImg: %v", err)
							return
						}
						_, err = upperImgFile.ReadAt(upperBytes, mt.upperEntry.Offset)
						if err != nil {
							logger.Errorf("failed to read from upperImg: %v", err)
							return
						}
						mergeBytes := bytes.NewBuffer(nil)
						err = deltaMergingBytes(bytes.NewBuffer(lowerBytes), bytes.NewBuffer(upperBytes), mergeBytes)
						if err != nil {
							logger.Errorf("failed to merge diffs: %v", err)
							return
						}
						mt.upperEntry.CompressedSize = int64(mergeBytes.Len())
						mt.data = mergeBytes.Bytes()
					} else {
						logger.Errorf("unexpected types lower=%v upper=%v", mt.lowerEntry.Type, mt.upperEntry.Type)
						return
					}
				} else if mt.lowerEntry != nil {
					lowerBytes := make([]byte, mt.lowerEntry.CompressedSize)
					_, err := lowerImgFile.ReadAt(lowerBytes, mt.lowerEntry.Offset)
					if err != nil {
						logger.Errorf("failed to read from lowerImg: %v", err)
						return
					}
					mt.upperEntry = mt.lowerEntry
					mt.data = lowerBytes
				} else if mt.upperEntry != nil {
					upperBytes := make([]byte, mt.upperEntry.CompressedSize)
					_, err := upperImgFile.ReadAt(upperBytes, mt.upperEntry.Offset)
					if err != nil {
						logger.Errorf("failed to read from upperImg: %v", err)
						return
					}
					mt.data = upperBytes
				}
				writeTasks <- mt
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

	logger.Infof("mergeTasks len=%d", len(mergeTasks))
	return upperEntry, nil
}

// upperEntry is updated to merged FileEntry
func enqueueMergeTaskToQueue(lowerEntry, upperEntry *FileEntry, taskChan chan mergeTask) error {
	for upperfName := range upperEntry.Childs {
		upperChild := upperEntry.Childs[upperfName]
		switch upperChild.Type {
		case FILE_ENTRY_DIR_NEW, FILE_ENTRY_FILE_NEW, FILE_ENTRY_SYMLINK, FILE_ENTRY_OPAQUE:
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
				return ErrUnexpected
			}

			// When the lower has SYMLINK or OPAQUE, the upper must have 'New' entries
			// Such files must be processed above case.
			if lowerChild.Type == FILE_ENTRY_SYMLINK || lowerChild.Type == FILE_ENTRY_OPAQUE {
				return ErrUnexpected
			}

			switch upperChild.Type {
			case FILE_ENTRY_DIR:
				if lowerChild.IsDir() {
					err := enqueueMergeTaskToQueue(lowerChild, upperChild, taskChan)
					if err != nil {
						return err
					}
				} else {
					return ErrUnexpected
				}
			case FILE_ENTRY_FILE_SAME:
				// lower must have FILE_NEW or FILE_DIFF
				if lowerChild.HasBody() {
					upperEntry.Childs[upperfName] = lowerChild
					taskChan <- mergeTask{
						lowerEntry: lowerChild,
						upperEntry: nil,
					}
				} else if lowerChild.Type == FILE_ENTRY_FILE_SAME {
					// this branch is ignored
				} else {
					return ErrUnexpected
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
					return ErrUnexpected
				}
			}
		}
	}

	return nil
}

func MergeDimg(lowerDimg, upperDimg string, merged io.Writer, threadNum int) (*DimgHeader, error) {
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
	mergedEntry, err := mergeDiffDimgMultihread(lowerImgFile, upperImgFile, &tmp, threadNum)
	if err != nil {
		panic(err)
	}

	header := DimgHeader{
		Id:        upperImgFile.Header().Id,
		ParentId:  lowerImgFile.Header().ParentId,
		FileEntry: *mergedEntry,
	}

	err = WriteDimg(merged, &header, &tmp)
	if err != nil {
		return nil, fmt.Errorf("failed to write to dimg: %v", err)
	}
	return &header, nil
}

func MergeCdimg(lowerCdimg, upperCdimg string, merged io.Writer, threadNum int) error {
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
	mergedEntry, err := mergeDiffDimgMultihread(lowerDimg, upperDimg, &tmp, threadNum)
	if err != nil {
		panic(err)
	}

	header := DimgHeader{
		Id:        upperDimg.Header().Id,
		ParentId:  lowerDimg.Header().ParentId,
		FileEntry: *mergedEntry,
	}

	mergedDimg := bytes.Buffer{}
	err = WriteDimg(&mergedDimg, &header, &tmp)
	if err != nil {
		return fmt.Errorf("failed to write to dimg: %v", err)
	}

	err = WriteCdimgHeader(bytes.NewBuffer(upperCdimgFile.Header.ConfigBytes), &header, int64(mergedDimg.Len()), merged)
	if err != nil {
		return fmt.Errorf("failed to cdimg header: %v", err)
	}
	_, err = io.Copy(merged, &mergedDimg)
	if err != nil {
		return fmt.Errorf("failed to write dimg: %v", err)
	}
	return nil
}
