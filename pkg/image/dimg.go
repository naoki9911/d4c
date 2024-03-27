package image

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/klauspost/compress/zstd"
)

type ImageHeader struct {
	BaseId    string    `json:"baseID"`
	FileEntry FileEntry `json:"fileEntry"`
}

type DimgFile struct {
	imageHeader *ImageHeader
	file        *os.File
	bodyOffset  int64
}

func OpenDimgFile(path string) (*DimgFile, error) {
	imageFile, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	curOffset := int64(0)
	bs := make([]byte, 4)
	_, err = imageFile.ReadAt(bs, curOffset)
	if err != nil {
		return nil, err
	}
	curOffset += 4

	compressedHeaderSize := binary.LittleEndian.Uint32(bs)
	compressedHeader := make([]byte, compressedHeaderSize)
	_, err = imageFile.ReadAt(compressedHeader, curOffset)
	if err != nil {
		return nil, err
	}
	curOffset += int64(compressedHeaderSize)
	imageHeader, err := UnmarshalJsonFromCompressed[ImageHeader](compressedHeader)
	if err != nil {
		return nil, err
	}

	df := &DimgFile{
		imageHeader: imageHeader,
		file:        imageFile,
		bodyOffset:  curOffset,
	}
	return df, nil
}

func (df *DimgFile) ImageHeader() *ImageHeader {
	return df.imageHeader
}

func (df *DimgFile) Close() error {
	err := df.file.Close()
	if err != nil {
		return err
	}

	return nil
}

func (df *DimgFile) ReadAt(b []byte, off int64) (int, error) {
	return df.file.ReadAt(b, df.bodyOffset+off)
}

func WriteDimg(outDimg io.Writer, imageHeader *ImageHeader, body *bytes.Buffer) error {
	jsonBytes, err := json.Marshal(imageHeader)
	if err != nil {
		return fmt.Errorf("failed to marshal ImageHeader: %v", err)
	}

	// encode header
	var headerZstdBuffer bytes.Buffer
	headerZstd, err := zstd.NewWriter(&headerZstdBuffer)
	if err != nil {
		return fmt.Errorf("failed to create zstd.Wrtier: %v", err)
	}
	_, err = headerZstd.Write(jsonBytes)
	if err != nil {
		return fmt.Errorf("failed to write to zstd: %v", err)
	}
	err = headerZstd.Close()
	if err != nil {
		return fmt.Errorf("failed to clsoe zstd: %v", err)
	}

	bs := make([]byte, 4)
	binary.LittleEndian.PutUint32(bs, uint32(len(headerZstdBuffer.Bytes())))

	// Image format
	// [ length of compressed image header (4bit)]
	// [ compressed image header ]
	// [ content body ]

	_, err = outDimg.Write(append(bs, headerZstdBuffer.Bytes()...))
	if err != nil {
		return fmt.Errorf("failed to write header: %v", err)
	}

	_, err = io.Copy(outDimg, body)
	if err != nil {
		return fmt.Errorf("failed to write body: %v", err)
	}

	return nil
}
