package image

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"os"

	"github.com/containerd/containerd/log"
	"github.com/klauspost/compress/zstd"
	"github.com/naoki9911/fuse-diff-containerd/pkg/utils"
	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

var logger = log.G(context.TODO())

type Di3FSImageHeader struct {
	ManifestSize   int64         `json:"manifestSize"`
	ManifestDigest digest.Digest `json:"manifestDigest"`
	ConfigSize     int64         `json:"configSize"`
	DimgSize       int64         `json:"dimgSize"`
}

type Di3FSImage struct {
	Header        Di3FSImageHeader
	ManifestBytes []byte
	Manifest      v1.Manifest
	ConfigBytes   []byte
	Config        v1.Image
	DImgDigest    digest.Digest
	DImgOffset    int64
	Image         *os.File
}

func (h *Di3FSImageHeader) pack() ([]byte, error) {
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

	dimgBytes, err := io.ReadAll(dimgFile)
	if err != nil {
		return err
	}

	err = PackIo(configFile, dimgBytes, outFile)
	if err != nil {
		return err
	}

	return nil
}

func PackIo(configReader io.Reader, dimg []byte, out io.Writer) error {
	header := Di3FSImageHeader{}
	outBytes := bytes.Buffer{}
	config, err := loadConfigFromReader(configReader)
	if err != nil {
		return err
	}
	dimgFileSize, dimgFilDigest, err := utils.GetSizeAndDigest(dimg)
	if err != nil {
		return err
	}

	config.RootFS.DiffIDs = []digest.Digest{*dimgFilDigest}
	configBytes, err := json.Marshal(config)
	if err != nil {
		return err
	}

	configSize, configDigest, err := utils.GetSizeAndDigest(configBytes)
	if err != nil {
		return err
	}

	manifest := v1.Manifest{
		MediaType: v1.MediaTypeImageManifest,
		Config: v1.Descriptor{
			MediaType: v1.MediaTypeImageConfig,
			Size:      configSize,
			Digest:    *configDigest,
		},
		Layers: []v1.Descriptor{
			{
				MediaType: v1.MediaTypeImageLayer,
				Size:      dimgFileSize,
				Digest:    *dimgFilDigest,
			},
		},
	}
	manifest.SchemaVersion = 2
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	_, manifestDigest, err := utils.GetSizeAndDigest(manifestBytes)
	if err != nil {
		return err
	}
	header.ManifestDigest = *manifestDigest

	header.ManifestSize, err = packBytes(manifestBytes, &outBytes)
	if err != nil {
		return err
	}
	logger.Debugf("compressed manifest (size=%d)", header.ManifestSize)

	header.ConfigSize, err = packBytes(configBytes, &outBytes)
	if err != nil {
		return err
	}
	logger.Debugf("compressed config (size=%d)", header.ConfigSize)

	header.DimgSize = dimgFileSize

	headerCompressedBytes, err := header.pack()
	if err != nil {
		return err
	}
	bs := make([]byte, 4)
	binary.LittleEndian.PutUint32(bs, uint32(len(headerCompressedBytes)))

	_, err = out.Write(append(bs, headerCompressedBytes...))
	if err != nil {
		return err
	}
	logger.WithField("header", header).Debugf("written Di3FSImageHeader")

	_, err = io.Copy(out, &outBytes)
	if err != nil {
		return err
	}
	_, err = out.Write(dimg)
	if err != nil {
		return err
	}
	logger.Debugf("written contents")

	return nil
}

func LoadHeader(r io.Reader) (*Di3FSImage, error) {
	var image Di3FSImage
	bs := make([]byte, 4)
	_, err := r.Read(bs)
	if err != nil {
		return nil, err
	}
	headerSize := binary.LittleEndian.Uint32(bs)
	curOffset := int64(len(bs))

	headerBytes := make([]byte, headerSize)
	_, err = r.Read(headerBytes)
	if err != nil {
		return nil, err
	}
	header, err := utils.UnmarshalJsonFromCompressed[Di3FSImageHeader](headerBytes)
	if err != nil {
		return nil, err
	}
	image.Header = *header
	curOffset += int64(len(headerBytes))

	// load manifest
	manifestZstdBytes := make([]byte, header.ManifestSize)
	_, err = r.Read(manifestZstdBytes)
	if err != nil {
		return nil, err
	}
	manifestBytes, err := utils.DecompressWithZstd(manifestZstdBytes)
	if err != nil {
		return nil, err
	}
	curOffset += int64(len(manifestZstdBytes))
	var manifest v1.Manifest
	err = json.Unmarshal(manifestBytes, &manifest)
	if err != nil {
		return nil, err
	}
	image.ManifestBytes = manifestBytes
	image.Manifest = manifest

	// load config
	configZstdBytes := make([]byte, header.ConfigSize)
	_, err = r.Read(configZstdBytes)
	if err != nil {
		return nil, err
	}
	configBytes, err := utils.DecompressWithZstd(configZstdBytes)
	if err != nil {
		return nil, err
	}
	curOffset += int64(len(configZstdBytes))
	var config v1.Image
	err = json.Unmarshal(configBytes, &config)
	if err != nil {
		return nil, err
	}
	image.ConfigBytes = configBytes
	image.Config = config

	image.DImgDigest = config.RootFS.DiffIDs[0]
	image.DImgOffset = curOffset
	return &image, nil
}

func Load(dimgPath string) (*Di3FSImage, error) {
	imgFile, err := os.Open(dimgPath)
	if err != nil {
		return nil, err
	}

	image, err := LoadHeader(imgFile)
	if err != nil {
		return nil, err
	}
	image.Image = imgFile

	return image, nil
}
