package di3fs

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"

	"github.com/klauspost/compress/zstd"
)

type EntryType int

const (
	FILE_ENTRY_FILE_NEW EntryType = iota
	FILE_ENTRY_FILE_SAME
	FILE_ENTRY_FILE_DIFF
	FILE_ENTRY_DIR
	FILE_ENTRY_DIR_NEW
	FILE_ENTRY_SYMLINK
	FILE_ENTRY_OPAQUE
)

func UnmarshalJsonFromCompressed[T any](b []byte) (*T, error) {
	buf := bytes.NewBuffer(b)
	reader, err := zstd.NewReader(buf)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	jsonBytes, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	var res T
	err = json.Unmarshal(jsonBytes, &res)
	if err != nil {
		return nil, err
	}

	return &res, nil
}

type ImageHeader struct {
	BaseId    string    `json:"baseID"`
	FileEntry FileEntry `json:"fileEntry"`
}

type FileEntry struct {
	Name           string      `json:"name"`
	Size           int         `json:"size"`
	Mode           uint32      `json:"mode"`
	UID            uint32      `json:"uid"`
	GID            uint32      `json:"gid"`
	DiffName       string      `json:"diffName,omitempty"`
	Type           EntryType   `json:"type"`
	OaqueFiles     []string    `json:"opaqueFiles,omitempty"`
	UncompressedGz bool        `json:"uncompressedGz"`
	RealPath       string      `json:"realPath,omitempty"`
	Childs         []FileEntry `json:"childs" copier:"-"`
	CompressedSize int64       `json:"compressedSize,omitempty"`
	Offset         int64       `json:"offset,omitempty"`
}

func CompressWithZstd(src []byte) ([]byte, error) {
	out := &bytes.Buffer{}
	z, err := zstd.NewWriter(out)
	if err != nil {
		return nil, err
	}

	_, err = z.Write(src)
	if err != nil {
		return nil, err
	}

	err = z.Close()
	if err != nil {
		return nil, err
	}

	return out.Bytes(), nil
}

func compressFileWithZstd(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	fileBytes, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}

	res, err := CompressWithZstd(fileBytes)
	if err != nil {
		return nil, err
	}

	return res, nil

}

func PackFile(srcFilePath string, out io.Writer) (int64, error) {
	compressed, err := compressFileWithZstd(srcFilePath)
	if err != nil {
		return 0, err
	}
	writtenSize, err := out.Write(compressed)
	if err != nil {
		return 0, err
	}

	return int64(writtenSize), err
}

// int64: imageBodyOffset
func LoadImage(dimgPath string) (*ImageHeader, *os.File, int64, error) {
	imageFile, err := os.Open(dimgPath)
	if err != nil {
		return nil, nil, 0, err
	}

	curOffset := int64(0)
	bs := make([]byte, 4)
	_, err = imageFile.ReadAt(bs, curOffset)
	if err != nil {
		return nil, nil, 0, err
	}
	curOffset += 4

	compressedHeaderSize := binary.LittleEndian.Uint32(bs)
	compressedHeader := make([]byte, compressedHeaderSize)
	_, err = imageFile.ReadAt(compressedHeader, curOffset)
	if err != nil {
		return nil, nil, 0, err
	}
	curOffset += int64(compressedHeaderSize)
	imageHeader, err := UnmarshalJsonFromCompressed[ImageHeader](compressedHeader)
	if err != nil {
		return nil, nil, 0, err
	}

	return imageHeader, imageFile, curOffset, nil
}

func NewFileEntry() *FileEntry {
	return &FileEntry{
		Childs: make([]FileEntry, 0),
	}
}

func (fe FileEntry) Print(prefix string, isLast bool) {
	r := "├"
	if isLast {
		r = "└"
	}
	state := "updated"
	if fe.Type == FILE_ENTRY_FILE_NEW {
		state = "new"
	} else if fe.Type == FILE_ENTRY_FILE_SAME {
		state = "same"
	}
	fmt.Printf("%v %s %s (%s, size=%d)\n", prefix, r, fe.Name, state, fe.Size)
	if len(fe.Childs) > 0 {
		for i, c := range fe.Childs {
			c.Print(prefix+"  ", i == len(fe.Childs)-1)
		}
	}
}

func (fe FileEntry) IsDir() bool {
	return fe.Type == FILE_ENTRY_DIR || fe.Type == FILE_ENTRY_DIR_NEW
}

func (fe FileEntry) IsNew() bool {
	return fe.Type == FILE_ENTRY_FILE_NEW ||
		fe.Type == FILE_ENTRY_OPAQUE ||
		fe.Type == FILE_ENTRY_DIR_NEW
}

func (fe FileEntry) IsSame() bool {
	return fe.Type == FILE_ENTRY_FILE_SAME
}

func (fe FileEntry) IsSymlink() bool {
	return fe.Type == FILE_ENTRY_SYMLINK
}

func (fe *FileEntry) SetUGID(path string) error {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return err
	}
	stat, ok := fileInfo.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("this supports only linux")
	}
	fe.UID = stat.Uid
	fe.GID = stat.Gid

	return nil
}

func (fe *FileEntry) Lookup(path string) (*FileEntry, error) {
	paths := strings.Split(path, "/")
	if paths[0] == "" {
		paths = paths[1:]
	}
	return fe.lookupImpl(paths)
}

func (fe *FileEntry) lookupImpl(paths []string) (*FileEntry, error) {
	// it must be file
	if len(paths) == 0 {
		return fe, nil
	}

	for idx := range fe.Childs {
		child := fe.Childs[idx]
		if child.Name == paths[0] {
			return child.lookupImpl(paths[1:])
		}
	}

	return nil, fmt.Errorf("not found")
}
