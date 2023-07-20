package main

import (
	"bytes"
	"compress/bzip2"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/pkg/errors"
)

var ErrInvalidMagic = errors.New("Invalid magic")
var sizeEncoding = binary.BigEndian
var magicText = []byte("ENDSLEY/BSDIFF43")

func ReadHeader(r io.Reader) (size uint64, err error) {
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

func readPatch(reader io.Reader) (io.Reader, uint64, error) {
	newLen, err := ReadHeader(reader)
	if err != nil {
		return nil, 0, err
	}
	fmt.Printf("newBytes: %d\n", newLen)

	// Decompression
	bz2Reader := bzip2.NewReader(reader)
	content, err := io.ReadAll(bz2Reader)
	if err != nil {
		return nil, 0, err
	}

	return bytes.NewReader(content), newLen, nil
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

func readContent(newSize uint64, reader io.Reader, oldFile *os.File) error {
	newPos := int64(0)
	oldPos := int64(0)

	//fmt.Printf("OP,OldOffset,NewOffset,NewValue,OldValue\n")
	fmt.Printf("Offset,NOUPDATE,ADD,INTERT,MOVE\n")
	for newPos < int64(newSize) {
		ctrl0, err := readInt64(reader)
		if err != nil {
			return err
		}
		ctrl1, err := readInt64(reader)
		if err != nil {
			return err
		}
		ctrl2, err := readInt64(reader)
		if err != nil {
			return err
		}

		if uint64(newPos+ctrl0) > newSize {
			return fmt.Errorf("newPos + ctrl0 exceeds newSize")
		}
		//fmt.Printf("ctrl0=%d\n", ctrl0)
		//fmt.Printf("ctrl1=%d\n", ctrl1)
		//fmt.Printf("ctrl2=%d\n", ctrl2)

		diff := make([]byte, ctrl0)
		diffSize, err := reader.Read(diff)
		if err != nil {
			return err
		}
		if int(ctrl0) != diffSize {
			return fmt.Errorf("invalid size expected=%d actual=%d", ctrl0, diffSize)
		}
		for i := int64(0); i < int64(diffSize); i++ {
			if diff[i] == 0 {
				fmt.Printf("%d,1,,,\n", newPos+i)
				continue
			}
			oldC := make([]byte, 1)
			_, err = oldFile.ReadAt(oldC, oldPos+i)
			if err != nil {
				return err
			}
			//fmt.Printf("ADD,%d,%d,%d,%d\n", oldPos+i, newPos+i, oldC[0]+diff[i], oldC[0])
			fmt.Printf("%d,,1,,\n", newPos+i)
		}

		newPos += ctrl0
		oldPos += ctrl0

		insert := make([]byte, ctrl1)
		insertSize, err := reader.Read(insert)
		if err != nil {
			return err
		}
		if int(ctrl1) != insertSize {
			return fmt.Errorf("invalid size expected=%d actual=%d", ctrl1, insertSize)
		}
		for i := int64(0); i < int64(insertSize); i++ {
			fmt.Printf("%d,,,1,\n", newPos+i)
			//fmt.Printf("INSERT,%d,%d,%d\n", oldPos+i, newPos+i, insert[i])
		}
		//if ctrl1 != 0 {
		//	for i := int64(0); i < ctrl1; i++ {
		//		fmt.Printf("%d,,,,1\n", newPos+i)
		//	}
		//}
		newPos += ctrl1
		oldPos += ctrl2
	}

	return nil
}

func main() {
	oldFile, err := os.Open(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	file, err := os.Open(os.Args[2])
	if err != nil {
		log.Fatal(err)
	}
	reader, newLen, err := readPatch(file)
	if err != nil {
		log.Fatal(err)
	}
	err = readContent(newLen, reader, oldFile)
	if err != nil {
		fmt.Printf("%+v\n", err)
	}
}
