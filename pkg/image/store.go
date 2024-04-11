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
	Path        string
	ConfigBytes []byte
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
		header := dimgFile.DimgHeader()

		entry := &DimgEntry{
			DimgHeader: *header,
			Path:       fPath,
		}
		ds.dimgGraph.Add(string(header.ParentId), header.Id.String(), string(header.Digest()), 1)
		ds.dimgDigests[header.Digest()] = entry
	}

	return nil
}

func (ds *DimgStore) AddDimg(dimgPath string, configBytes ...[]byte) error {
	ds.storeLock.Lock()
	defer ds.storeLock.Unlock()

	dimgFile, err := OpenDimgFile(dimgPath)
	if err != nil {
		return fmt.Errorf("failed to open dimg %s: %v", dimgPath, err)
	}

	header := dimgFile.DimgHeader()
	fPath := filepath.Join(ds.storeDir, fmt.Sprintf("%s.dimg", string(header.Digest())))
	err = os.Rename(dimgPath, fPath)
	if err != nil {
		return fmt.Errorf("failed to rename %s to %s", dimgPath, fPath)
	}

	entry := &DimgEntry{
		DimgHeader: *header,
		Path:       fPath,
	}

	if configBytes != nil {
		entry.ConfigBytes = configBytes[0]
	}

	// Id -> ParentId
	ds.dimgGraph.Add(header.Id.String(), header.ParentId.String(), header.Digest().String(), 1)
	ds.dimgDigests[header.Digest()] = entry

	return nil
}

// string[0] == top
// string[1] == layer(parentId top.Id)
func (ds *DimgStore) GetDimgPathsWithDimgId(dimgId digest.Digest) ([]string, error) {
	ds.storeLock.Lock()
	defer ds.storeLock.Unlock()

	// start search from current Id towards baseId(="")
	dimgs, err := ds.GetDimgEntriesWithDimgIds(dimgId, []digest.Digest{""})
	if err != nil {
		return nil, fmt.Errorf("failed to get dimgs: %v", err)
	}

	paths := make([]string, 0)
	for _, dimg := range dimgs {
		paths = append(paths, dimg.Path)
	}

	return paths, nil
}

func (ds *DimgStore) GetDimgEntriesWithDimgIds(startDimgId digest.Digest, goalDimgIds []digest.Digest) ([]*DimgEntry, error) {
	goals := []string{}
	for _, dimg := range goalDimgIds {
		goals = append(goals, dimg.String())
	}
	_, dimgs, err := ds.dimgGraph.ShortestPathWithMultipleGoals(startDimgId.String(), goals)
	if err != nil {
		return nil, fmt.Errorf("failed to get shortest path from %s to %v: %v", startDimgId, goals, err)
	}

	dimgChain := make([]*DimgEntry, len(dimgs))
	for i, dimgEdge := range dimgs {
		dimg, ok := ds.dimgDigests[digest.Digest(dimgEdge.GetName())]
		if !ok {
			return nil, fmt.Errorf("dimg for %s not found", dimgEdge.GetName())
		}
		dimgChain[i] = dimg
	}

	return dimgChain, nil
}
