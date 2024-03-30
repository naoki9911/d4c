package bsdiffx

import (
	"encoding/binary"
	"errors"
	"io"

	"github.com/dsnet/compress/bzip2"
	"github.com/icedream/go-bsdiff/raw/diff"
	"github.com/icedream/go-bsdiff/raw/patch"
)

var (
	ErrInvalidCompressionMode = errors.New("invalid compression mode")

	sizeEncoding = binary.BigEndian
)

type CompressionMode = uint8

const (
	CompressionModeBzip2 = CompressionMode(1)
)

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

	mode = CompressionModeBzip2
	switch mode {
	case CompressionModeBzip2:
	default:
		err = ErrInvalidCompressionMode
		return
	}

	return
}

func WritePatch(w io.Writer, newLen uint64) (io.WriteCloser, error) {
	if err := WriteHeader(w, newLen, CompressionModeBzip2); err != nil {
		return nil, err
	}

	// Compression
	bz2Writer, err := bzip2.NewWriter(w, nil)
	if err != nil {
		return nil, err
	}

	return bz2Writer, nil
}

func ReadPatch(r io.Reader) (io.ReadCloser, uint64, error) {
	newLen, compMode, err := ReadHeader(r)
	if err != nil {
		return nil, 0, err
	}

	// Decompression
	var reader io.ReadCloser
	switch compMode {
	case CompressionModeBzip2:
		reader, err = bzip2.NewReader(r, nil)
		if err != nil {
			return nil, 0, err
		}
	}

	return reader, newLen, nil
}

func Diff(oldReader, newReader io.Reader, newSize int64, patchWriter io.Writer) error {
	writer, err := WritePatch(patchWriter, uint64(newSize))
	if err != nil {
		return err
	}
	defer writer.Close()

	return diff.Diff(oldReader, newReader, writer)
}

func Patch(oldReader io.Reader, newWriter io.Writer, patchReader io.Reader) error {
	reader, newLen, err := ReadPatch(patchReader)
	if err != nil {
		return err
	}
	defer reader.Close()

	return patch.Patch(oldReader, newWriter, reader, newLen)
}
