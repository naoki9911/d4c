package image

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/opencontainers/go-digest"
)

type DimgEntry struct {
	DimgHeader
	Path string
}

type DimgStore struct {
	storeDir    string
	dimgIds     map[digest.Digest]*DimgEntry
	dimgDigests map[digest.Digest]*DimgEntry
}

func NewDimgStore(storeDir string) (*DimgStore, error) {
	err := os.MkdirAll(storeDir, 0755)
	if err != nil {
		return nil, fmt.Errorf("failed to create dir %s: %v", storeDir, err)
	}

	s := &DimgStore{
		storeDir:    storeDir,
		dimgIds:     map[digest.Digest]*DimgEntry{},
		dimgDigests: map[digest.Digest]*DimgEntry{},
	}

	// check existing dimgs in image store
	err = s.Walk()
	if err != nil {
		return nil, fmt.Errorf("failed to walk: %v", err)
	}

	return s, nil
}

func (ds *DimgStore) Walk() error {
	dirs, err := os.ReadDir(ds.storeDir)
	if err != nil {
		return fmt.Errorf("failed to ReadDir %s: %v", ds.storeDir, err)
	}

	ds.dimgIds = map[digest.Digest]*DimgEntry{}
	ds.dimgDigests = map[digest.Digest]*DimgEntry{}

	for _, dir := range dirs {
		fPath := filepath.Join(ds.storeDir, dir.Name())
		if !dir.Type().IsRegular() {
			logger.Infof("%s is not regular file. ignored", fPath)
			continue
		}

		dimgFile, err := OpenDimgFile(fPath)
		if err != nil {
			logger.Infof("%s is invalid dimg file: %v", fPath, err)
			continue
		}
		header := dimgFile.Header()

		entry := &DimgEntry{
			DimgHeader: *header,
			Path:       fPath,
		}
		ds.dimgIds[header.Id] = entry
		ds.dimgDigests[header.Digest()] = entry
	}

	return nil
}

func (ds *DimgStore) AddDimg(dimgPath string) error {
	dimgFile, err := OpenDimgFile(dimgPath)
	if err != nil {
		return fmt.Errorf("failed to open dimg %s: %v", dimgPath, err)
	}

	header := dimgFile.Header()
	fPath := filepath.Join(ds.storeDir, fmt.Sprintf("%s.dimg", string(header.Digest())))
	err = os.Rename(dimgPath, fPath)
	if err != nil {
		return fmt.Errorf("failed to rename %s to %s", dimgPath, fPath)
	}

	entry := &DimgEntry{
		DimgHeader: *header,
		Path:       fPath,
	}
	ds.dimgIds[header.Id] = entry
	ds.dimgDigests[header.Digest()] = entry

	return nil
}

// string[0] == top
// string[1] == layer(parentId top.Id)
func (ds *DimgStore) GetDimgPaths(dimgDigest digest.Digest) ([]string, error) {
	targetDimg, ok := ds.dimgDigests[dimgDigest]
	if !ok {
		return nil, fmt.Errorf("dimg for %s not found", dimgDigest)
	}

	paths := []string{targetDimg.Path}
	nextId := targetDimg.ParentId
	for nextId != "" {
		dimg, ok := ds.dimgIds[nextId]
		if !ok {
			return nil, fmt.Errorf("dimg for %s not found", dimgDigest)
		}
		paths = append(paths, dimg.Path)
		nextId = dimg.ParentId
	}

	return paths, nil
}
