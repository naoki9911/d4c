package di3fs

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/icedream/go-bsdiff"
	"github.com/klauspost/compress/zstd"
	log "github.com/sirupsen/logrus"
)

type Di3fsNodeID uint64

type Di3fsNode struct {
	fs.Inode

	basePath  []string
	patchPath string
	meta      *FileEntry
	baseMeta  []*FileEntry
	file      *os.File
	data      []byte
	root      *Di3fsRoot
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
		baseImage := dn.root.baseImage[i]
		baseImageBodyOffset := dn.root.baseImageBodyOffset[i]
		if baseMeta.Type == FILE_ENTRY_OPAQUE {
			return nil, nil
		}
		if baseMeta.IsSame() {
			continue
		}
		if baseMeta.IsNew() {
			zstdBytes := make([]byte, baseMeta.CompressedSize)
			_, err := baseImage.ReadAt(zstdBytes, baseImageBodyOffset+baseImageOffset)
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
				_, err := dn.root.baseImage[diffIdx].ReadAt(patchBytes, dn.root.baseImageBodyOffset[diffIdx]+dn.baseMeta[diffIdx].Offset)
				if err != nil {
					fmt.Println(err)
					return nil, err
				}
				patchReader := bytes.NewBuffer(patchBytes)
				baseReader := bytes.NewBuffer(data)

				writer := new(bytes.Buffer)
				err = bsdiff.Patch(baseReader, writer, patchReader)
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
	if dn.file != nil || len(dn.data) != 0 {
	} else if dn.meta.IsNew() {
		patchBytes := make([]byte, dn.meta.CompressedSize)
		offset := dn.root.diffImageBodyOffset + dn.meta.Offset
		_, err := dn.root.diffImage.ReadAt(patchBytes, dn.root.diffImageBodyOffset+dn.meta.Offset)
		if err != nil {
			log.Errorf("failed to read from diffImage offset=%d err=%s", offset, err)
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
	} else if dn.meta.Type == FILE_ENTRY_OPAQUE {
		dn.file = nil
		dn.data = []byte{}
	} else {
		var patchReader io.Reader
		patchBytes := make([]byte, dn.meta.CompressedSize)
		offset := dn.root.diffImageBodyOffset + dn.meta.Offset
		_, err := dn.root.diffImage.ReadAt(patchBytes, offset)
		if err != nil {
			log.Errorf("failed to read from diffImage offset=%d len=%d err=%s", offset, len(patchBytes), err)
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
		err = bsdiff.Patch(baseReader, writer, patchReader)
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
	isImage := dn.root.diffImage != nil
	if isImage {
		return dn.openFileInImage()
	}

	if dn.file != nil || len(dn.data) != 0 {
	} else if dn.meta.IsNew() {
		file, err := os.OpenFile(dn.patchPath, os.O_RDONLY, 0)
		if err != nil {
			log.Errorf("Open failed(new=%s) err=%v", dn.patchPath, err)
			return 0, 0, syscall.ENOENT
		}
		log.Debugf("Successfully opened new file=%s", dn.patchPath)
		dn.file = file
	} else if dn.meta.IsSame() {
		file, err := os.OpenFile(dn.basePath[0], os.O_RDONLY, 0)
		if err != nil {
			log.Errorf("Open failed(same=%s) err=%v", dn.basePath, err)
			return 0, 0, syscall.ENOENT
		}
		log.Debugf("Successfully opened same file=%s", dn.basePath)
		dn.file = file
	} else if dn.meta.Type == FILE_ENTRY_OPAQUE {
		dn.file = nil
		dn.data = []byte{}
	} else {
		patchFile, err := os.OpenFile(dn.patchPath, os.O_RDONLY, 0)
		if err != nil {
			log.Errorf("Open failed(patch=%s) err=%v", dn.patchPath, err)
			return 0, 0, syscall.ENOENT
		}
		defer patchFile.Close()
		patchReader := patchFile

		baseFile, err := os.OpenFile(dn.basePath[0], os.O_RDONLY, 0)
		if err != nil {
			log.Errorf("Open failed(base=%s) err=%v", dn.basePath, err)
			return 0, 0, syscall.ENOENT
		}
		defer baseFile.Close()

		writer := new(bytes.Buffer)
		err = bsdiff.Patch(baseFile, writer, patchReader)
		if err != nil {
			log.Errorf("Open failed(bsdiff) err=%v", err)
			return 0, 0, syscall.ENOENT
		}
		dn.data = writer.Bytes()
		log.Debugf("Successfully patched %s", dn.meta.Name)
	}
	return nil, fuse.FOPEN_KEEP_CACHE | fuse.FOPEN_CACHE_DIR, 0
}

func (dn *Di3fsNode) Read(ctx context.Context, f fs.FileHandle, data []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	log.Traceln("Read started")
	defer log.Traceln("Read finished")

	end := int64(off) + int64(len(data))
	log.Debugf("READ STARTED file=%s offset=%d len=%d", dn.meta.Name, off, len(data))

	if dn.file != nil {
		readBuf := make([]byte, end-off)
		if dn.meta.IsNew() {
			log.Debugf("READ reading from %s", dn.patchPath)
		} else {
			log.Debugf("READ reading from %s", dn.basePath)
		}
		readSize, err := dn.file.ReadAt(readBuf, off)
		if err != nil {
			if err != io.EOF {
				log.Errorf("READ failed err=%v", err)
				return nil, syscall.EIO
			}
		}
		log.Debugf("READ FINISHED file=%s offset=%d len=%d", dn.meta.Name, off, readSize)
		return fuse.ReadResultData(readBuf[0:readSize]), 0
	} else {
		if end > int64(len(dn.data)) {
			end = int64(len(dn.data))
		}
		log.Debugf("READ FINISHED file=%s offset=%d len=%d", dn.meta.Name, off, (end - off))
		return fuse.ReadResultData(dn.data[off:end]), 0
	}

}

func (dn *Di3fsNode) Readlink(ctx context.Context) ([]byte, syscall.Errno) {
	log.Traceln("Readlink started")
	defer log.Traceln("Readlink finished")
	return []byte(dn.meta.RealPath), 0
}

func generateOpaqueFileEntry(dir *FileEntry) (*FileEntry, error) {
	fe := &FileEntry{
		Name:     ".wh..wh..opq",
		Size:     int(0),
		Mode:     uint32(dir.Mode),
		Type:     FILE_ENTRY_OPAQUE,
		RealPath: "",
		Childs:   map[string]*FileEntry{},
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

	isImage := dr.root.diffImage != nil
	if isImage && dr.root.IsBase() && !dr.meta.IsDir() && !dr.meta.IsSymlink() && !dr.meta.IsNew() {
		log.Fatalf("invalid base image %q", dr.patchPath)
	}
	// here, rootNode is initialized
	log.Debugf("base=%s patch=%s", dr.basePath, dr.patchPath)
	if dr.meta.IsDir() {
		for _, o := range dr.meta.OaqueFiles {
			if strings.Contains(o, "/") {
				continue
			}
			fe, err := generateOpaqueFileEntry(dr.meta)
			if err != nil {
				log.Fatalf("failed to generate opaqueFileEntry: %v", err)
			}
			n := newNode(dr.basePath, dr.patchPath, fe, nil, dr.root)
			stableAttr := fs.StableAttr{}
			cNode := dr.NewPersistentInode(ctx, n, stableAttr)
			dr.AddChild(n.meta.Name, cNode, false)
		}
	}
	for childfName := range dr.meta.Childs {
		c := dr.meta.Childs[childfName]
		var childBaseFEs []*FileEntry = make([]*FileEntry, 0)
		for baseMetaIdx := range dr.baseMeta {
			baseChild := dr.baseMeta[baseMetaIdx].Childs[childfName]
			if baseChild != nil {
				childBaseFEs = append(childBaseFEs, baseChild)
			}
		}
		n := newNode(dr.basePath, dr.patchPath, c, childBaseFEs, dr.root)
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
	baseImage           []*os.File
	baseImageBodyOffset []int64
	diffImage           *os.File
	diffImageBodyOffset int64
	RootNode            *Di3fsNode
}

func (dr *Di3fsRoot) IsBase() bool {
	return dr.baseImage == nil
}

func newNode(diffBaseDirPath []string, patchDirPath string, fe *FileEntry, baseFE []*FileEntry, root *Di3fsRoot) *Di3fsNode {
	node := &Di3fsNode{
		meta:     fe,
		baseMeta: baseFE,
		root:     root,
		basePath: []string{},
	}
	name := node.meta.Name
	if fe.IsDir() || fe.IsNew() {
		node.patchPath = path.Join(patchDirPath, name)
	} else {
		node.patchPath = path.Join(patchDirPath, name+".diff")
	}

	for i := range diffBaseDirPath {
		node.basePath = append(node.basePath, path.Join(diffBaseDirPath[i], name))
	}
	return node
}

func NewDi3fsRoot(opts *fs.Options, diffBase []string, patchBase string, fileEntry *FileEntry, baseFileEntry []*FileEntry, baseImage []*os.File, baseImageBodyOffset []int64, diffImage *os.File, diffImageBodyOffset int64) (Di3fsRoot, error) {
	rootNode := newNode(diffBase, patchBase, fileEntry, baseFileEntry, nil)
	root := Di3fsRoot{
		baseImage:           baseImage,
		baseImageBodyOffset: baseImageBodyOffset,
		diffImage:           diffImage,
		diffImageBodyOffset: diffImageBodyOffset,
		RootNode:            rootNode,
	}
	rootNode.root = &root

	return root, nil
}

func Do(diffImagePath, mountPath string) error {
	start := time.Now()
	customFormatter := new(log.TextFormatter)
	customFormatter.TimestampFormat = "2006-01-02 15:04:05"
	customFormatter.FullTimestamp = true
	log.SetFormatter(customFormatter)
	log.SetLevel(log.InfoLevel)

	// Scans the arg list and sets up flags
	diffImagePathAbs, err := filepath.Abs(diffImagePath)
	if err != nil {
		panic(err)
	}

	imageBodyOffset := int64(0)
	var baseImageFiles []*os.File = make([]*os.File, 0)
	var baseImageBodyOffsets []int64 = make([]int64, 0)
	var baseMetaJsons []*FileEntry = make([]*FileEntry, 0)
	var baseImagePaths []string = make([]string, 0)

	imageHeader, imageFile, imageBodyOffset, err := LoadImage(diffImagePathAbs)
	if err != nil {
		panic(err)
	}

	baseImageId := imageHeader.BaseId
	fmt.Println(baseImageId)
	for baseImageId != "" {
		imageStore, _ := filepath.Split(diffImagePathAbs)
		baseImagePath := filepath.Join(imageStore, baseImageId+".dimg")
		baseImageHeader, baseImageFile, baseImageBodyOffset, err := LoadImage(baseImagePath)
		if err != nil {
			panic(err)
		}
		baseImageFiles = append(baseImageFiles, baseImageFile)
		baseImageBodyOffsets = append(baseImageBodyOffsets, baseImageBodyOffset)
		baseMetaJsons = append(baseMetaJsons, &baseImageHeader.FileEntry)
		baseImagePaths = append(baseImagePaths, baseImagePath)

		baseImageId = baseImageHeader.BaseId
		log.Infof("baseImage %s is loaded", baseImageId)
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

	di3fsRoot, err := NewDi3fsRoot(opts, baseImagePaths, diffImagePathAbs, &imageHeader.FileEntry, baseMetaJsons, baseImageFiles, baseImageBodyOffsets, imageFile, imageBodyOffset)
	if err != nil {
		log.Fatalf("creating Di3fsRoot failed: %v\n", err)
	}

	server, err := fs.Mount(mountPath, di3fsRoot.RootNode, opts)
	if err != nil {
		log.Fatalf("Mount fail: %v\n", err)
	}
	log.Infof("Mounted!")
	fmt.Printf("elapsed = %v\n", (time.Since(start).Milliseconds()))
	server.Wait()

	return nil
}
