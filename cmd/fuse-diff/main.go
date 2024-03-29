// Copyright 2016 the Go-FUSE Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This is main program driver for the loopback filesystem from
// github.com/hanwen/go-fuse/fs/, a filesystem that shunts operations
// to an underlying file system.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path"
	"runtime/pprof"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/naoki9911/fuse-diff-containerd/pkg/benchmark"
	"github.com/naoki9911/fuse-diff-containerd/pkg/di3fs"
	"github.com/naoki9911/fuse-diff-containerd/pkg/image"
	log "github.com/sirupsen/logrus"
)

func writeMemProfile(fn string, sigs <-chan os.Signal) {
	i := 0
	for range sigs {
		fn := fmt.Sprintf("%s-%d.memprof", fn, i)
		i++

		log.Printf("Writing mem profile to %s\n", fn)
		f, err := os.Create(fn)
		if err != nil {
			log.Printf("Create: %v", err)
			continue
		}
		err = pprof.WriteHeapProfile(f)
		if err != nil {
			log.Printf("failed WriteHeapProfile: %v", err)
		}
		if err := f.Close(); err != nil {
			log.Printf("close %v", err)
		}
	}
}

func main() {
	start := time.Now()
	customFormatter := new(log.TextFormatter)
	customFormatter.TimestampFormat = "2006-01-02 15:04:05"
	customFormatter.FullTimestamp = true
	log.SetFormatter(customFormatter)
	log.SetLevel(log.InfoLevel)

	// Scans the arg list and sets up flags
	debug := flag.Bool("debug", false, "print debugging messages.")
	other := flag.Bool("allow-other", false, "mount with -o allowother.")
	bench := flag.Bool("benchmark", false, "measure benchmark")
	cpuprofile := flag.String("cpuprofile", "", "write cpu profile to this file")
	memprofile := flag.String("memprofile", "", "write memory profile to this file")
	parentDimg := flag.String("parentDimg", "", "path to parent dimg")
	diffDimg := flag.String("diffDimg", "", "path to diff dimg")
	parentCdimg := flag.String("parentCdimg", "", "path to parent cdimg")
	diffCdimg := flag.String("diffCdimg", "", "path to diff cdimg")
	flag.Parse()
	if flag.NArg() < 1 {
		fmt.Printf("usage: %s MOUNTPOINT\n", path.Base(os.Args[0]))
		fmt.Printf("\noptions:\n")
		flag.PrintDefaults()
		os.Exit(2)
	}

	var b *benchmark.Benchmark = nil
	var err error
	if *bench {
		b, err = benchmark.NewBenchmark("./benchmark.log")
		if err != nil {
			panic(err)
		}
	}
	if *cpuprofile != "" {
		fmt.Printf("Writing cpu profile to %s\n", *cpuprofile)
		f, err := os.Create(*cpuprofile)
		if err != nil {
			fmt.Println(err)
			os.Exit(3)
		}
		err = pprof.StartCPUProfile(f)
		if err != nil {
			log.Fatalf("failed to start CPUProfile: %v", err)
		}
		defer pprof.StopCPUProfile()
	}
	if *memprofile != "" {
		log.Printf("send SIGUSR1 to %d to dump memory profile", os.Getpid())
		profSig := make(chan os.Signal, 1)
		signal.Notify(profSig, syscall.SIGUSR1)
		go writeMemProfile(*memprofile, profSig)
	}
	if *cpuprofile != "" || *memprofile != "" {
		fmt.Printf("Note: You must unmount gracefully, otherwise the profile file(s) will stay empty!\n")
	}

	if *diffDimg == "" && *diffCdimg == "" {
		fmt.Println("please specify '--diffDimg' or '--diffCdimg'")
		os.Exit(1)
	}

	var diffImageFile *image.DimgFile = nil
	if *diffDimg != "" {
		diffImageFile, err = image.OpenDimgFile(*diffDimg)
		if err != nil {
			panic(err)
		}
		defer diffImageFile.Close()
	} else {
		diffCdimgFile, err := image.OpenCdimgFile(*diffCdimg)
		if err != nil {
			panic(err)
		}
		defer diffCdimgFile.Close()
		diffImageFile = diffCdimgFile.Dimg
	}

	parentNeeded := diffImageFile.Header().ParentId != ""
	if parentNeeded && *parentDimg == "" && *parentCdimg == "" {
		fmt.Println("please specify '--parentDimg' or '--parentCdimg'")
		os.Exit(1)
	}

	var parentImageFile *image.DimgFile = nil
	if *parentDimg != "" {
		parentImageFile, err = image.OpenDimgFile(*parentDimg)
		if err != nil {
			panic(err)
		}
		defer parentImageFile.Close()
	} else {
		parentCdimgFile, err := image.OpenCdimgFile(*parentCdimg)
		if err != nil {
			panic(err)
		}
		parentImageFile = parentCdimgFile.Dimg
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

	di3fsRoot, err := di3fs.NewDi3fsRoot(opts, []*image.DimgFile{parentImageFile}, diffImageFile)
	if err != nil {
		log.Fatalf("creating Di3fsRoot failed: %v\n", err)
	}

	server, err := fs.Mount(flag.Arg(0), di3fsRoot.RootNode, opts)
	if err != nil {
		log.Fatalf("Mount fail: %v\n", err)
	}
	log.Infof("Mounted!")
	fmt.Printf("elapsed = %v\n", (time.Since(start).Milliseconds()))
	if *bench {
		elapsedMilli := time.Since(start).Milliseconds()
		metric := benchmark.Metric{
			TaskName:     "di3fs",
			ElapsedMilli: int(elapsedMilli),
			Labels: []string{
				"parent:" + *parentDimg,
				"patch:" + *diffDimg,
			},
		}
		err = b.AppendResult(metric)
		if err != nil {
			panic(err)
		}
	}
	server.Wait()
}
