// some codes are retrieved from https://github.com/imjasonh/kontain.me/blob/main/cmd/flatten/main.go
package oci

import (
	"context"
	"fmt"
	"io"

	"github.com/containerd/containerd/log"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/sirupsen/logrus"
)

var acceptableMediaTypes = map[types.MediaType]bool{
	types.DockerManifestSchema2: true,
	types.DockerManifestList:    true,
	types.OCIImageIndex:         true,
	types.OCIManifestSchema1:    true,
}

type Puller struct {
	logger *logrus.Entry
}

func NewPuller() *Puller {
	p := &Puller{
		logger: log.G(context.TODO()),
	}

	return p
}

func (p *Puller) Pull(imageName string, os, arch string) (v1.Layer, *v1.ConfigFile, error) {
	refstr := imageName
	ref, err := name.ParseReference(refstr)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to pull: %v", err)
	}

	var idx v1.ImageIndex
	var img v1.Image

	// Determine whether the ref is for an image or index.
	d, err := remote.Head(ref)
	if err != nil {
		p.logger.Warnf("failed to Head ref %s: %v", ref, err)
		// HEAD failed, let's figure out if it was an index or image by doing GETs.
		idx, err = remote.Index(ref)
		if err != nil {
			p.logger.Warnf("failed to get Index for %s: %v", ref, err)
			img, err = remote.Image(ref)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to retrieve image: %v", err)
			}
		}

		if idx != nil {
			_, err = idx.Digest()
		} else if img != nil {
			_, err = img.Digest()
		}
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get digest: %v", err)
		}
	} else {
		if !acceptableMediaTypes[d.MediaType] {
			return nil, nil, fmt.Errorf("unknown media type: %s", d.MediaType)
		}

		switch d.MediaType {
		case types.OCIImageIndex, types.DockerManifestList:
			idx, err = remote.Index(ref)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get Index for %s: %v", ref, err)
			}
		case types.OCIManifestSchema1, types.DockerManifestSchema2:
			img, err = remote.Image(ref)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get Image for %s: %v", ref, err)
			}
		}
	}

	var layer v1.Layer
	var config *v1.ConfigFile
	if idx != nil {
		layer, config, err = p.retrieveFlattenLayerFromIndex(idx, os, arch)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to flattenIndex: %v", err)
		}
	} else if img != nil {
		layer, config, err = p.retrieveFlattenLayerFromImage(img, os, arch)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to flatten: %v", err)
		}
	}

	return layer, config, nil
}

func (p *Puller) retrieveFlattenLayerFromIndex(idx v1.ImageIndex, os, arch string) (v1.Layer, *v1.ConfigFile, error) {
	im, err := idx.IndexManifest()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to IndexManifest: %v", err)
	}

	manifests := []v1.Descriptor{}
	for i, m := range im.Manifests {
		if m.Platform.OS != os {
			continue
		}
		if m.Platform.Architecture != arch {
			continue
		}
		manifests = append(manifests, im.Manifests[i])
	}

	if len(manifests) == 0 {
		return nil, nil, fmt.Errorf("no available Image found for os=%s arch=%s", os, arch)
	}

	if len(manifests) > 1 {
		s := ""
		for _, m := range manifests {
			s += fmt.Sprintf("{%v},", m.Platform)
		}
		return nil, nil, fmt.Errorf("multiple avaiable Imageas found %s", s[:len(s)-1])
	}

	m := manifests[0]
	img, err := idx.Image(m.Digest)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to Image with %s: %v", m.Digest, err)
	}

	layer, config, err := p.retrieveFlattenLayerFromImage(img, os, arch)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to flatten: %v", err)
	}

	if config.Architecture != arch {
		return nil, nil, fmt.Errorf("unexpected architecture in config expected=%s actual=%s", arch, config.Architecture)
	}

	if config.OS != os {
		return nil, nil, fmt.Errorf("unexpected os in config expected=%s actual=%s", os, config.OS)
	}

	return layer, config, nil
}

func (p *Puller) retrieveFlattenLayerFromImage(img v1.Image, os, arch string) (v1.Layer, *v1.ConfigFile, error) {
	l, err := tarball.LayerFromOpener(func() (io.ReadCloser, error) { return mutate.Extract(img), nil })
	if err != nil {
		return nil, nil, fmt.Errorf("failed tarball.LayerFromOpener: %v", err)
	}

	config, err := img.ConfigFile()
	if err != nil {
		return nil, nil, fmt.Errorf("failed img.ConfigFile: %v", err)
	}

	if config.Architecture != arch {
		return nil, nil, fmt.Errorf("unexpected architecture in config expected=%s actual=%s", arch, config.Architecture)
	}

	if config.OS != os {
		return nil, nil, fmt.Errorf("unexpected os in config expected=%s actual=%s", os, config.OS)
	}

	return l, config, nil
}
