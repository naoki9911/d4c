package di3fs

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/klauspost/compress/zstd"
	"github.com/naoki9911/fuse-diff-containerd/pkg/bsdiffx"
	"github.com/naoki9911/fuse-diff-containerd/pkg/image"
	log "github.com/sirupsen/logrus"
)

type Di3fsNodeID uint64

type Di3fsNode struct {
	fs.Inode

	path      string
	linkNum   uint32
	openCount int
	openLock  sync.Mutex

	meta            *image.FileEntry
	baseMeta        []*image.FileEntry
	patchedFile     *os.File
	patchedFilePath string
	root            *Di3fsRoot
	plugin          *bsdiffx.Plugin
}

var _ = (fs.NodeGetattrer)((*Di3fsNode)(nil))
var _ = (fs.NodeOpener)((*Di3fsNode)(nil))
var _ = (fs.NodeReader)((*Di3fsNode)(nil))
var _ = (fs.NodeReaddirer)((*Di3fsNode)(nil))
var _ = (fs.NodeReadlinker)((*Di3fsNode)(nil))
var _ = (fs.FileReleaser)((*Di3fsNode)(nil))

func (dn *Di3fsNode) Getattr(ctx context.Context, f fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	log.Traceln("Getattr started")
	defer log.Traceln("Getattr finished")

	out.Mode = dn.meta.Mode & 0777
	out.Nlink = dn.linkNum
	out.Mtime = uint64(time.Now().Unix())
	out.Atime = out.Mtime
	out.Ctime = out.Mtime
	out.Size = uint64(dn.meta.Size)
	out.Uid = dn.meta.UID
	out.Gid = dn.meta.GID
	const bs = 512
	out.Blksize = bs
	out.Blocks = (out.Size + bs - 1) / bs
	return 0
}

func (dn *Di3fsNode) readBaseFiles() ([]byte, error) {
	diffIdxs := make([]int, 0)
	for i := 0; i < len(dn.baseMeta); i++ {
		baseMeta := dn.baseMeta[i]
		baseImageOffset := dn.baseMeta[i].Offset
		baseImageFile := dn.root.baseImageFiles[i]
		if baseMeta.IsSame() {
			continue
		}
		if baseMeta.IsNew() {
			zstdBytes := make([]byte, baseMeta.CompressedSize)
			_, err := baseImageFile.ReadAt(zstdBytes, baseImageOffset)
			if err != nil {
				return nil, fmt.Errorf("failed to read from image: %v", err)
			}
			zstdReader, err := zstd.NewReader(bytes.NewBuffer(zstdBytes))
			if err != nil {
				return nil, err
			}
			defer zstdReader.Close()
			data, err := io.ReadAll(zstdReader)
			if err != nil {
				return nil, err
			}
			if len(diffIdxs) == 0 {
				return data, nil
			}
			for j := len(diffIdxs) - 1; j >= 0; j -= 1 {
				diffIdx := diffIdxs[j]
				patchBytes := make([]byte, dn.baseMeta[diffIdx].CompressedSize)
				_, err := dn.root.baseImageFiles[diffIdx].ReadAt(patchBytes, dn.baseMeta[diffIdx].Offset)
				if err != nil {
					fmt.Println(err)
					return nil, err
				}
				patchReader := bytes.NewBuffer(patchBytes)

				newBytes, err := dn.plugin.Patch(data, patchReader)
				if err != nil {
					return nil, err
				}
				data = newBytes
			}
			return data, nil
		}
		diffIdxs = append(diffIdxs, i)
	}
	return nil, fmt.Errorf("not implemented")
}

func (dn *Di3fsNode) openFileInImage() (fs.FileHandle, uint32, syscall.Errno) {
	if dn.patchedFile != nil {
	} else if dn.patchedFilePath != "" {
		file, err := os.Open(dn.patchedFilePath)
		if err != nil {
			log.Errorf("failed to open existing patched file %s: %v", dn.patchedFilePath, err)
			return 0, 0, syscall.EIO
		}
		dn.patchedFile = file
	} else {
		var dataReader io.Reader
		if dn.meta.IsNew() {
			patchBytes := make([]byte, dn.meta.CompressedSize)
			_, err := dn.root.diffImageFile.ReadAt(patchBytes, dn.meta.Offset)
			if err != nil {
				log.Errorf("failed to read from diffImage offset=%d err=%s", dn.meta.Offset, err)
				return 0, 0, syscall.EIO
			}
			patchBuf := bytes.NewBuffer(patchBytes)
			patchReader, err := zstd.NewReader(patchBuf)
			if err != nil {
				log.Errorf("failed to create zstd Reader err=%s", err)
				return 0, 0, syscall.EIO
			}
			defer patchReader.Close()
			dataReader = patchReader
		} else if dn.meta.IsSame() {
			data, err := dn.readBaseFiles()
			if err != nil {
				log.Errorf("failed to read from base: %v", err)
				return 0, 0, syscall.EIO
			}
			dataReader = bytes.NewReader(data)
		} else {
			var patchReader io.Reader
			patchBytes := make([]byte, dn.meta.CompressedSize)
			_, err := dn.root.diffImageFile.ReadAt(patchBytes, dn.meta.Offset)
			if err != nil {
				log.Errorf("failed to read from diffImage offset=%d len=%d err=%s", dn.meta.Offset, len(patchBytes), err)
				return 0, 0, syscall.EIO
			}
			patchReader = bytes.NewBuffer(patchBytes)
			baseData, err := dn.readBaseFiles()
			if err != nil {
				log.Errorf("failed to read from base: %v", err)
				return 0, 0, syscall.EIO
			}

			newBytes, err := dn.plugin.Patch(baseData, patchReader)
			if err != nil {
				log.Errorf("Open failed(bsdiff) err=%v", err)
				return 0, 0, syscall.EIO
			}
			dataReader = bytes.NewReader(newBytes)
			log.Debugf("Successfully patched %s", dn.meta.Name)
		}

		data, err := io.ReadAll(dataReader)
		if err != nil {
			log.Errorf("failed to read all: %v", err)
			return 0, 0, syscall.EIO
		}
		err = dn.meta.Verify(data)
		if err != nil {
			log.Errorf("failed to verify %s(%d): %v", dn.path, dn.meta.Type, err)
			return 0, 0, syscall.EIO
		}

		dn.patchedFile, err = os.CreateTemp(dn.root.PatchedFilesDir, fmt.Sprintf("%s-*", dn.meta.Name))
		if err != nil {
			log.Errorf("failed to creat temporary file: %v", err)
			return 0, 0, syscall.EIO
		}
		_, err = dn.patchedFile.Write(data)
		if err != nil {
			log.Errorf("failed to write data: %v", err)
			return 0, 0, syscall.EIO
		}
		dn.patchedFilePath = dn.patchedFile.Name()
	}
	return nil, fuse.FOPEN_KEEP_CACHE | fuse.FOPEN_CACHE_DIR, 0
}

func (dn *Di3fsNode) Open(ctx context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {
	dn.openLock.Lock()
	defer dn.openLock.Unlock()

	defer func() {
		dn.openCount += 1
	}()

	log.Traceln("Open started")
	defer log.Traceln("Open finished")
	if dn.openCount == 0 {
		return dn.openFileInImage()
	}

	return nil, fuse.FOPEN_KEEP_CACHE | fuse.FOPEN_CACHE_DIR, 0
}

func (dn *Di3fsNode) Release(ctx context.Context) syscall.Errno {
	dn.openLock.Lock()
	defer dn.openLock.Unlock()

	dn.openCount -= 1
	if dn.openCount == 0 {
		// close patched file
		dn.patchedFile.Close()
		dn.patchedFile = nil
	}
	return 0
}

func (dn *Di3fsNode) Read(ctx context.Context, f fs.FileHandle, data []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	log.Traceln("Read started")
	defer log.Traceln("Read finished")

	end := int64(off) + int64(len(data))
	log.Debugf("READ STARTED file=%s offset=%d len=%d", dn.meta.Name, off, len(data))
	defer log.Debugf("READ FINISHED file=%s offset=%d len=%d", dn.meta.Name, off, (end - off))
	length := end - off
	readLen, err := dn.patchedFile.ReadAt(data[0:length], off)
	if err != nil && err != io.EOF {
		log.Errorf("failed to read from patched file: %v", err)
		return nil, syscall.EIO
	}
	return fuse.ReadResultData(data[0:readLen]), 0
}

func (dn *Di3fsNode) Readlink(ctx context.Context) ([]byte, syscall.Errno) {
	log.Traceln("Readlink started")
	defer log.Traceln("Readlink finished")
	return []byte(dn.meta.RealPath), 0
}

func (dn *Di3fsNode) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	log.Traceln("Readdir started")
	defer log.Traceln("Readdir finished")

	r := []fuse.DirEntry{}
	for k, ch := range dn.Children() {
		r = append(r, fuse.DirEntry{Mode: ch.Mode(),
			Name: k,
			Ino:  ch.StableAttr().Ino})
	}
	return fs.NewListDirStream(r), 0
}

func (dr *Di3fsNode) OnAdd(ctx context.Context) {
	log.Traceln("OnAdd started")
	defer log.Traceln("OnAdd finished")

	if dr.root.IsBase() && dr.meta.IsBaseRequired() {
		log.Fatalf("invalid base image")
	}

	if !dr.meta.IsFile() {
		err := dr.meta.Verify(nil)
		if err != nil {
			log.Fatalf("failed to verify %s: %v", dr.path, err)
		}
	}

	// here, rootNode is initialized
	//log.Debugf("base=%s patch=%s", dr.basePath, dr.patchPath)
	for childfName := range dr.meta.Childs {
		c := dr.meta.Childs[childfName]
		var childBaseFEs = make([]*image.FileEntry, 0)
		for baseMetaIdx := range dr.baseMeta {
			baseChild := dr.baseMeta[baseMetaIdx].Childs[childfName]
			if baseChild != nil {
				childBaseFEs = append(childBaseFEs, baseChild)
			}
		}
		n := newNode(c, childBaseFEs, dr.root)
		n.path = filepath.Join(dr.path, childfName)
		stableAttr := fs.StableAttr{}
		if c.IsDir() {
			stableAttr.Mode = fuse.S_IFDIR
		} else if c.Type == image.FILE_ENTRY_SYMLINK {
			stableAttr.Mode = fuse.S_IFLNK
		} else if c.Type == image.FILE_ENTRY_HARDLINK {
			hn := &hardlinkNode{
				parent: dr,
				entry:  c,
			}
			dr.root.hardlinks = append(dr.root.hardlinks, hn)
			continue
		}
		cNode := dr.NewPersistentInode(ctx, n, stableAttr)
		dr.AddChild(n.meta.Name, cNode, false)

		dr.root.nodes[n.path] = n
		dr.root.inodes[n.path] = cNode
	}

	// process hardlink
	if dr == dr.root.RootNode {
		for _, h := range dr.root.hardlinks {
			targetInode, ok := dr.root.inodes[h.entry.RealPath]
			if !ok {
				log.Fatalf("inode for %s does not exist", h.entry.RealPath)
			}
			targetNode, ok := dr.root.nodes[h.entry.RealPath]
			if !ok {
				log.Fatalf("node for %s does not exist", h.entry.RealPath)
			}
			targetNode.linkNum += 1
			h.parent.AddChild(h.entry.Name, targetInode, false)
		}
	}
}

type hardlinkNode struct {
	parent *Di3fsNode
	entry  *image.FileEntry
}

type Di3fsRoot struct {
	baseImageFiles  []*image.DimgFile
	diffImageFile   *image.DimgFile
	RootNode        *Di3fsNode
	inodes          map[string]*fs.Inode
	nodes           map[string]*Di3fsNode
	hardlinks       []*hardlinkNode
	pm              *bsdiffx.PluginManager
	PatchedFilesDir string
}

func (dr *Di3fsRoot) IsBase() bool {
	return dr.baseImageFiles == nil
}

func newNode(fe *image.FileEntry, baseFE []*image.FileEntry, root *Di3fsRoot) *Di3fsNode {
	var p *bsdiffx.Plugin = nil
	if root != nil {
		p = root.pm.GetPluginByExt(filepath.Ext(fe.Name))
	}
	node := &Di3fsNode{
		openCount: 0,
		openLock:  sync.Mutex{},
		linkNum:   1,
		meta:      fe,
		baseMeta:  baseFE,
		root:      root,
		plugin:    p,
	}
	return node
}

func NewDi3fsRoot(opts *fs.Options, baseImages []*image.DimgFile, diffImage *image.DimgFile, pm *bsdiffx.PluginManager) (*Di3fsRoot, error) {
	baseFEs := make([]*image.FileEntry, 0)
	for i := range baseImages {
		if baseImages[i] == nil {
			continue
		}
		baseFEs = append(baseFEs, &baseImages[i].DimgHeader().FileEntry)
	}
	dirUuid, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}
	rootNode := newNode(&diffImage.DimgHeader().FileEntry, baseFEs, nil)
	root := &Di3fsRoot{
		baseImageFiles:  baseImages,
		diffImageFile:   diffImage,
		RootNode:        rootNode,
		inodes:          map[string]*fs.Inode{},
		nodes:           map[string]*Di3fsNode{},
		hardlinks:       []*hardlinkNode{},
		pm:              pm,
		PatchedFilesDir: filepath.Join(os.TempDir(), fmt.Sprintf("di3fs-%s", dirUuid.String())),
	}
	err = os.MkdirAll(root.PatchedFilesDir, 0644)
	if err != nil {
		return nil, err
	}
	rootNode.root = root

	return root, nil
}

func Do(dimgPaths []string, mountPath string, mountDone chan bool) error {
	start := time.Now()
	customFormatter := new(log.TextFormatter)
	customFormatter.TimestampFormat = "2006-01-02 15:04:05"
	customFormatter.FullTimestamp = true
	log.SetFormatter(customFormatter)
	log.SetLevel(log.InfoLevel)

	parentImageFiles := make([]*image.DimgFile, 0)

	diffImageFile, err := image.OpenDimgFile(dimgPaths[0])
	if err != nil {
		panic(err)
	}
	defer diffImageFile.Close()
	log.Infof("diffImage %s is loaded", diffImageFile.DimgHeader().Id)

	dimgIdx := 1
	parentImageId := diffImageFile.DimgHeader().ParentId
	for parentImageId != "" {
		parentImageFile, err := image.OpenDimgFile(dimgPaths[dimgIdx])
		if err != nil {
			panic(err)
		}
		defer parentImageFile.Close()
		parentImageFiles = append(parentImageFiles, parentImageFile)
		log.Infof("parentImage %s is loaded", parentImageFile.DimgHeader().Id)
		parentImageId = parentImageFile.DimgHeader().ParentId
		dimgIdx += 1
	}

	sec := time.Second
	opts := &fs.Options{
		AttrTimeout:  &sec,
		EntryTimeout: &sec,
	}

	opts.MountOptions.Options = append(opts.MountOptions.Options, "ro")
	opts.MountOptions.Options = append(opts.MountOptions.Options, "fsname=fuse-diff")
	opts.MountOptions.Name = "fuse-diff"
	opts.NullPermissions = true

	pm, err := bsdiffx.LoadOrDefaultPlugins("")
	if err != nil {
		return err
	}

	di3fsRoot, err := NewDi3fsRoot(opts, parentImageFiles, diffImageFile, pm)
	if err != nil {
		log.Fatalf("creating Di3fsRoot failed: %v\n", err)
	}

	server, err := fs.Mount(mountPath, di3fsRoot.RootNode, opts)
	if err != nil {
		log.Fatalf("Mount fail: %v\n", err)
	}
	log.Infof("Mounted!")
	fmt.Printf("elapsed = %v\n", (time.Since(start).Milliseconds()))
	mountDone <- true
	server.Wait()

	os.RemoveAll(di3fsRoot.PatchedFilesDir)
	log.Infof("patched files dir %s has been cleanuuped", di3fsRoot.PatchedFilesDir)

	return nil
}
