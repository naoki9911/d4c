package main

import (
	"context"
	"os/exec"
	"path"

	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/log"
	"github.com/naoki9911/fuse-diff-containerd/pkg/di3fs"
	sns "github.com/naoki9911/fuse-diff-containerd/pkg/snapshotter"
	"github.com/sirupsen/logrus"
)

var imageStore = "/tmp/di3fs/sn/images"

type DummyFS struct {
}

func (f *DummyFS) Mount(ctx context.Context, mountpoint string, labels map[string]string) error {
	log.G(ctx).WithFields(logrus.Fields{
		"mountpoint": mountpoint,
		"labels":     labels,
	}).Info("DummyFS Mount called")
	d, ok := labels[sns.SnapshotLabelRefUncompressed]
	if !ok {
		return errdefs.ErrNotFound
	}

	err := f.mountDImg(ctx, mountpoint, d)
	if err != nil {
		return err
	}
	log.G(ctx).Infof("success to mount %q", d)
	return nil
}

func (f *DummyFS) mountDImg(ctx context.Context, mountpoint, dimgDigest string) error {
	log.G(ctx).WithFields(logrus.Fields{
		"mountpoint": mountpoint,
		"dimgDigest": dimgDigest,
	}).Info("start to mount DImg")
	patchDirPath := path.Join(imageStore, dimgDigest+".dimg")
	log.G(ctx).WithFields(logrus.Fields{
		"patchDirPath": patchDirPath,
	}).Info("mounting di3fs")
	go func() {
		err := di3fs.Do(patchDirPath, mountpoint)
		if err != nil {
			log.G(ctx).WithFields(logrus.Fields{
				"patchDirPath": patchDirPath,
			}).Errorf("failed to mount di3fs : %v", err)
		}
	}()
	return nil
}

func (f *DummyFS) Check(ctx context.Context, mountpoint string, labels map[string]string) error {
	log.G(ctx).WithFields(logrus.Fields{
		"mountpoint": mountpoint,
		"labels":     labels,
	}).Info("DummyFS Check called")
	return nil
}

func (f *DummyFS) Unmount(ctx context.Context, mountpoint string) error {
	log.G(ctx).WithFields(logrus.Fields{
		"mountpoint": mountpoint,
	}).Info("DummyFS Unmount called")

	err := exec.Command("fusermount3", "-u", mountpoint).Run()
	if err != nil {
		log.G(ctx).Errorf("failed to unmount %s", mountpoint)
		return err
	}
	return nil
}
