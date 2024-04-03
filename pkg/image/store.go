package image

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/naoki9911/fuse-diff-containerd/pkg/algorithm"
	"github.com/opencontainers/go-digest"
)

type DimgEntry struct {
	DimgHeader
	Path string
}

type DimgStore struct {
	storeDir    string
	storeLock   sync.Mutex
	dimgGraph   *algorithm.DirectedGraph
	dimgDigests map[digest.Digest]*DimgEntry
}

func NewDimgStore(storeDir string) (*DimgStore, error) {
	err := os.MkdirAll(storeDir, 0755)
	if err != nil {
		return nil, fmt.Errorf("failed to create dir %s: %v", storeDir, err)
	}

	s := &DimgStore{
		storeDir:    storeDir,
		storeLock:   sync.Mutex{},
		dimgGraph:   algorithm.NewDirectedGraph(),
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
	ds.storeLock.Lock()
	defer ds.storeLock.Unlock()

	dirs, err := os.ReadDir(ds.storeDir)
	if err != nil {
		return fmt.Errorf("failed to ReadDir %s: %v", ds.storeDir, err)
	}

	ds.dimgGraph = algorithm.NewDirectedGraph()
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
		ds.dimgGraph.Add(string(header.ParentId), header.Id.String(), string(header.Digest()), 1)
		ds.dimgDigests[header.Digest()] = entry
	}

	return nil
}

func (ds *DimgStore) AddDimg(dimgPath string) error {
	ds.storeLock.Lock()
	defer ds.storeLock.Unlock()

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
	ds.dimgGraph.Add(header.ParentId.String(), header.Id.String(), header.Digest().String(), 1)
	ds.dimgDigests[header.Digest()] = entry

	return nil
}

// string[0] == top
// string[1] == layer(parentId top.Id)
func (ds *DimgStore) GetDimgPathsWithDimgDigest(dimgDigest digest.Digest) ([]string, error) {
	ds.storeLock.Lock()
	defer ds.storeLock.Unlock()

	targetDimg, ok := ds.dimgDigests[dimgDigest]
	if !ok {
		return nil, fmt.Errorf("dimg for %s not found", dimgDigest)
	}

	_, dimgs, err := ds.dimgGraph.ShortestPath("", targetDimg.Id.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get shortest path for %s: %v", targetDimg.Id, err)
	}

	paths := make([]string, len(dimgs))
	for i, dimgEdge := range dimgs {
		dimg, ok := ds.dimgDigests[digest.Digest(dimgEdge.GetName())]
		if !ok {
			return nil, fmt.Errorf("dimg for %s not found", dimgEdge.GetName())
		}
		// dimgs are ordered from base to top
		// we need to reverse the path
		paths[len(dimgs)-i-1] = dimg.Path
	}

	return paths, nil
}
