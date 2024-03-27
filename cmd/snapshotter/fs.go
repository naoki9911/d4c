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

type Di3FSManager struct {
	mounts map[string]struct{}
}

func NewDi3FSManager() *Di3FSManager {
	return &Di3FSManager{
		mounts: map[string]struct{}{},
	}
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
	d, ok := labels[sns.SnapshotLabelRefUncompressed]
	if !ok {
		return errdefs.ErrNotFound
	}

	err := f.mountDImg(ctx, mountpoint, d)
	if err != nil {
		return err
	}
	f.mounts[mountpoint] = struct{}{}
	log.G(ctx).Infof("success to mount %q", d)
	return nil
}

func (f *Di3FSManager) mountDImg(ctx context.Context, mountpoint, dimgDigest string) error {
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
