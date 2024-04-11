package stat

import (
	"encoding/json"
	"fmt"

	"github.com/naoki9911/fuse-diff-containerd/pkg/image"
	"github.com/naoki9911/fuse-diff-containerd/pkg/utils"
	"github.com/urfave/cli/v2"
)

func Command() *cli.Command {
	cmd := cli.Command{
		Name: "stat",
		Subcommands: []*cli.Command{
			compareCommand(),
		},
	}

	return &cmd
}

func compareCommand() *cli.Command {
	cmd := cli.Command{
		Name:   "compare",
		Usage:  "compare dimg(or cdimg)",
		Action: compareAction,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "fileDimg",
				Usage:    "path to file diff based dimg",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "binaryDimg",
				Usage:    "path to binary diff based dimg",
				Required: true,
			},
		},
	}

	return &cmd
}

func compareAction(c *cli.Context) error {
	fileImg, err := image.OpenDimgOrCdimg(c.String("fileDimg"))
	if err != nil {
		return err
	}
	defer fileImg.Close()

	binaryImg, err := image.OpenDimgOrCdimg(c.String("binaryDimg"))
	if err != nil {
		return err
	}
	defer binaryImg.Close()

	results, err := image.CompareFileEntries(&fileImg.DimgHeader().FileEntry, &binaryImg.DimgHeader().FileEntry, "")
	if err != nil {
		return err
	}

	labels := utils.ParseLabels(c.StringSlice("labels"))

	for _, r := range results {
		r.Labels = labels
		jsonBytes, err := json.Marshal(r)
		if err != nil {
			return fmt.Errorf("failed to marshal: %v", err)
		}
		fmt.Println(string(jsonBytes))
	}

	return nil
}
