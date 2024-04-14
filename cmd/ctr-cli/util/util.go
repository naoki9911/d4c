package util

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/naoki9911/fuse-diff-containerd/pkg/bsdiffx"
	"github.com/naoki9911/fuse-diff-containerd/pkg/image"
	"github.com/urfave/cli/v2"
)

func Command() *cli.Command {
	cmd := cli.Command{
		Name: "util",
		Subcommands: []*cli.Command{
			diffCommand(),
			patchCommand(),
			mergeCommand(),
			getDiffCommand(),
		},
	}

	return &cmd
}

func diffCommand() *cli.Command {
	cmd := cli.Command{
		Name:  "diff",
		Usage: "generate diff with bsdiffx",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "old",
				Usage:    "old file path",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "new",
				Usage:    "new file path",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "diff",
				Usage:    "diff file path",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "compressionMode",
				Usage:    "Mode to compress diffs",
				Value:    "bzip2",
				Required: false,
			},
		},
		Action: func(c *cli.Context) error {
			oldFilePath := c.String("old")
			newFilePath := c.String("new")
			diffFilePath := c.String("diff")

			pm, err := bsdiffx.LoadOrDefaultPlugins("")
			if err != nil {
				return err
			}
			p := pm.GetPluginByExt(filepath.Ext(newFilePath))
			compMode, err := bsdiffx.GetCompressMode(c.String("compressionMode"))
			if err != nil {
				return err
			}

			diffFile, err := os.Create(diffFilePath)
			if err != nil {
				return err
			}
			defer diffFile.Close()

			oldFile, err := os.Open(oldFilePath)
			if err != nil {
				return err
			}
			defer oldFile.Close()
			oldBytes, err := io.ReadAll(oldFile)
			if err != nil {
				return err
			}
			newFile, err := os.Open(newFilePath)
			if err != nil {
				return err
			}
			newBytes, err := io.ReadAll(newFile)
			if err != nil {
				return err
			}
			defer newFile.Close()
			err = p.Diff(oldBytes, newBytes, diffFile, compMode)
			if err != nil {
				return err
			}

			return nil
		},
	}

	return &cmd
}

func patchCommand() *cli.Command {
	cmd := cli.Command{
		Name:  "patch",
		Usage: "generate patch with bsdiffx",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "old",
				Usage:    "old file path",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "new",
				Usage:    "new file path",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "diff",
				Usage:    "diff file path",
				Required: true,
			},
		},
		Action: func(c *cli.Context) error {
			oldFilePath := c.String("old")
			newFilePath := c.String("new")
			diffFilePath := c.String("diff")

			pm, err := bsdiffx.LoadOrDefaultPlugins("")
			if err != nil {
				return err
			}
			p := pm.GetPluginByExt(filepath.Ext(newFilePath))

			diffFile, err := os.Open(diffFilePath)
			if err != nil {
				return err
			}
			defer diffFile.Close()

			err = image.ApplyFilePatch(oldFilePath, newFilePath, diffFile, p)
			if err != nil {
				return err
			}

			return nil
		},
	}

	return &cmd
}

func mergeCommand() *cli.Command {
	cmd := cli.Command{
		Name:  "merge",
		Usage: "generate merge with bsdiffx",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "lower",
				Usage:    "lower file path",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "upper",
				Usage:    "upper file path",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "out",
				Usage:    "out file path",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "base",
				Usage:    "base file path (for debug)",
				Required: false,
			},
			&cli.StringFlag{
				Name:     "updated",
				Usage:    "updated file path (for debug)",
				Required: false,
			},
		},
		Action: func(c *cli.Context) error {
			lower := c.String("lower")
			upper := c.String("upper")
			out := c.String("out")
			base := c.String("base")
			updated := c.String("updated")

			pm, err := bsdiffx.LoadOrDefaultPlugins("")
			if err != nil {
				return err
			}
			p := pm.GetPluginByExt(filepath.Ext(upper))

			lowerFile, err := os.Open(lower)
			if err != nil {
				return fmt.Errorf("failed to open lower %s: %v", lower, err)
			}
			defer lowerFile.Close()

			upperFile, err := os.Open(upper)
			if err != nil {
				return fmt.Errorf("failed to open upper %s: %v", upper, err)
			}
			defer upperFile.Close()

			outFile, err := os.Create(out)
			if err != nil {
				return fmt.Errorf("failed to creat out %s: %v", out, err)
			}

			if base != "" && updated != "" {
				baseFile, err := os.Open(base)
				if err != nil {
					return fmt.Errorf("failed to open base %s: %v", base, err)
				}
				defer baseFile.Close()
				updatedFile, err := os.Open(updated)
				if err != nil {
					return fmt.Errorf("failed to open updated %s: %v", updated, err)
				}
				defer updatedFile.Close()

				err = bsdiffx.DeltaMergingBytesDebug(lowerFile, upperFile, outFile, baseFile, updatedFile)
				if err != nil {
					return fmt.Errorf("failed to DeltaMerging: %v", err)
				}
			} else {
				err = p.Merge(lowerFile, upperFile, outFile)
				if err != nil {
					return fmt.Errorf("failed to DeltaMerging: %v", err)
				}
			}
			return nil
		},
	}

	return &cmd
}

func getDiffCommand() *cli.Command {
	cmd := cli.Command{
		Name:  "getDiff",
		Usage: "get diff from Dimg or Cdimg",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "cdimg",
				Usage:    "path to cdimg",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "path",
				Usage:    "path to the file",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "out",
				Usage:    "path to output",
				Required: true,
			},
		},
		Action: func(c *cli.Context) error {
			cdimg := c.String("cdimg")
			path := c.String("path")
			out := c.String("out")

			cdimgFile, err := image.OpenCdimgFile(cdimg)
			if err != nil {
				return fmt.Errorf("failed to open cdimg %s: %v", cdimg, err)
			}
			defer cdimgFile.Close()

			dimg := cdimgFile.Dimg
			targetFE, err := dimg.DimgHeader().FileEntry.Lookup(path)
			if err != nil {
				return fmt.Errorf("failed to lookup %s: %v", path, err)
			}

			outFile, err := os.Create(out)
			if err != nil {
				return fmt.Errorf("failed to create out file %s: %v", out, err)
			}
			defer outFile.Close()

			diffBytes := make([]byte, targetFE.CompressedSize)
			_, err = dimg.ReadAt(diffBytes, targetFE.Offset)
			if err != nil {
				return fmt.Errorf("failed to ReadAt 0x%x (%d)", targetFE.Offset, targetFE.Size)
			}
			_, err = outFile.Write(diffBytes)
			if err != nil {
				return fmt.Errorf("failed to write out file: %v", err)
			}
			return nil
		},
	}

	return &cmd
}
