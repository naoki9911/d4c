package image

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/containerd/containerd/log"
	"github.com/klauspost/compress/zstd"
	"github.com/naoki9911/fuse-diff-containerd/pkg/utils"
	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

var logger = log.G(context.TODO())

// Container delta image (Cdimg) format
// [ length of compressed CdimgHeadHeader (4bit)]
// [ compressed CdimgHeadHeader ]
// [ compressed CdimgHeader ]
// [ content body(dimg) ]

type CdimgHeadHeader struct {
	ConfigSize int64         `json:"configSize"`
	DimgSize   int64         `json:"dimgSize"`
	DimgDigest digest.Digest `json:"dimgDigest"`
}

type CdimgHeader struct {
	Head        CdimgHeadHeader
	Config      v1.Image
	ConfigBytes []byte
}

type CdimgFile struct {
	Header     *CdimgHeader
	Dimg       *DimgFile
	DimgOffset int64
}

func (cf *CdimgFile) DimgHeader() *DimgHeader {
	return cf.Dimg.DimgHeader()
}

func (cf *CdimgFile) Close() error {
	if cf.Dimg != nil {
		cf.Dimg.Close()
	}

	return nil
}

func (cf *CdimgFile) WriteDimg(writer io.Writer) error {
	_, err := cf.Dimg.file.Seek(cf.DimgOffset, 0)
	if err != nil {
		return fmt.Errorf("failed to seek dimg: %v", err)
	}
	_, err = io.Copy(writer, cf.Dimg.file)
	if err != nil {
		return fmt.Errorf("faield to copy dimg: %v", err)
	}

	return nil
}

func (h *CdimgHeadHeader) pack() ([]byte, error) {
	jsonBytes, err := json.Marshal(h)
	if err != nil {
		return nil, err
	}

	res, err := compressWithZstd(jsonBytes)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func compressWithZstd(src []byte) ([]byte, error) {
	out := &bytes.Buffer{}
	z, err := zstd.NewWriter(out)
	if err != nil {
		return nil, err
	}

	_, err = z.Write(src)
	if err != nil {
		return nil, err
	}

	err = z.Close()
	if err != nil {
		return nil, err
	}

	return out.Bytes(), nil
}

func compressWithZstdIo(src io.Reader) (*bytes.Buffer, error) {
	out := &bytes.Buffer{}
	z, err := zstd.NewWriter(out)
	if err != nil {
		return nil, err
	}

	_, err = io.Copy(z, src)
	if err != nil {
		return nil, err
	}

	err = z.Close()
	if err != nil {
		return nil, err
	}

	return out, nil
}

func packBytes(b []byte, out *bytes.Buffer) (int64, error) {
	compressed, err := compressWithZstd(b)
	if err != nil {
		return 0, err
	}
	writtenSize, err := out.Write(compressed)
	if err != nil {
		return 0, err
	}

	return int64(writtenSize), err
}

func loadConfigFromReader(r io.Reader) (*v1.Image, error) {
	return utils.UnmarshalJsonFromReader[v1.Image](r)
}

func PackCdimg(configPath, dimgPath, outPath string) error {
	outFile, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer outFile.Close()
	logger.Debugf("created outFile %q", outPath)

	configFile, err := os.Open(configPath)
	if err != nil {
		return err
	}
	defer configFile.Close()
	logger.Debugf("opened configFile %q", configPath)

	dimgFile, err := os.Open(dimgPath)
	if err != nil {
		return err
	}
	defer dimgFile.Close()
	logger.Debugf("opened dimgFile %q", dimgPath)

	dimgStat, err := dimgFile.Stat()
	if err != nil {
		return err
	}

	dimgHeader, _, err := LoadDimgHeader(dimgFile)
	if err != nil {
		return err
	}

	err = WriteCdimgHeader(configFile, dimgHeader, dimgStat.Size(), outFile)
	if err != nil {
		return err
	}

	_, err = dimgFile.Seek(0, 0)
	if err != nil {
		return err
	}

	_, err = io.Copy(outFile, dimgFile)
	if err != nil {
		return err
	}

	return nil
}

func WriteCdimgHeader(configReader io.Reader, dimgHeader *DimgHeader, dimgSize int64, out io.Writer) error {
	head := CdimgHeadHeader{}
	outBytes := bytes.Buffer{}
	config, err := loadConfigFromReader(configReader)
	if err != nil {
		return err
	}

	dimgId := dimgHeader.Id
	config.RootFS.DiffIDs = []digest.Digest{dimgId}
	configBytes, err := json.Marshal(config)
	if err != nil {
		return err
	}

	head.ConfigSize, err = packBytes(configBytes, &outBytes)
	if err != nil {
		return err
	}
	logger.Debugf("compressed config (size=%d)", head.ConfigSize)

	head.DimgDigest = dimgHeader.Digest()
	head.DimgSize = dimgSize

	headCompressedBytes, err := head.pack()
	if err != nil {
		return err
	}
	bs := make([]byte, 4)
	binary.LittleEndian.PutUint32(bs, uint32(len(headCompressedBytes)))

	_, err = out.Write(append(bs, headCompressedBytes...))
	if err != nil {
		return err
	}
	logger.WithField("head", head).Debugf("written CdimgHeadHeader")

	_, err = io.Copy(out, &outBytes)
	if err != nil {
		return err
	}

	return nil
}

func LoadCdimgHeader(r io.Reader) (*CdimgHeader, int64, error) {
	var header CdimgHeader
	bs := make([]byte, 4)
	_, err := r.Read(bs)
	if err != nil {
		return nil, 0, err
	}
	headerSize := binary.LittleEndian.Uint32(bs)
	curOffset := int64(len(bs))

	headerBytes := make([]byte, headerSize)
	_, err = r.Read(headerBytes)
	if err != nil {
		return nil, 0, err
	}
	head, err := utils.UnmarshalJsonFromCompressed[CdimgHeadHeader](headerBytes)
	if err != nil {
		return nil, 0, err
	}
	header.Head = *head
	curOffset += int64(len(headerBytes))

	// load config
	configZstdBytes := make([]byte, header.Head.ConfigSize)
	_, err = r.Read(configZstdBytes)
	if err != nil {
		return nil, 0, err
	}
	configBytes, err := utils.DecompressWithZstd(configZstdBytes)
	if err != nil {
		return nil, 0, err
	}
	curOffset += int64(len(configZstdBytes))
	var config v1.Image
	err = json.Unmarshal(configBytes, &config)
	if err != nil {
		return nil, 0, err
	}
	header.Config = config
	header.ConfigBytes = configBytes

	return &header, curOffset, nil
}

func OpenCdimgFile(path string) (*CdimgFile, error) {
	imgFile, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	header, dimgOffset, err := LoadCdimgHeader(imgFile)
	if err != nil {
		return nil, err
	}

	dimgHeader, dimgBodyOffsetInc, err := LoadDimgHeader(imgFile)
	if err != nil {
		return nil, err
	}

	return &CdimgFile{
		Header:     header,
		DimgOffset: dimgOffset,
		Dimg: &DimgFile{
			header:     dimgHeader,
			file:       imgFile,
			bodyOffset: dimgOffset + dimgBodyOffsetInc,
		},
	}, nil
}
