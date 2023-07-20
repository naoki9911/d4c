package main

import (
	"fmt"
	"os"

	"github.com/naoki9911/fuse-diff-containerd/cmd/ctr-cli/convert"
	"github.com/naoki9911/fuse-diff-containerd/cmd/ctr-cli/load"
	"github.com/naoki9911/fuse-diff-containerd/cmd/ctr-cli/pack"
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
		pack.Command(),
		load.Command(),
		pull.Command(),
	}

	return app
}
