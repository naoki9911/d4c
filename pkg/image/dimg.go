package image

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/klauspost/compress/zstd"
	"github.com/opencontainers/go-digest"
)

type DimgHeader struct {
	Id        digest.Digest `json:"id"` // generated when the full image generated
	ParentId  digest.Digest `json:"parentID"`
	FileEntry FileEntry     `json:"fileEntry"`
}

func (dh *DimgHeader) Digest() digest.Digest {
	dhBytes, err := json.Marshal(dh)
	if err != nil {
		panic(err)
	}
	return digest.FromBytes(dhBytes)
}

type DimgFile struct {
	header     *DimgHeader
	file       *os.File
	bodyOffset int64
}

func OpenDimgFile(path string) (*DimgFile, error) {
	imageFile, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	header, offset, err := LoadDimgHeader(imageFile)
	if err != nil {
		return nil, err
	}

	df := &DimgFile{
		header:     header,
		file:       imageFile,
		bodyOffset: offset,
	}
	return df, nil
}

// reader must be seek at the head of dimg header
func LoadDimgHeader(reader io.Reader) (*DimgHeader, int64, error) {
	curOffset := int64(0)
	bs := make([]byte, 4)
	_, err := reader.Read(bs)
	if err != nil {
		return nil, 0, err
	}
	curOffset += 4

	compressedHeaderSize := binary.LittleEndian.Uint32(bs)
	compressedHeader := make([]byte, compressedHeaderSize)
	_, err = reader.Read(compressedHeader)
	if err != nil {
		return nil, 0, err
	}
	curOffset += int64(compressedHeaderSize)
	header, err := UnmarshalJsonFromCompressed[DimgHeader](compressedHeader)
	if err != nil {
		return nil, 0, err
	}

	return header, curOffset, nil
}

func (df *DimgFile) Header() *DimgHeader {
	return df.header
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

func WriteDimg(outDimg io.Writer, header *DimgHeader, body *bytes.Buffer) error {
	jsonBytes, err := json.Marshal(header)
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
