package stat

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/naoki9911/fuse-diff-containerd/pkg/benchmark"
	"github.com/naoki9911/fuse-diff-containerd/pkg/bsdiffx"
	"github.com/naoki9911/fuse-diff-containerd/pkg/image"
	"github.com/naoki9911/fuse-diff-containerd/pkg/utils"
	"github.com/urfave/cli/v2"
)

func Command() *cli.Command {
	cmd := cli.Command{
		Name: "stat",
		Subcommands: []*cli.Command{
			compareCommand(),
			diffCommand(),
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

func diffCommand() *cli.Command {
	cmd := cli.Command{
		Name:      "diff",
		Usage:     "validate files or dirs",
		ArgsUsage: "<fileA or dirA> <fileB or dirB>",
		Action:    diffAction,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:     "benchmark",
				Usage:    "enable benchmark",
				Required: false,
			},
			&cli.StringFlag{
				Name:     "pathALabel",
				Required: false,
				Value:    "pathA",
			},
			&cli.StringFlag{
				Name:     "pathBLabel",
				Required: false,
				Value:    "pathB",
			},
			&cli.IntFlag{
				Name:     "count",
				Required: false,
				Value:    1,
			},
		},
	}

	return &cmd
}

func diffAction(c *cli.Context) error {
	if c.NArg() < 2 {
		return fmt.Errorf("invalid argurment")
	}

	pathA := c.Args().Get(0)
	pathB := c.Args().Get(1)

	pm, err := bsdiffx.LoadOrDefaultPlugins("")
	if err != nil {
		return err
	}

	var b *benchmark.Benchmark = nil
	if c.Bool("benchmark") {
		b, err = benchmark.NewBenchmark("./benchmark-io.log")
		if err != nil {
			return err
		}
		defer b.Close()
		b.SetDefaultLabels(utils.ParseLabels(c.StringSlice("labels")))
		fmt.Println("benchmarker enabled")
	}

	defer fmt.Println("done")
	for i := 0; i < c.Int("count"); i++ {
		b.SetLabel("count", strconv.Itoa(i))
		err = diffImpl(pathA, pathB, pm, b, c.String("pathALabel"), c.String("pathBLabel"), pathA, pathB)
		if err != nil {
			return err
		}
	}

	return nil
}

func doBoth[T, U any](pathA, pathB T, f func(path T) (U, error)) (U, U, error) {
	rA, err := f(pathA)
	if err != nil {
		return rA, rA, fmt.Errorf("on %v: %v", pathA, err)
	}
	rB, err := f(pathB)
	if err != nil {
		return rB, rB, fmt.Errorf("on %v: %v", pathB, err)
	}

	return rA, rB, nil
}

func doBothNoError[T, U any](pathA, pathB T, f func(path T) U) (U, U) {
	rA := f(pathA)
	rB := f(pathB)

	return rA, rB
}

func openAndReadAllFile(path string, b *benchmark.Benchmark, pathLabel, rootPath string) ([]byte, error) {
	start := time.Now()
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	openEnd := time.Now()
	bytes, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	readEnd := time.Now()

	if b != nil {
		stat, err := f.Stat()
		if err != nil {
			return nil, err
		}
		m := benchmark.Metric{
			TaskName:     "open",
			ElapsedMicro: openEnd.Sub(start).Microseconds(),
			Size:         stat.Size(),
			Labels: map[string]string{
				"path":      path,
				"pathLabel": pathLabel,
				"root":      rootPath,
			},
		}
		err = b.AppendResult(m)
		if err != nil {
			return nil, err
		}

		m.TaskName = "read"
		m.ElapsedMicro = readEnd.Sub(openEnd).Microseconds()
		err = b.AppendResult(m)
		if err != nil {
			return nil, err
		}

		m.TaskName = "open+read"
		m.ElapsedMicro = readEnd.Sub(start).Microseconds()
		err = b.AppendResult(m)
		if err != nil {
			return nil, err
		}
	}

	return bytes, nil
}

func diffImpl(pathA, pathB string, pm *bsdiffx.PluginManager, b *benchmark.Benchmark, pathALabel, pathBLabel, pathARoot, pathBRoot string) error {
	statA, statB, err := doBoth(pathA, pathB, os.Lstat)
	if err != nil {
		return fmt.Errorf("failed to stats: %v", err)
	}

	typeA, typeB := doBothNoError(statA, statB, func(s os.FileInfo) os.FileMode { return s.Mode().Type() })
	if typeA != typeB {
		return fmt.Errorf("unmatched type %s is %s but %s is %s", pathA, typeA, pathB, typeB)
	}

	if typeA == os.ModeSymlink {
		linkA, linkB, err := doBoth(pathA, pathB, os.Readlink)
		if err != nil {
			return fmt.Errorf("failed to readlink: %v", err)
		}
		if linkA != linkB {
			return fmt.Errorf("unmatched link %s is %s but %s is %s", pathA, linkA, pathB, linkB)
		}

		return nil
	}

	if typeA.IsRegular() {
		fileA, err := openAndReadAllFile(pathA, b, pathALabel, pathARoot)
		if err != nil {
			return fmt.Errorf("failed to open and readall %s: %v", pathA, err)
		}

		fileB, err := openAndReadAllFile(pathB, b, pathBLabel, pathBRoot)
		if err != nil {
			return fmt.Errorf("failed to open and readall %s: %v", pathB, err)
		}

		p := pm.GetPluginByExt(filepath.Ext(pathA))
		if !p.Compare(fileA, fileB) {
			return fmt.Errorf("unmatched content %s and %s", fileA, fileB)
		}

		return nil
	}

	if typeA.IsDir() {
		childs, err := os.ReadDir(pathA)
		if err != nil {
			return fmt.Errorf("failed to readdir %s: %v", pathA, err)
		}
		for _, c := range childs {
			err := diffImpl(filepath.Join(pathA, c.Name()), filepath.Join(pathB, c.Name()), pm, b, pathALabel, pathBLabel, pathARoot, pathBRoot)
			if err != nil {
				return err
			}
		}

		return nil
	}

	// ignore others
	return nil
}
