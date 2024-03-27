package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"

	"github.com/icedream/go-bsdiff"
)

var gzipUnpackDiff = true

func generateFileHash(filePath string) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	h := sha256.New()
	_, err = io.Copy(h, file)
	if err != nil {
		return nil, err
	}

	return h.Sum(nil), nil
}

// return true if files are same.
func compareFile(fileAPath, fileBPath string) (bool, error) {
	fileAHash, err := generateFileHash(fileAPath)
	if err != nil {
		return false, err
	}
	fileBHash, err := generateFileHash(fileBPath)
	if err != nil {
		return false, err
	}

	for i := range fileAHash {
		if fileAHash[i] != fileBHash[i] {
			return false, nil
		}
	}

	return true, nil
}

func generateFileDiff(baseFilePath, newFilePath string) ([]byte, error) {
	baseFile, err := os.Open(baseFilePath)
	if err != nil {
		return nil, err
	}
	defer baseFile.Close()
	newFile, err := os.Open(newFilePath)
	if err != nil {
		return nil, err
	}
	defer newFile.Close()
	writer := new(bytes.Buffer)
	err = bsdiff.Diff(baseFile, newFile, writer)
	if err != nil {
		return nil, err
	}

	return writer.Bytes(), nil
}

func generateFileDiffFromGzip(baseFilePath, newFilePath string) ([]byte, error) {
	baseFile, err := os.Open(baseFilePath)
	if err != nil {
		return nil, err
	}
	defer baseFile.Close()
	gzipBaseReader, err := gzip.NewReader(baseFile)
	if err != nil {
		return nil, err
	}
	newFile, err := os.Open(newFilePath)
	if err != nil {
		return nil, err
	}
	defer newFile.Close()
	gzipNewReader, err := gzip.NewReader(newFile)
	if err != nil {
		return nil, err
	}
	writer := new(bytes.Buffer)
	err = bsdiff.Diff(gzipBaseReader, gzipNewReader, writer)
	if err != nil {
		return nil, err
	}

	return writer.Bytes(), nil
}

func compressWithGzipFromFile(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	return compressWithGzip(fileBytes)
}

func compressWithGzip(src []byte) ([]byte, error) {
	writer := new(bytes.Buffer)
	gWriter := gzip.NewWriter(writer)
	_, err := gWriter.Write(src)
	if err != nil {
		return nil, err
	}
	err = gWriter.Close()
	if err != nil {
		return nil, err
	}

	return writer.Bytes(), nil
}

type DiffStats = struct {
	numDir         int
	numFile        int
	numSymlink     int
	numNewDir      int
	numNewFile     int
	numNewSymlink  int
	numSameDir     int
	numSameFile    int
	numSameSymlink int
	numDiffDir     int
	numDiffFile    int
	numDiffSymlink int
	diffFileStats  []DiffFileStats
}

type DiffFileStats = struct {
	path            string
	newFileSize     int64
	diffSize        int
	newFileGzipSize int
	diffGzipSize    int
}

func accumulateStats(baseDir, newDir string, stats *DiffStats) error {
	baseFiles := map[string]fs.DirEntry{}
	if _, err := os.Stat(baseDir); err == nil {
		baseEntries, err := os.ReadDir(baseDir)
		if err != nil {
			return err
		}
		for i := range baseEntries {
			baseFiles[baseEntries[i].Name()] = baseEntries[i]
		}
	}
	newEntries, err := os.ReadDir(newDir)
	if err != nil {
		return err
	}

	for _, entry := range newEntries {
		fName := entry.Name()
		baseFilePath := path.Join(baseDir, fName)
		newFilePath := path.Join(newDir, fName)
		fileInfo, err := entry.Info()
		if err != nil {
			return err
		}

		isDir := entry.IsDir()
		isSymlink := fileInfo.Mode()&os.ModeSymlink == os.ModeSymlink
		if isDir {
			stats.numDir += 1
		} else if isSymlink {
			stats.numSymlink += 1
		} else {
			stats.numFile += 1
		}

		baseEntry, ok := baseFiles[fName]
		if !ok {
			if isDir {
				stats.numNewDir += 1
			} else if isSymlink {
				stats.numNewSymlink += 1
			} else {
				stats.numNewFile += 1
			}
		} else {
			baseFI, err := baseEntry.Info()
			isBaseDir := baseEntry.IsDir()
			isBaseSymlink := baseFI.Mode()&os.ModeSymlink == os.ModeSymlink
			if err != nil {
				return err
			}
			if isDir {
				if isBaseDir {
					stats.numSameDir += 1
				} else {
					stats.numDiffDir += 1
				}
			} else if isSymlink {
				if isBaseSymlink {
					baseRealPath, err := os.Readlink(baseFilePath)
					if err != nil {
						return err
					}

					newRealPath, err := os.Readlink(newFilePath)
					if err != nil {
						return err
					}

					if baseRealPath == newRealPath {
						stats.numSameSymlink += 1
					} else {
						stats.numDiffSymlink += 1
					}
				} else {
					stats.numDiffSymlink += 1
				}
			} else {
				if !isBaseDir && !isBaseSymlink {
					isSame, err := compareFile(baseFilePath, newFilePath)
					if err != nil {
						return err
					}
					if isSame {
						stats.numSameFile += 1
					} else {
						stats.numDiffFile += 1
						var diffBytes []byte
						if strings.Contains(baseFilePath, ".gz") && gzipUnpackDiff {
							diffBytes, err = generateFileDiffFromGzip(baseFilePath, newFilePath)
							if err != nil {
								return err
							}
						} else {
							diffBytes, err = generateFileDiff(baseFilePath, newFilePath)
							if err != nil {
								return err
							}
						}
						gzipNewFile, err := compressWithGzipFromFile(newFilePath)
						if err != nil {
							return err
						}
						gzipDiffBytes, err := compressWithGzip(diffBytes)
						if err != nil {
							return err
						}
						stats.diffFileStats = append(stats.diffFileStats,
							DiffFileStats{
								path:            newFilePath,
								newFileSize:     fileInfo.Size(),
								diffSize:        len(diffBytes),
								newFileGzipSize: len(gzipNewFile),
								diffGzipSize:    len(gzipDiffBytes),
							})
					}
				} else {
					stats.numDiffFile += 1
				}
			}
		}
		if isDir {
			err = accumulateStats(baseFilePath, newFilePath, stats)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("diff base-dir new-dir")
		os.Exit(1)
	}
	baseDir := os.Args[1]
	newDir := os.Args[2]
	stats := &DiffStats{
		diffFileStats: make([]DiffFileStats, 0),
	}
	err := accumulateStats(baseDir, newDir, stats)
	if err != nil {
		panic(err)
	}

	fmt.Printf("numDir: %d\n", stats.numDir)
	fmt.Printf("numFile: %d\n", stats.numFile)
	fmt.Printf("numSynlink: %d\n", stats.numSymlink)
	fmt.Printf("numNewDir: %d\n", stats.numNewDir)
	fmt.Printf("numNewFile: %d\n", stats.numNewFile)
	fmt.Printf("numNewSymlink: %d\n", stats.numNewSymlink)
	fmt.Printf("numSameDir: %d\n", stats.numSameDir)
	fmt.Printf("numSameFile: %d\n", stats.numSameFile)
	fmt.Printf("numSameSymlink: %d\n", stats.numSameSymlink)
	fmt.Printf("numDiffDir: %d\n", stats.numDiffDir)
	fmt.Printf("numDiffFile: %d\n", stats.numDiffFile)
	fmt.Printf("numDiffSymlink: %d\n", stats.numDiffSymlink)
	fmt.Printf("path, newFileSize(bytes), diffSize(bytes), newFileGzipSize(bytes), diffGzipSize(bytes)\n")
	for _, diff := range stats.diffFileStats {
		fmt.Printf("\"%s\",%d,%d,%d,%d\n", diff.path, diff.newFileSize, diff.diffSize, diff.newFileGzipSize, diff.diffGzipSize)
	}
}
