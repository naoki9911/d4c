package di3fs

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"syscall"
	"time"

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

	meta     *image.FileEntry
	baseMeta []*image.FileEntry
	data     []byte
	root     *Di3fsRoot
}

var _ = (fs.NodeGetattrer)((*Di3fsNode)(nil))
var _ = (fs.NodeOpener)((*Di3fsNode)(nil))
var _ = (fs.NodeReader)((*Di3fsNode)(nil))
var _ = (fs.NodeReaddirer)((*Di3fsNode)(nil))
var _ = (fs.NodeReadlinker)((*Di3fsNode)(nil))

func (dn *Di3fsNode) Getattr(ctx context.Context, f fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	log.Traceln("Getattr started")
	defer log.Traceln("Getattr finished")

	out.Mode = dn.meta.Mode & 0777
	out.Nlink = 1
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
		if baseMeta.Type == image.FILE_ENTRY_OPAQUE {
			return nil, nil
		}
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
				baseReader := bytes.NewBuffer(data)

				writer := new(bytes.Buffer)
				err = bsdiffx.Patch(baseReader, writer, patchReader)
				if err != nil {
					return nil, err
				}
				data = writer.Bytes()
			}
			return data, nil
		}
		diffIdxs = append(diffIdxs, i)
	}
	return nil, fmt.Errorf("not implemented")
}

func (dn *Di3fsNode) openFileInImage() (fs.FileHandle, uint32, syscall.Errno) {
	if len(dn.data) != 0 {
	} else if dn.meta.IsNew() {
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
		dn.data, err = io.ReadAll(patchReader)
		if err != nil {
			log.Errorf("failed to read with zstd Reader err=%s", err)
			return 0, 0, syscall.EIO
		}
	} else if dn.meta.IsSame() {
		data, err := dn.readBaseFiles()
		if err != nil {
			log.Errorf("failed to read from base: %v", err)
			return 0, 0, syscall.EIO
		}
		dn.data = data
	} else if dn.meta.Type == image.FILE_ENTRY_OPAQUE {
		dn.data = []byte{}
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

		baseReader := bytes.NewBuffer(baseData)

		writer := new(bytes.Buffer)
		err = bsdiffx.Patch(baseReader, writer, patchReader)
		if err != nil {
			log.Errorf("Open failed(bsdiff) err=%v", err)
			return 0, 0, syscall.EIO
		}
		dn.data = writer.Bytes()
		log.Debugf("Successfully patched %s", dn.meta.Name)
	}
	return nil, fuse.FOPEN_KEEP_CACHE | fuse.FOPEN_CACHE_DIR, 0
}

func (dn *Di3fsNode) Open(ctx context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {
	log.Traceln("Open started")
	defer log.Traceln("Open finished")
	return dn.openFileInImage()
}

func (dn *Di3fsNode) Read(ctx context.Context, f fs.FileHandle, data []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	log.Traceln("Read started")
	defer log.Traceln("Read finished")

	end := int64(off) + int64(len(data))
	log.Debugf("READ STARTED file=%s offset=%d len=%d", dn.meta.Name, off, len(data))

	if end > int64(len(dn.data)) {
		end = int64(len(dn.data))
	}
	log.Debugf("READ FINISHED file=%s offset=%d len=%d", dn.meta.Name, off, (end - off))
	return fuse.ReadResultData(dn.data[off:end]), 0
}

func (dn *Di3fsNode) Readlink(ctx context.Context) ([]byte, syscall.Errno) {
	log.Traceln("Readlink started")
	defer log.Traceln("Readlink finished")
	return []byte(dn.meta.RealPath), 0
}

func generateOpaqueFileEntry(dir *image.FileEntry) (*image.FileEntry, error) {
	fe := &image.FileEntry{
		Name:     ".wh..wh..opq",
		Size:     int(0),
		Mode:     uint32(dir.Mode),
		Type:     image.FILE_ENTRY_OPAQUE,
		RealPath: "",
		Childs:   map[string]*image.FileEntry{},
	}

	return fe, nil
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

	if dr.root.IsBase() && !dr.meta.IsDir() && !dr.meta.IsSymlink() && !dr.meta.IsNew() {
		log.Fatalf("invalid base image")
	}
	// here, rootNode is initialized
	//log.Debugf("base=%s patch=%s", dr.basePath, dr.patchPath)
	if dr.meta.IsDir() {
		for _, o := range dr.meta.OaqueFiles {
			if strings.Contains(o, "/") {
				continue
			}
			fe, err := generateOpaqueFileEntry(dr.meta)
			if err != nil {
				log.Fatalf("failed to generate opaqueFileEntry: %v", err)
			}
			n := newNode(fe, nil, dr.root)
			stableAttr := fs.StableAttr{}
			cNode := dr.NewPersistentInode(ctx, n, stableAttr)
			dr.AddChild(n.meta.Name, cNode, false)
		}
	}
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
		stableAttr := fs.StableAttr{}
		if c.IsDir() {
			stableAttr.Mode = fuse.S_IFDIR
		} else if c.IsSymlink() {
			stableAttr.Mode = fuse.S_IFLNK
		}
		cNode := dr.NewPersistentInode(ctx, n, stableAttr)
		dr.AddChild(n.meta.Name, cNode, false)
	}
}

type Di3fsRoot struct {
	baseImageFiles []*image.DimgFile
	diffImageFile  *image.DimgFile
	RootNode       *Di3fsNode
}

func (dr *Di3fsRoot) IsBase() bool {
	return dr.baseImageFiles == nil
}

func newNode(fe *image.FileEntry, baseFE []*image.FileEntry, root *Di3fsRoot) *Di3fsNode {
	node := &Di3fsNode{
		meta:     fe,
		baseMeta: baseFE,
		root:     root,
	}
	return node
}

func NewDi3fsRoot(opts *fs.Options, baseImages []*image.DimgFile, diffImage *image.DimgFile) (Di3fsRoot, error) {
	baseFEs := make([]*image.FileEntry, 0)
	for i := range baseImages {
		if baseImages[i] == nil {
			continue
		}
		baseFEs = append(baseFEs, &baseImages[i].Header().FileEntry)
	}
	rootNode := newNode(&diffImage.Header().FileEntry, baseFEs, nil)
	root := Di3fsRoot{
		baseImageFiles: baseImages,
		diffImageFile:  diffImage,
		RootNode:       rootNode,
	}
	rootNode.root = &root

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
	log.Infof("diffImage %s is loaded", diffImageFile.Header().Id)

	dimgIdx := 1
	parentImageId := diffImageFile.Header().ParentId
	for parentImageId != "" {
		parentImageFile, err := image.OpenDimgFile(dimgPaths[dimgIdx])
		if err != nil {
			panic(err)
		}
		defer parentImageFile.Close()
		parentImageFiles = append(parentImageFiles, parentImageFile)
		log.Infof("parentImage %s is loaded", parentImageFile.Header().Id)
		parentImageId = parentImageFile.Header().ParentId
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

	di3fsRoot, err := NewDi3fsRoot(opts, parentImageFiles, diffImageFile)
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

	return nil
}
