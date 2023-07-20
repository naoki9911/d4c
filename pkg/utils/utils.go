package utils

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"os"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/opencontainers/go-digest"
)

func UnmarshalJsonFromFile[T any](path string) (*T, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	jsonBytes, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	var res T
	err = json.Unmarshal(jsonBytes, &res)
	if err != nil {
		return nil, err
	}

	return &res, nil
}

func UnmarshalJsonFromReader[T any](r io.Reader) (*T, error) {
	jsonBytes, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	var res T
	err = json.Unmarshal(jsonBytes, &res)
	if err != nil {
		return nil, err
	}

	return &res, nil
}

func UnmarshalJsonFromCompressed[T any](b []byte) (*T, error) {
	buf := bytes.NewBuffer(b)
	reader, err := zstd.NewReader(buf)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	jsonBytes, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	var res T
	err = json.Unmarshal(jsonBytes, &res)
	if err != nil {
		return nil, err
	}

	return &res, nil
}

func DecompressWithZstd(b []byte) ([]byte, error) {
	buf := bytes.NewBuffer(b)
	reader, err := zstd.NewReader(buf)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	return io.ReadAll(reader)
}

func GetFileSizeAndDigest(path string) (int64, *digest.Digest, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, nil, err
	}
	defer file.Close()
	h := sha256.New()
	size, err := io.Copy(h, file)
	if err != nil {
		return 0, nil, err
	}
	d := digest.Digest("sha256:" + fmt.Sprintf("%x", h.Sum(nil)))
	return size, &d, nil
}

func GetSizeAndDigest(b []byte) (int64, *digest.Digest, error) {
	h := sha256.New()
	size, err := h.Write(b)
	if err != nil {
		return 0, nil, err
	}
	d := digest.Digest("sha256:" + fmt.Sprintf("%x", h.Sum(nil)))
	return int64(size), &d, nil
}

var (
	letters = []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
)

func randSequence(n int) string {
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func GetRandomId(prefix string) string {
	return fmt.Sprintf("%s-%s", prefix, randSequence(10))
}
