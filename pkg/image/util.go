package image

import (
	"fmt"
	"path/filepath"
)

type ImageFile interface {
	DimgHeader() *DimgHeader
	Close() error
}

func OpenDimgOrCdimg(path string) (ImageFile, error) {
	if filepath.Ext(path) == "cdimg" {
		cdimgFile, err := OpenCdimgFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to open cdimg %s: %v", path, err)
		}
		return cdimgFile, nil
	} else {
		dimgFile, err := OpenDimgFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to open dimg %s: %v", path, err)
		}
		return dimgFile, nil
	}
}
