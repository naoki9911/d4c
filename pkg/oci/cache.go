package oci

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	v1 "github.com/google/go-containerregistry/pkg/v1"
)

var defaultCachePath = filepath.Join(os.TempDir(), "ctr-cli", "cache")

type BlobCache struct {
	cachePath string
}

func NewBlobCache() (*BlobCache, error) {
	c := &BlobCache{
		cachePath: defaultCachePath,
	}
	err := os.MkdirAll(c.cachePath, 0755)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache dir %s: %v", c.cachePath, err)
	}

	return c, nil
}

func (b *BlobCache) Store(d *v1.Hash, r io.Reader) error {
	name := filepath.Join(b.cachePath, d.String())
	f, err := os.Create(name)
	if err != nil {
		return fmt.Errorf("failed to create blob %s: %v", name, err)
	}
	defer f.Close()

	_, err = io.Copy(f, r)
	if err != nil {
		return fmt.Errorf("failed to copy: %v", err)
	}
	return nil
}

func (b *BlobCache) StoreBytes(d *v1.Hash, bytes []byte) error {
	name := filepath.Join(b.cachePath, d.String())
	f, err := os.Create(name)
	if err != nil {
		return fmt.Errorf("failed to create blob %s: %v", name, err)
	}
	defer f.Close()

	_, err = f.Write(bytes)
	if err != nil {
		return fmt.Errorf("failed to write content: %v", err)
	}
	return nil
}

func (b *BlobCache) Get(d *v1.Hash) (io.ReadCloser, error) {
	name := filepath.Join(b.cachePath, d.String())
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}

	return f, nil
}
