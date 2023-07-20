// partially copied from https://github.com/mc256/starlight/blob/25af23b9655e133ab0e5f31a7dde5aaec2241bfa/client/client.go

package main

import (
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/containerd/containerd"
	snapshotsapi "github.com/containerd/containerd/api/services/snapshots/v1"
	"github.com/containerd/containerd/contrib/snapshotservice"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/snapshots"
	sns "github.com/naoki9911/fuse-diff-containerd/pkg/snapshotter"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

type Client struct {
	ctx context.Context

	ctr *containerd.Client

	snRootPath       string
	snImageStorePath string
	snSocketPath     string
	snGrpcServer     *grpc.Server
	snSnapshotter    *snapshotter
	snListener       net.Listener
}

func unmountDi3FS() error {
	cmd := exec.Command("mount", "-l")
	mountList, err := cmd.Output()
	if err != nil {
		return err
	}
	mounts := strings.Split(string(mountList), "\n")
	for _, m := range mounts {
		if !strings.HasPrefix(m, "fuse-diff") {
			continue
		}
		cols := strings.Split(m, " ")
		di3fsMountPath := cols[2]
		err = exec.Command("fusermount3", "-u", di3fsMountPath).Run()
		if err != nil {
			log.G(context.TODO()).Errorf("failed to unmount %s", di3fsMountPath)
			return err
		}
		log.G(context.TODO()).Infof("unmounted %s", di3fsMountPath)
	}

	return nil
}

func removeImages(ctx context.Context, ctrClient *containerd.Client) error {
	imgStore := ctrClient.ImageService()
	imgs, err := imgStore.List(ctx)
	if err != nil {
		return err
	}
	for _, img := range imgs {
		targetSns, ok := img.Labels[sns.TargetSnapshotLabel]
		if !ok {
			continue
		}
		if targetSns != "di3fs" {
			continue
		}
		err = imgStore.Delete(ctx, img.Name)
		if err != nil {
			log.G(ctx).Errorf("failed to remove image %s", img.Name)
		}
		log.G(ctx).Infof("removed image %s", img.Name)
	}

	return nil
}

func NewClient() (*Client, error) {
	ctr, err := containerd.New("/run/containerd/containerd.sock", containerd.WithDefaultNamespace("default"))
	if err != nil {
		return nil, err
	}

	c := &Client{
		ctx:          context.TODO(),
		ctr:          ctr,
		snRootPath:   "/tmp/di3fs/sn",
		snSocketPath: "/run/di3fs/snapshotter.sock",
	}

	c.snImageStorePath = filepath.Join(c.snRootPath, "images")

	return c, nil
}

func main() {
	log.GetLogger(context.TODO()).Logger.SetLevel(logrus.DebugLevel)
	err := unmountDi3FS()
	if err != nil {
		log.G(context.TODO()).WithError(err).Fatal("failed to unmount di3fs")
	}
	client, err := NewClient()
	if err != nil {
		log.G(context.TODO()).WithError(err).Error("failed to create client")
		os.Exit(1)
	}

	err = client.initSnapshotter()
	if err != nil {
		log.G(client.ctx).WithError(err).Error("failed to init snapsohtter")
		os.Exit(1)
	}

	client.startSnapshotter()
}

func (c *Client) initSnapshotter() error {
	log.G(c.ctx).Debug("initializing snapshotter")
	c.snGrpcServer = grpc.NewServer()
	var err error
	fs := &DummyFS{}
	err = removeImages(context.TODO(), c.ctr)
	if err != nil {
		log.G(context.TODO()).WithError(err).Error("failed to remove images")
		os.Exit(1)
	}
	if err = os.RemoveAll(c.snRootPath); err != nil {
		return errors.Wrapf(err, "failed to remove %q", c.snRootPath)
	}
	if err = os.MkdirAll(c.snRootPath, 0755); err != nil {
		return errors.Wrapf(err, "failed to create directory %q", c.snRootPath)
	}
	if err = os.MkdirAll(c.snImageStorePath, 0755); err != nil {
		return errors.Wrapf(err, "failed to create directory %q", c.snRootPath)
	}
	c.snSnapshotter, err = NewSnapshotter(c.ctx, c.snRootPath, fs)
	if err != nil {
		return err
	}

	svc := snapshotservice.FromSnapshotter(c.snSnapshotter)
	socketDir := filepath.Dir(c.snSocketPath)
	if err = os.MkdirAll(socketDir, 0700); err != nil {
		return errors.Wrapf(err, "failed to create directory %q", socketDir)
	}

	if err = os.RemoveAll(c.snSocketPath); err != nil {
		return errors.Wrapf(err, "failed to remove %q", c.snSocketPath)
	}

	snapshotsapi.RegisterSnapshotsServer(c.snGrpcServer, svc)

	return nil
}

func (c *Client) startSnapshotter() {
	log.G(c.ctx).Debug("snapshotter service starting")
	// Listen and serve
	var err error
	c.snListener, err = net.Listen("unix", c.snSocketPath)
	if err != nil {
		log.G(c.ctx).WithError(err).Errorf("failed to listen on %q", c.snSocketPath)
		return
	}

	log.G(c.ctx).
		WithField("socket", c.snSocketPath).
		Info("di3fs snapshotter service started")

	c.snSnapshotter.Walk(c.ctx, func(ctx context.Context, i snapshots.Info) error {
		log.G(ctx).WithField("snapshots.Info", i).Info("walking")
		return nil
	})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		if err = c.snGrpcServer.Serve(c.snListener); err != nil {
			log.G(c.ctx).WithError(err).Errorf("failed to serve snapshotter")
			return
		}
	}()

	wg.Wait()
}
