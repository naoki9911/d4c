package image

import (
	"encoding/binary"
	"os"
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
