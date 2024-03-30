package util

import (
	"os"

	"github.com/naoki9911/fuse-diff-containerd/pkg/bsdiffx"
	"github.com/urfave/cli/v2"
)

func Command() *cli.Command {
	cmd := cli.Command{
		Name: "util",
		Subcommands: []*cli.Command{
			diffCommand(),
			patchCommand(),
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
		},
		Action: func(c *cli.Context) error {
			oldFilePath := c.String("old")
			newFilePath := c.String("new")
			diffFilePath := c.String("diff")

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
			newFile, err := os.Open(newFilePath)
			if err != nil {
				return err
			}
			defer newFile.Close()
			newFileStat, err := newFile.Stat()
			if err != nil {
				return err
			}
			err = bsdiffx.Diff(oldFile, newFile, newFileStat.Size(), diffFile)
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

			diffFile, err := os.Open(diffFilePath)
			if err != nil {
				return err
			}
			defer diffFile.Close()

			oldFile, err := os.Open(oldFilePath)
			if err != nil {
				return err
			}
			defer oldFile.Close()
			newFile, err := os.Create(newFilePath)
			if err != nil {
				return err
			}
			defer newFile.Close()
			err = bsdiffx.Patch(oldFile, newFile, diffFile)
			if err != nil {
				return err
			}

			return nil
		},
	}

	return &cmd
}
