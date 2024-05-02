package bsdiffx

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"plugin"

	"github.com/google/uuid"
	"github.com/naoki9911/fuse-diff-containerd/pkg/utils"
)

type PluginEntry struct {
	Name string    `json:"name"`
	Uuid uuid.UUID `json:"uuid"`
	Path string    `json:"path"`
	Ext  string    `json:"ext"`
	Size int       `json:"size"`

	p *Plugin `json:"-"`
}

type PluginManager struct {
	defaultPlugin *Plugin
	plugins       []PluginEntry
}

func LoadOrDefaultPlugins(path string) (*PluginManager, error) {
	d4cBinPath, err := os.Executable()
	if err != nil {
		return nil, err
	}
	xdelta3Plugin := filepath.Join(filepath.Dir(d4cBinPath), "plugin_xdelta3.so")
	bsdiffxPlugin := filepath.Join(filepath.Dir(d4cBinPath), "plugin_bsdiffx.so")
	_ = xdelta3Plugin
	mgr := &PluginManager{
		defaultPlugin: DefaultPluigin(),
		plugins: []PluginEntry{
			{
				Name: "xdelta3",
				Path: xdelta3Plugin,
				Size: 512 * 1024 * 1024, // handle more than 512 MiB file
			},
			{
				Name: "bsdiffx",
				Path: bsdiffxPlugin,
			},
		},
	}
	if path == "" {
		for i := range mgr.plugins {
			pe := &mgr.plugins[i]
			p, err := OpenPlugin(pe.Path)
			if err != nil {
				return nil, fmt.Errorf("failed to open %s (%s): %v", pe.Name, pe.Path, err)
			}
			pe.p = p
			pe.Uuid = p.ID()
		}
		return mgr, nil
	}

	entries, err := utils.UnmarshalJsonFromFile[[]PluginEntry](path)
	if err != nil {
		return nil, fmt.Errorf("failed to load plugin entry %s: %v", path, err)
	}

	for i := range *entries {
		pe := (*entries)[i]
		p, err := OpenPlugin(pe.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to open %s (%s): %v", pe.Name, pe.Path, err)
		}
		pe.p = p
		mgr.plugins = append(mgr.plugins, pe)
	}

	return mgr, nil
}

func (pm *PluginManager) GetPluginByExt(ext string) *Plugin {
	for i := range pm.plugins {
		pe := pm.plugins[i]
		if pe.Ext == ext {
			return pe.p
		}
	}

	return pm.defaultPlugin
}

func (pm *PluginManager) GetPluginByName(name string) *Plugin {
	for i := range pm.plugins {
		pe := pm.plugins[i]
		if pe.Name == name {
			return pe.p
		}
	}

	return nil
}

func (pm *PluginManager) GetPluginBySize(size int) *Plugin {
	for i := range pm.plugins {
		pe := pm.plugins[i]
		if size > pe.Size {
			return pe.p
		}
	}

	return pm.defaultPlugin
}

func (pm *PluginManager) GetPluginByUuid(uuid uuid.UUID) *Plugin {
	for i := range pm.plugins {
		pe := pm.plugins[i]
		if pe.Uuid == uuid {
			return pe.p
		}
	}

	return nil
}

type Plugin struct {
	p       *plugin.Plugin
	info    func() string
	diff    func(oldBytes, newBytes []byte, patchWriter io.Writer, mode CompressionMode) error
	patch   func(oldBytes []byte, patchReader io.Reader) ([]byte, error)
	merge   func(lowerDiff, upperDiff io.Reader, mergedDiff io.Writer) error
	compare func(a, b []byte) bool
	id      func() uuid.UUID
}

func defaultPluginInfo() string {
	return "Default plugin with bsdiffx"
}

func defaultCompare(a, b []byte) bool {
	return bytes.Equal(a, b)
}

func DefaultPluigin() *Plugin {
	p := &Plugin{}

	p.info = defaultPluginInfo
	p.diff = Diff
	p.patch = Patch
	p.merge = DeltaMergingBytes
	p.compare = defaultCompare

	return p
}

func OpenPlugin(path string) (*Plugin, error) {
	p, err := plugin.Open(path)
	if err != nil {
		return nil, err
	}

	plugin := &Plugin{
		p: p,
	}

	sInfo, err := p.Lookup("Info")
	if err != nil {
		return nil, err
	}

	plugin.info = sInfo.(func() string)

	sDiff, err := p.Lookup("Diff")
	if err != nil {
		return nil, err
	}
	plugin.diff = sDiff.(func(oldBytes, newBytes []byte, patchWriter io.Writer, mode CompressionMode) error)

	sPatch, err := p.Lookup("Patch")
	if err != nil {
		return nil, err
	}
	plugin.patch = sPatch.(func(oldBytes []byte, patchReader io.Reader) ([]byte, error))

	sMerge, err := p.Lookup("Merge")
	if err != nil {
		return nil, err
	}
	plugin.merge = sMerge.(func(lowerDiff, upperDiff io.Reader, mergedDiff io.Writer) error)

	sCompare, err := p.Lookup("Compare")
	if err != nil {
		return nil, err
	}
	plugin.compare = sCompare.(func(a, b []byte) bool)

	sID, err := p.Lookup("ID")
	if err != nil {
		return nil, err
	}
	plugin.id = sID.(func() uuid.UUID)

	return plugin, nil
}

func (p *Plugin) Info() string {
	return p.info()
}

func (p *Plugin) Diff(oldBytes, newBytes []byte, patchWriter io.Writer, mode CompressionMode) error {
	return p.diff(oldBytes, newBytes, patchWriter, mode)
}

func (p *Plugin) Patch(oldBytes []byte, patchReader io.Reader) ([]byte, error) {
	return p.patch(oldBytes, patchReader)
}

func (p *Plugin) Merge(lowerDiff, upperDiff io.Reader, mergedDiff io.Writer) error {
	return p.merge(lowerDiff, upperDiff, mergedDiff)
}

func (p *Plugin) Compare(a, b []byte) bool {
	return p.compare(a, b)
}

func (p *Plugin) ID() uuid.UUID {
	return p.id()
}
