package server

import (
	"fmt"

	"github.com/opencontainers/go-digest"
)

type ImageTag struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func (it *ImageTag) String() string {
	return fmt.Sprintf("%s:%s", it.Name, it.Version)
}

func (it *ImageTag) Exist() bool {
	return it.Name != "" && it.Version != ""
}

type DiffData struct {
	ImageTag  ImageTag `json:"imageTag"`
	CdimgPath string   `json:"cdimgPath"`
}

type UpdateDataRequest struct {
	RequestImage ImageTag        `json:"requestImage"`
	LocalDimgs   []digest.Digest `json:"localDiffs"` // list of DimgHeader.Id
}

type UpdateDataResponse struct {
	ImageTag
	SourceDimgs []digest.Digest `json:"sourceDimgs"`
}
