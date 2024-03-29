package main

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/log"
	"github.com/naoki9911/fuse-diff-containerd/pkg/di3fs"
	"github.com/naoki9911/fuse-diff-containerd/pkg/image"
	sns "github.com/naoki9911/fuse-diff-containerd/pkg/snapshotter"
	"github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
)

type Di3FSManager struct {
	dimgStore *image.DimgStore
	mounts    map[string]struct{}
}

func NewDi3FSManager(storePath string) (*Di3FSManager, error) {
	store, err := image.NewDimgStore(storePath)
	if err != nil {
		return nil, err
	}

	dm := &Di3FSManager{
		dimgStore: store,
		mounts:    map[string]struct{}{},
	}

	return dm, nil
}

func (f *Di3FSManager) UpdateStore() error {
	return f.dimgStore.Walk()
}

func (f *Di3FSManager) UnmountAll() {
	for m := range f.mounts {
		err := exec.Command("fusermount3", "-u", m).Run()
		if err != nil {
			log.G(context.TODO()).Errorf("failed to unmount %s", m)
			continue
		}
		log.G(context.TODO()).Infof("unmounted %s", m)
	}

	f.mounts = map[string]struct{}{}
}

func (f *Di3FSManager) Mount(ctx context.Context, mountpoint string, labels map[string]string) error {
	log.G(ctx).WithFields(logrus.Fields{
		"mountpoint": mountpoint,
		"labels":     labels,
	}).Info("DummyFS Mount called")
	tempDimgPath, ok := labels[sns.SnapshotLabelTempDimg]
	if !ok {
		return errdefs.ErrNotFound
	}
	err := f.dimgStore.AddDimg(tempDimgPath)
	if err != nil {
		return fmt.Errorf("failed to add dimg %s to DimgStore: %v", tempDimgPath, err)
	}

	d, ok := labels[sns.SnapshotLabelRefUncompressed]
	if !ok {
		return errdefs.ErrNotFound
	}

	dimgPaths, err := f.dimgStore.GetDimgPaths(digest.Digest(d))
	if err != nil {
		return fmt.Errorf("failed to get dimg paths for %s: %v", d, err)
	}

	err = f.mountDImg(ctx, mountpoint, dimgPaths)
	if err != nil {
		return err
	}
	f.mounts[mountpoint] = struct{}{}
	log.G(ctx).Infof("success to mount %q", d)
	return nil
}

func (f *Di3FSManager) mountDImg(ctx context.Context, mountpoint string, dimgPaths []string) error {
	log.G(ctx).WithFields(logrus.Fields{
		"mountpoint": mountpoint,
		"dimgPaths":  dimgPaths,
	}).Info("start to mount DImg")
	mountDoneChan := make(chan bool)
	go func() {
		err := di3fs.Do(dimgPaths, mountpoint, mountDoneChan)
		if err != nil {
			log.G(ctx).WithFields(logrus.Fields{
				"dimgPaths": dimgPaths,
			}).Errorf("failed to mount di3fs : %v", err)
		}
	}()
	// wait for the mount to be done
	<-mountDoneChan
	return nil
}

func (f *Di3FSManager) Check(ctx context.Context, mountpoint string, labels map[string]string) error {
	log.G(ctx).WithFields(logrus.Fields{
		"mountpoint": mountpoint,
		"labels":     labels,
	}).Info("DummyFS Check called")
	return nil
}

func (f *Di3FSManager) Unmount(ctx context.Context, mountpoint string) error {
	log.G(ctx).WithFields(logrus.Fields{
		"mountpoint": mountpoint,
	}).Info("DummyFS Unmount called")

	delete(f.mounts, mountpoint)
	err := exec.Command("fusermount3", "-u", mountpoint).Run()
	if err != nil {
		log.G(ctx).Errorf("failed to unmount %s", mountpoint)
		return err
	}
	return nil
}
