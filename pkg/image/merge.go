package image

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path"

	"github.com/icedream/go-bsdiff"
	"github.com/klauspost/compress/zstd"
	log "github.com/sirupsen/logrus"

	"github.com/dsnet/compress/bzip2"

	"github.com/pkg/errors"
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

var ErrInvalidMagic = errors.New("Invalid magic")
var sizeEncoding = binary.BigEndian
var magicText = []byte("ENDSLEY/BSDIFF43")

func readHeader(r io.Reader) (size uint64, err error) {
	magicBuf := make([]byte, len(magicText))
	n, err := r.Read(magicBuf)
	if err != nil {
		return
	}
	if n < len(magicText) {
		err = ErrInvalidMagic
		return
	}

	err = binary.Read(r, sizeEncoding, &size)

	return
}

func writeHeader(w io.Writer, size uint64) error {
	_, err := w.Write(magicText)
	if err != nil {
		return err
	}

	err = binary.Write(w, sizeEncoding, size)
	if err != nil {
		return err
	}

	return err
}

func readPatch(reader io.Reader) ([]DiffBlock, uint64, error) {
	newLen, err := readHeader(reader)
	if err != nil {
		return nil, 0, err
	}

	// Decompression
	bz2Reader, err := bzip2.NewReader(reader, nil)
	if err != nil {
		return nil, 0, err
	}
	defer bz2Reader.Close()

	content, err := io.ReadAll(bz2Reader)
	if err != nil {
		return nil, 0, err
	}

	lowerBlocks, err := readContent(newLen, bytes.NewReader(content))
	if err != nil {
		return nil, 0, err
	}

	return lowerBlocks, newLen, nil
}

func writePatch(w io.Writer, size uint64, blocks []DiffBlock) error {
	err := writeHeader(w, size)
	if err != nil {
		return err
	}

	bz2Writer, err := bzip2.NewWriter(w, nil)
	if err != nil {
		return err
	}
	defer bz2Writer.Close()

	for i, b := range blocks {
		ctrl0 := int64(len(b.addBytes))
		err = writeInt64(bz2Writer, ctrl0)
		if err != nil {
			return err
		}

		ctrl1 := int64(len(b.insertBytes))
		err = writeInt64(bz2Writer, ctrl1)
		if err != nil {
			return err
		}

		ctrl2 := int64(0)
		if i != len(blocks)-1 {
			ctrl2 = blocks[i+1].oldPos - blocks[i].oldPos - ctrl0
		}
		err = writeInt64(bz2Writer, ctrl2)
		if err != nil {
			return err
		}

		_, err = bz2Writer.Write(b.addBytes)
		if err != nil {
			return err
		}

		_, err = bz2Writer.Write(b.insertBytes)
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

func DeltaMerging(lowerDiff, upperDiff, mergedDiff, lowerFile, upperFile string) error {
	//fmt.Println(upperDiff)
	lowerPatch, err := os.Open(lowerDiff)
	if err != nil {
		return err
	}
	defer lowerPatch.Close()

	upperPatch, err := os.Open(upperDiff)
	if err != nil {
		return err
	}
	defer upperPatch.Close()

	mergedPatch, err := os.Create(mergedDiff)
	if err != nil {
		return err
	}
	defer mergedPatch.Close()

	lowerBlocks, _, err := readPatch(lowerPatch)
	if err != nil {
		return err
	}
	upperBlocks, newLen, err := readPatch(upperPatch)
	if err != nil {
		return err
	}

	var lowerF *os.File = nil
	var upperF *os.File = nil
	if lowerFile != "" {
		lowerF, err = os.Open(lowerFile)
		if err != nil {
			return err
		}
	}
	if upperFile != "" {
		upperF, err = os.Open(upperFile)
		if err != nil {
			return err
		}
	}

	mergedBlocks, err := mergeBlocks(lowerBlocks, upperBlocks, lowerF, upperF)
	if err != nil {
		return err
	}

	err = writePatch(mergedPatch, newLen, mergedBlocks)
	if err != nil {
		return err
	}

	return nil
}

func DeltaMergingBytes(lowerDiff, upperDiff io.Reader, mergedDiff io.Writer) error {
	lowerBlocks, _, err := readPatch(lowerDiff)
	if err != nil {
		return err
	}
	upperBlocks, newLen, err := readPatch(upperDiff)
	if err != nil {
		return err
	}

	mergedBlocks, err := mergeBlocks(lowerBlocks, upperBlocks, nil, nil)
	if err != nil {
		return err
	}

	err = writePatch(mergedDiff, newLen, mergedBlocks)
	if err != nil {
		return err
	}

	return nil
}

var ErrUnexpected = fmt.Errorf("unexpected error")

func copyDimg(entry *FileEntry, upperPath string, upperImgFile *DimgFile, mergeOut *bytes.Buffer) (*FileEntry, error) {
	mergeEntry := entry.DeepCopy()
	if entry.IsDir() {
		for fName := range entry.Childs {
			e := entry.Childs[fName]
			mergeChild, err := copyDimg(e, path.Join(upperPath, e.Name), upperImgFile, mergeOut)
			if err != nil {
				return nil, err
			}
			mergeEntry.Childs[fName] = mergeChild
		}
	} else {
		log.Debugf("Copy %s from upper offset=0x%x type=%d", upperPath, entry.Offset, entry.Type)
		upperBytes := make([]byte, entry.CompressedSize)
		_, err := upperImgFile.ReadAt(upperBytes, entry.Offset)
		if err != nil {
			return nil, err
		}
		mergeEntry.Offset = int64(len(mergeOut.Bytes()))
		_, err = mergeOut.Write(upperBytes)
		if err != nil {
			return nil, err
		}
	}
	return mergeEntry, nil

}

func MergeDiffDimg(lowerEntry, upperEntry *FileEntry, lowerDiff, upperDiff string, lowerImgFile, upperImgFile *DimgFile, mergeEntry *FileEntry, mergeOut *bytes.Buffer) error {
	for upperfName := range upperEntry.Childs {
		upperChild := upperEntry.Childs[upperfName]
		log.Debugf("Processsing %s", path.Join(upperDiff, upperChild.Name))
		upperDiffPath := path.Join(upperDiff, upperChild.Name)
		var mergeChild *FileEntry = nil
		if upperChild.IsNew() {
			log.Debugf("upperChild is New")
			c, err := copyDimg(upperChild, upperDiffPath, upperImgFile, mergeOut)
			if err != nil {
				return err
			}
			mergeChild = c
		} else {
			log.Debugf("upperChild is not New")
			if upperChild.IsSymlink() {
				log.Debugf("upperChild is symlink")
				mergeChild = upperChild.DeepCopy()
				mergeEntry.Childs[upperfName] = mergeChild
				continue
			}

			lowerChild, ok := lowerEntry.Childs[upperfName]
			if !ok {
				log.Errorf("lowerChild(%s) not found", path.Join(lowerDiff, upperChild.Name))
				return ErrUnexpected
			}
			lowerDiffPath := path.Join(lowerDiff, lowerChild.Name)
			log.Debugf("lowerChild is found(%s)", lowerDiffPath)

			if upperChild.IsDir() {
				log.Debugf("upperChild is dir")
				if lowerChild.IsDir() {
					mergeChild = upperChild.DeepCopy()
					err := MergeDiffDimg(lowerChild, upperChild, lowerDiffPath, upperDiffPath, lowerImgFile, upperImgFile, mergeChild, mergeOut)
					if err != nil {
						return err
					}
				} else {
					mergeChild = upperChild.DeepCopy()
					log.Debugf("Copy %v from upper", upperDiffPath)
					upperBytes := make([]byte, upperChild.CompressedSize)
					_, err := upperImgFile.ReadAt(upperBytes, upperChild.Offset)
					if err != nil {
						return err
					}
					mergeChild.Offset = int64(len(mergeOut.Bytes()))
					_, err = mergeOut.Write(upperBytes)
					if err != nil {
						return err
					}
				}
			} else {
				log.Debugf("upperChild is not dir")
				if lowerChild.IsSymlink() {
					log.Debugf("lowerChild is symlink")
					if !upperChild.IsSymlink() {
						mergeChild = upperChild.DeepCopy()
						log.Debugf("Copy %q from upper", upperDiffPath)
						upperBytes := make([]byte, upperChild.CompressedSize)
						_, err := upperImgFile.ReadAt(upperBytes, upperChild.Offset)
						if err != nil {
							return err
						}
						mergeChild.Offset = int64(len(mergeOut.Bytes()))
						_, err = mergeOut.Write(upperBytes)
						if err != nil {
							return err
						}
					} else {
						return ErrUnexpected
					}
				} else if lowerChild.IsSame() {
					log.Debugf("lowerChild is same")
					mergeChild = upperChild.DeepCopy()
					if !upperChild.IsSame() {
						// something diff
						log.Debugf("Copy %v from upper", upperDiffPath)
						upperBytes := make([]byte, upperChild.CompressedSize)
						_, err := upperImgFile.ReadAt(upperBytes, upperChild.Offset)
						if err != nil {
							return err
						}
						mergeChild.Offset = int64(len(mergeOut.Bytes()))
						_, err = mergeOut.Write(upperBytes)
						if err != nil {
							return err
						}
					}
				} else if lowerChild.IsNew() {
					log.Debugf("lowerChild is new")
					if upperChild.IsSame() {
						mergeChild = lowerChild.DeepCopy()
						log.Debugf("Copy %v from lower", lowerDiffPath)
						lowerBytes := make([]byte, lowerChild.CompressedSize)
						_, err := lowerImgFile.ReadAt(lowerBytes, lowerChild.Offset)
						if err != nil {
							return err
						}
						mergeChild.Offset = int64(len(mergeOut.Bytes()))
						_, err = mergeOut.Write(lowerBytes)
						if err != nil {
							return err
						}
					} else if !upperChild.IsNew() {
						log.Debugf("Apply patch %v to %v", lowerDiffPath, upperDiffPath)
						mergeChild = upperChild.DeepCopy()
						mergeChild.Type = FILE_ENTRY_FILE_NEW

						lowerBytes := make([]byte, lowerChild.CompressedSize)
						upperBytes := make([]byte, upperChild.CompressedSize)
						_, err := lowerImgFile.ReadAt(lowerBytes, lowerChild.Offset)
						if err != nil {
							return err
						}
						baseBuf := bytes.NewBuffer(nil)
						baseReader, err := zstd.NewReader(bytes.NewBuffer(lowerBytes))
						if err != nil {
							return err
						}
						defer baseReader.Close()
						_, err = io.Copy(baseBuf, baseReader)
						if err != nil {
							return err
						}

						_, err = upperImgFile.ReadAt(upperBytes, upperChild.Offset)
						if err != nil {
							return err
						}
						mergeBytes := bytes.NewBuffer(nil)
						err = bsdiff.Patch(bytes.NewBuffer(baseBuf.Bytes()), mergeBytes, bytes.NewBuffer(upperBytes))
						if err != nil {
							return err
						}

						mergeCompressed, err := CompressWithZstd(mergeBytes.Bytes())
						if err != nil {
							return err
						}
						mergeChild.Offset = int64(len(mergeOut.Bytes()))
						mergeChild.CompressedSize = int64(len(mergeCompressed))
						_, err = mergeOut.Write(mergeCompressed)
						if err != nil {
							return err
						}
					} else {
						return ErrUnexpected
					}
				} else {
					log.Debugf("lowerChild is diff")
					if upperChild.IsSame() {
						log.Debugf("upperChild is same")
						mergeChild = lowerChild.DeepCopy()
						log.Debugf("Copy %v from lower", lowerDiffPath)
						lowerBytes := make([]byte, lowerChild.CompressedSize)
						_, err := lowerImgFile.ReadAt(lowerBytes, lowerChild.Offset)
						if err != nil {
							return err
						}
						mergeChild.Offset = int64(len(mergeOut.Bytes()))
						_, err = mergeOut.Write(lowerBytes)
						if err != nil {
							return err
						}
					} else if !upperChild.IsNew() {
						log.Debugf("upperChild is diff")
						// DeltaMerging
						mergeChild = upperChild.DeepCopy()
						lowerBytes := make([]byte, lowerChild.CompressedSize)
						upperBytes := make([]byte, upperChild.CompressedSize)
						_, err := lowerImgFile.ReadAt(lowerBytes, lowerChild.Offset)
						if err != nil {
							return err
						}
						_, err = upperImgFile.ReadAt(upperBytes, upperChild.Offset)
						if err != nil {
							return err
						}
						mergeBytes := bytes.NewBuffer(nil)
						err = DeltaMergingBytes(bytes.NewBuffer(lowerBytes), bytes.NewBuffer(upperBytes), mergeBytes)
						if err != nil {
							return err
						}
						mergeChild.Offset = int64(len(mergeOut.Bytes()))
						mergeChild.CompressedSize = int64(len(mergeBytes.Bytes()))
						_, err = mergeOut.Write(mergeBytes.Bytes())
						if err != nil {
							return err
						}
					} else {
						return ErrUnexpected
					}
				}
			}
		}
		mergeEntry.Childs[upperfName] = mergeChild
	}

	return nil
}

func MergeDimg(lowerDimg, upperDimg string, merged io.Writer) (*DimgHeader, error) {
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
	mergedEntry := upperImgFile.Header().FileEntry.DeepCopy()
	lowerFE := &lowerImgFile.Header().FileEntry
	upperFE := &upperImgFile.Header().FileEntry
	err = MergeDiffDimg(lowerFE, upperFE, lowerFE.Name, upperFE.Name, lowerImgFile, upperImgFile, mergedEntry, &tmp)
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

func MergeCdimg(lowerCdimg, upperCdimg string, merged io.Writer) error {
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
	mergedEntry := upperDimg.Header().FileEntry.DeepCopy()
	lowerFE := &lowerDimg.Header().FileEntry
	upperFE := &upperDimg.Header().FileEntry
	err = MergeDiffDimg(lowerFE, upperFE, lowerFE.Name, upperFE.Name, lowerDimg, upperDimg, mergedEntry, &tmp)
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
