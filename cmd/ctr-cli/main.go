package main

import (
	"fmt"
	"os"

	"github.com/naoki9911/fuse-diff-containerd/cmd/ctr-cli/convert"
	"github.com/naoki9911/fuse-diff-containerd/cmd/ctr-cli/diff"
	"github.com/naoki9911/fuse-diff-containerd/cmd/ctr-cli/load"
	"github.com/naoki9911/fuse-diff-containerd/cmd/ctr-cli/merge"
	"github.com/naoki9911/fuse-diff-containerd/cmd/ctr-cli/pack"
	"github.com/naoki9911/fuse-diff-containerd/cmd/ctr-cli/patch"
	"github.com/naoki9911/fuse-diff-containerd/cmd/ctr-cli/pull"
	"github.com/urfave/cli/v2"
)

func main() {
	app := NewApp()
	err := app.Run(os.Args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ctr-cli: %v\n", err)
		os.Exit(1)
	}
}

func NewApp() *cli.App {
	app := cli.NewApp()

	app.Name = "ctr-cli"
	app.Version = "0.0.0"
	app.Usage = "CLI tool for di3fs-containerd"
	app.EnableBashCompletion = true
	app.Flags = []cli.Flag{}
	app.Commands = []*cli.Command{
		convert.Command(),
		dimgCommand(),
		cdimgCommand(),
		pack.Command(),
		load.Command(),
		pull.Command(),
		merge.Command(),
		diff.Command(),
	}

	return app
}

func dimgCommand() *cli.Command {
	cmd := cli.Command{
		Name:  "dimg",
		Usage: "Delta image (dimg) related commands",
		Flags: []cli.Flag{},
		Subcommands: []*cli.Command{
			patch.DimgCommand(),
		},
	}
	return &cmd
}

func cdimgCommand() *cli.Command {
	cmd := cli.Command{
		Name:  "cdimg",
		Usage: "Container delta image (cdimg) related commands",
		Flags: []cli.Flag{},
		Subcommands: []*cli.Command{
			patch.CdimgCommand(),
		},
	}
	return &cmd
}
