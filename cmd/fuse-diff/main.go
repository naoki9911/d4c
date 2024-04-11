// Copyright 2016 the Go-FUSE Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This is main program driver for the loopback filesystem from
// github.com/hanwen/go-fuse/fs/, a filesystem that shunts operations
// to an underlying file system.
package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/naoki9911/fuse-diff-containerd/pkg/benchmark"
	"github.com/naoki9911/fuse-diff-containerd/pkg/di3fs"
	"github.com/naoki9911/fuse-diff-containerd/pkg/image"
	"github.com/naoki9911/fuse-diff-containerd/pkg/utils"
	log "github.com/sirupsen/logrus"
)

var (
	debug       = flag.Bool("debug", false, "print debugging messages.")
	other       = flag.Bool("allow-other", false, "mount with -o allowother.")
	bench       = flag.Bool("benchmark", false, "measure benchmark")
	parentDimg  = flag.String("parentDimg", "", "path to parent dimg")
	diffDimg    = flag.String("diffDimg", "", "path to diff dimg")
	parentCdimg = flag.String("parentCdimg", "", "path to parent cdimg")
	diffCdimg   = flag.String("diffCdimg", "", "path to diff cdimg")
	label       = flag.String("label", "", "label for benchmark")
	daemon      = flag.Bool("daemon", false, "run as daemon")
	child       = flag.Bool("child", false, "run as child process (internal use)")
)

const (
	CHILD_READY byte = 0
	CHILD_ERROR byte = 1
)

func main() {
	customFormatter := new(log.TextFormatter)
	customFormatter.TimestampFormat = "2006-01-02 15:04:05"
	customFormatter.FullTimestamp = true
	log.SetFormatter(customFormatter)
	log.SetLevel(log.InfoLevel)
	flag.Parse()
	if flag.NArg() < 1 {
		fmt.Printf("usage: %s MOUNTPOINT\n", path.Base(os.Args[0]))
		fmt.Printf("\noptions:\n")
		flag.PrintDefaults()
		os.Exit(2)
	}

	if !*daemon {
		// not daemon
		err := runMain(false, nil)
		if err != nil {
			log.Fatalf("error occured: %v", err)
		}
		return
	}

	if *child {
		pipe := os.NewFile(uintptr(3), "pipe")
		if pipe == nil {
			panic("fd 3 is not valid")
		}
		defer pipe.Close()
		err := runMain(true, pipe)
		if err != nil {
			log.Errorf("error occured: %v", err)

			// notify error to parent process via pipe
			errMsg := fmt.Sprintf("child error: %v", err)
			msgLen := 1 + 2 + len(errMsg)
			msgBytes := make([]byte, msgLen)
			msgBytes[0] = CHILD_ERROR
			binary.LittleEndian.PutUint16(msgBytes[1:], uint16(len(errMsg)))
			copy(msgBytes[3:], errMsg)
			_, err = pipe.Write(msgBytes)
			if err != nil {
				panic(err)
			}
		}
		return
	}

	// thanks to https://qiita.com/hironobu_s/items/77d99436457ef57889d6
	args := []string{"--child"}
	args = append(args, os.Args[1:]...)

	r, w, err := os.Pipe()
	if err != nil {
		panic(err)
	}

	cmd := exec.Command(os.Args[0], args...)
	cmd.ExtraFiles = []*os.File{w}
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if err = cmd.Start(); err != nil {
		panic(err)
	}

	// wait for the child process is ready
	readyBuf := make([]byte, 1)
	_, err = r.Read(readyBuf)
	if err != nil {
		panic(err)
	}
	if readyBuf[0] == CHILD_READY {
		log.Infof("successfully started child process")
		return
	} else {
		log.Errorf("child process encountered error")
		readyBuf = make([]byte, 2)
		cnt, err := r.Read(readyBuf)
		if err != nil {
			log.Fatalf("failed to read message length from pipe: %v", err)
		}
		if cnt != 2 {
			log.Fatalf("invalid msg length header %d", cnt)
		}
		msgLen := binary.LittleEndian.Uint16(readyBuf)
		log.Errorf("message length %d", msgLen)
		readyBuf = make([]byte, msgLen)
		cnt, err = r.Read(readyBuf)
		if err != nil {
			log.Fatalf("failed to read message from pipe: %v", err)
		}
		log.Fatalf("child error: %s", string(readyBuf[0:cnt]))
	}
}

func runMain(isChild bool, readyFd *os.File) error {
	if isChild {
		// this cause error in fs.Mount with 'waitid: no child processe'
		//signal.Ignore(syscall.SIGCHLD)
		syscall.Close(0)
		syscall.Close(1)
		syscall.Close(2)
		_, err := syscall.Setsid()
		if err != nil {
			return fmt.Errorf("failed to setsid(): %s", err)
		}
		syscall.Umask(022)
	}

	start := time.Now()

	var b *benchmark.Benchmark = nil
	var err error
	if *bench {
		b, err = benchmark.NewBenchmark("./benchmark.log")
		if err != nil {
			return err
		}
	}

	if *diffDimg == "" && *diffCdimg == "" {
		return fmt.Errorf("'--diffDimg' or '--diffCdimg' are not specified")
	}

	var diffImageFile *image.DimgFile = nil
	if *diffDimg != "" {
		diffImageFile, err = image.OpenDimgFile(*diffDimg)
		if err != nil {
			return fmt.Errorf("failed to open diff dimg %s: %v", *diffDimg, err)
		}
		defer diffImageFile.Close()
	} else {
		diffCdimgFile, err := image.OpenCdimgFile(*diffCdimg)
		if err != nil {
			return fmt.Errorf("failed to open diff cdimg %s: %v", *diffCdimg, err)
		}
		defer diffCdimgFile.Close()
		diffImageFile = diffCdimgFile.Dimg
	}

	parentNeeded := diffImageFile.DimgHeader().ParentId != ""
	if parentNeeded && *parentDimg == "" && *parentCdimg == "" {
		return fmt.Errorf("'--parentDimg' or '--parentCdimg' are not specified")
	}

	parentImages := []*image.DimgFile{}
	if parentNeeded {
		var parentImageFile *image.DimgFile
		if *parentDimg != "" {
			parentImageFile, err = image.OpenDimgFile(*parentDimg)
			if err != nil {
				return fmt.Errorf("failed to open parent dimg %s: %v", *parentDimg, err)
			}
			defer parentImageFile.Close()
		} else {
			parentCdimgFile, err := image.OpenCdimgFile(*parentCdimg)
			if err != nil {
				return fmt.Errorf("failed to open parent cdimg %s: %v", *parentCdimg, err)
			}
			parentImageFile = parentCdimgFile.Dimg
		}
		parentImages = append(parentImages, parentImageFile)
	}

	sec := time.Second
	opts := &fs.Options{
		// These options are to be compatible with libfuse defaults,
		// making benchmarking easier.
		AttrTimeout:  &sec,
		EntryTimeout: &sec,
	}
	if *debug {
		log.SetLevel(log.TraceLevel)
	}
	opts.Debug = *debug
	opts.AllowOther = *other
	if opts.AllowOther {
		// Make the kernel check file permissions for us
		opts.MountOptions.Options = append(opts.MountOptions.Options, "default_permissions")
	}
	// mount only with read-only
	opts.MountOptions.Options = append(opts.MountOptions.Options, "ro")
	// First column in "df -T": original dir
	opts.MountOptions.Options = append(opts.MountOptions.Options, "fsname=fuse-diff")
	// Second column in "df -T" will be shown as "fuse." + Name
	opts.MountOptions.Name = "fuse-diff"
	// Leave file permissions on "000" files as-is
	opts.NullPermissions = true

	di3fsRoot, err := di3fs.NewDi3fsRoot(opts, parentImages, diffImageFile)
	if err != nil {
		return fmt.Errorf("creating Di3fsRoot failed: %v", err)
	}

	server, err := fs.Mount(flag.Arg(0), di3fsRoot.RootNode, opts)
	if err != nil {
		return fmt.Errorf("mount fail: %v", err)
	}
	log.Infof("Mounted!")
	if *bench {
		elapsedMilli := time.Since(start).Milliseconds()
		metric := benchmark.Metric{
			TaskName:     "di3fs",
			ElapsedMilli: int(elapsedMilli),
			Labels: map[string]string{
				"parent": *parentDimg,
				"patch":  *diffDimg,
			},
		}
		metric.AddLabels(utils.ParseLabels([]string{*label}))
		err = b.AppendResult(metric)
		if err != nil {
			return fmt.Errorf("failed to append benchmark result: %v", err)
		}
	}
	if readyFd != nil {
		_, err = readyFd.Write([]byte{CHILD_READY})
		if err != nil {
			return fmt.Errorf("failed to write readyFd: %v", err)
		}
	}
	server.Wait()

	return nil
}
