package bsdiffx

import (
	"encoding/binary"
	"errors"
	"io"

	"github.com/dsnet/compress/bzip2"
	"github.com/icedream/go-bsdiff/raw/diff"
	"github.com/icedream/go-bsdiff/raw/patch"
	"github.com/klauspost/compress/zstd"
)

var (
	ErrInvalidCompressionMode = errors.New("invalid compression mode")

	sizeEncoding = binary.BigEndian
)

type CompressionMode = uint8

const (
	CompressionModeBzip2 = CompressionMode(1)
	CompressionModeZstd  = CompressionMode(2)
)

func GetCompressMode(mode string) (CompressionMode, error) {
	switch mode {
	case "bzip2":
		return CompressionModeBzip2, nil
	case "zstd":
		return CompressionModeZstd, nil
	default:
		return 0, ErrInvalidCompressionMode
	}
}

func CompressionModeToString(cm CompressionMode) string {
	switch cm {
	case CompressionModeBzip2:
		return "bzip2"
	case CompressionModeZstd:
		return "zstd"
	default:
		return ""
	}
}

func WriteHeader(w io.Writer, size uint64, mode CompressionMode) (err error) {
	err = binary.Write(w, sizeEncoding, size)
	if err != nil {
		return
	}
	err = binary.Write(w, sizeEncoding, mode)
	return
}

func ReadHeader(r io.Reader) (size uint64, mode CompressionMode, err error) {
	err = binary.Read(r, sizeEncoding, &size)
	if err != nil {
		return
	}
	err = binary.Read(r, sizeEncoding, &mode)

	switch mode {
	case CompressionModeBzip2:
	case CompressionModeZstd:
	default:
		err = ErrInvalidCompressionMode
		return
	}

	return
}

func WritePatch(w io.Writer, newLen uint64, mode CompressionMode) (io.WriteCloser, error) {
	err := WriteHeader(w, newLen, mode)
	if err != nil {
		return nil, err
	}

	// Compression
	var writer io.WriteCloser
	switch mode {
	case CompressionModeBzip2:
		writer, err = bzip2.NewWriter(w, nil)
	case CompressionModeZstd:
		writer, err = zstd.NewWriter(w)
	}
	if err != nil {
		return nil, err
	}

	return writer, nil
}

func ReadPatch(r io.Reader) (io.Reader, uint64, CompressionMode, error) {
	newLen, compMode, err := ReadHeader(r)
	if err != nil {
		return nil, 0, 0, err
	}

	// Decompression
	var reader io.Reader
	switch compMode {
	case CompressionModeBzip2:
		reader, err = bzip2.NewReader(r, nil)
	case CompressionModeZstd:
		reader, err = zstd.NewReader(r)
	}
	if err != nil {
		return nil, 0, 0, err
	}

	return reader, newLen, compMode, nil
}

func Diff(oldBytes, newBytes []byte, patchWriter io.Writer, mode CompressionMode) error {
	writer, err := WritePatch(patchWriter, uint64(len(newBytes)), mode)
	if err != nil {
		return err
	}
	defer writer.Close()

	return diff.Diff(oldBytes, newBytes, writer)
}

func Patch(oldBytes []byte, patchReader io.Reader) ([]byte, error) {
	reader, newLen, _, err := ReadPatch(patchReader)
	if err != nil {
		return nil, err
	}

	return patch.Patch(oldBytes, reader, newLen)
}
