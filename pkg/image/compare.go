package image

import (
	"fmt"
	"path/filepath"
)

type FileEntryCompareResult struct {
	Path                      string            `json:"path"`
	FileSize                  int               `json:"fileSize"`
	FileEntryAType            EntryType         `json:"fileEntryAType"`
	FileEntryACompressionSize int               `json:"fileEntryACompressionSize"`
	FileEntryBType            EntryType         `json:"fileEntryBType"`
	FileEntryBCompressionSize int               `json:"fileEntryBCompressionSize"`
	Labels                    map[string]string `json:"labels"`
}

func CompareFileEntries(feA, feB *FileEntry, pathPrefix string) ([]FileEntryCompareResult, error) {
	if feA.Name != feB.Name {
		return nil, fmt.Errorf("non-equal FileEntry Name A:%s B:%s", feA.Name, feB.Name)
	}
	path := filepath.Join(pathPrefix, feA.Name)
	if len(feA.Childs) != len(feB.Childs) {
		return nil, fmt.Errorf("non-equal FileEntry %s len(Childs) A:%d B:%d", path, len(feA.Childs), len(feB.Childs))
	}

	if feA.Size != feB.Size {
		return nil, fmt.Errorf("non-equal FileEntry %s Size A:%d B:%d", path, feA.Size, feB.Size)
	}

	if feA.HasBody() != feB.HasBody() {
		return nil, fmt.Errorf("non-equal FileEntry %s HasBody A:%v B:%v", path, feA.HasBody(), feB.HasBody())
	}

	if len(feA.Childs) != 0 && feA.HasBody() {
		return nil, fmt.Errorf("invalid FileEntry. %s has both Childs and Body", path)
	}

	if len(feA.Childs) == 0 {
		fecr := FileEntryCompareResult{
			Path:                      filepath.Join(pathPrefix, feA.Name),
			FileSize:                  feA.Size,
			FileEntryAType:            feA.Type,
			FileEntryACompressionSize: int(feA.CompressedSize),
			FileEntryBType:            feB.Type,
			FileEntryBCompressionSize: int(feB.CompressedSize),
		}
		return []FileEntryCompareResult{fecr}, nil
	}

	res := []FileEntryCompareResult{}
	for childName := range feA.Childs {
		fileName := fmt.Sprintf("%s/%s", pathPrefix, childName)
		feAChild, ok := feA.Childs[childName]
		if !ok {
			return nil, fmt.Errorf("FileEntry A does not have file %s", fileName)
		}
		feBChild, ok := feB.Childs[childName]
		if !ok {
			return nil, fmt.Errorf("FileEntry B does not have file %s", fileName)
		}
		r, err := CompareFileEntries(feAChild, feBChild, path)
		if err != nil {
			return nil, fmt.Errorf("failed to compre entries in %s: %v", path, err)
		}
		res = append(res, r...)
	}

	return res, nil
}
