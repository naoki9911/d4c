package image

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"syscall"

	"github.com/google/uuid"
	"github.com/klauspost/compress/zstd"
	"github.com/opencontainers/go-digest"
)

type EntryType int

const (
	FILE_ENTRY_FILE_NEW EntryType = iota
	FILE_ENTRY_FILE_SAME
	FILE_ENTRY_FILE_DIFF
	FILE_ENTRY_DIR
	FILE_ENTRY_DIR_NEW
	FILE_ENTRY_SYMLINK
	FILE_ENTRY_HARDLINK
)

func EntryTypeToString(e EntryType) string {
	switch e {
	case FILE_ENTRY_FILE_NEW:
		return "file_new"
	case FILE_ENTRY_FILE_SAME:
		return "file_same"
	case FILE_ENTRY_FILE_DIFF:
		return "file_diff"
	case FILE_ENTRY_DIR:
		return "dir"
	case FILE_ENTRY_DIR_NEW:
		return "dir_new"
	case FILE_ENTRY_SYMLINK:
		return "symlink"
	case FILE_ENTRY_HARDLINK:
		return "hardlink"
	default:
		panic(fmt.Errorf("unexpected EntryType: %v", e))
	}
}

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

type FileEntry struct {
	Name     string                `json:"name"`
	Size     int                   `json:"size"`
	Mode     uint32                `json:"mode"`
	UID      uint32                `json:"uid"`
	GID      uint32                `json:"gid"`
	RealPath string                `json:"realPath,omitempty"`
	Childs   map[string]*FileEntry `json:"childs"`

	Type           EntryType     `json:"type"`
	CompressedSize int64         `json:"compressedSize,omitempty"`
	Offset         int64         `json:"offset,omitempty"`
	Digest         digest.Digest `json:"digest"`
	PluginUuid     uuid.UUID     `json:"pluginUuid"`
}

func (fe *FileEntry) DeepCopy() *FileEntry {
	feJson, err := json.Marshal(fe)
	if err != nil {
		panic(err)
	}
	res := FileEntry{}
	err = json.Unmarshal(feJson, &res)
	if err != nil {
		panic(err)
	}

	return &res
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

func NewFileEntry() *FileEntry {
	return &FileEntry{
		Childs: map[string]*FileEntry{},
	}
}

func (fe FileEntry) IsDir() bool {
	return fe.Type == FILE_ENTRY_DIR || fe.Type == FILE_ENTRY_DIR_NEW
}

// IsNew() represents this file entry does not depend on any other images
func (fe FileEntry) IsNew() bool {
	return fe.Type == FILE_ENTRY_FILE_NEW ||
		fe.Type == FILE_ENTRY_DIR_NEW ||
		fe.Type == FILE_ENTRY_SYMLINK ||
		fe.Type == FILE_ENTRY_HARDLINK
}

func (fe FileEntry) IsSame() bool {
	return fe.Type == FILE_ENTRY_FILE_SAME
}

func (fe FileEntry) IsLink() bool {
	return fe.Type == FILE_ENTRY_SYMLINK
}
func (fe FileEntry) IsFile() bool {
	return fe.Type == FILE_ENTRY_FILE_DIFF ||
		fe.Type == FILE_ENTRY_FILE_NEW ||
		fe.Type == FILE_ENTRY_FILE_SAME
}

func (fe FileEntry) IsBaseRequired() bool {
	return fe.Type == FILE_ENTRY_FILE_DIFF ||
		fe.Type == FILE_ENTRY_FILE_SAME
}

func (fe FileEntry) HasBody() bool {
	return fe.Type == FILE_ENTRY_FILE_NEW ||
		fe.Type == FILE_ENTRY_FILE_DIFF
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

	child, ok := fe.Childs[paths[0]]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return child.lookupImpl(paths[1:])
}

type feForDigest struct {
	Name     string          `json:"name"`
	Size     int             `json:"size"`
	Mode     uint32          `json:"mode"`
	UID      uint32          `json:"uid"`
	GID      uint32          `json:"gid"`
	RealPath string          `json:"realPath,omitempty"`
	Childs   []digest.Digest `json:"childs"`
}

func (fe *FileEntry) feForDigest() (*feForDigest, error) {
	res := &feForDigest{
		Name:     fe.Name,
		Size:     fe.Size,
		Mode:     fe.Mode,
		UID:      fe.UID,
		GID:      fe.GID,
		RealPath: fe.RealPath,
		Childs:   []digest.Digest{},
	}

	if fe.IsDir() {
		childNames := []string{}
		for name := range fe.Childs {
			childNames = append(childNames, name)
		}
		slices.Sort(childNames)

		for _, name := range childNames {
			c := fe.Childs[name]
			if c.Digest == "" {
				return nil, fmt.Errorf("child %s does not have digest", name)
			}
			res.Childs = append(res.Childs, c.Digest)
		}
	}

	return res, nil
}

func (fe *FileEntry) GenerateDigest(body []byte) (digest.Digest, error) {
	fed, err := fe.feForDigest()
	if err != nil {
		return "", nil
	}
	feBytes, err := json.Marshal(fed)
	if err != nil {
		return "", nil
	}

	if fe.IsFile() {
		feBytes = append(feBytes, body...)
	}
	d := digest.FromBytes(feBytes)
	return d, nil
}

func (fe *FileEntry) Verify(body []byte) error {
	d, err := fe.GenerateDigest(body)
	if err != nil {
		return nil
	}

	if d != fe.Digest {
		return fmt.Errorf("failed to verify digest")
	}

	return nil
}
