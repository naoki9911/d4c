package snapshotter

import (
	"path/filepath"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/snapshots"
)

type Client struct {
	CtrClient *containerd.Client

	SnClient         snapshots.Snapshotter
	SnRootPath       string
	SnImageStorePath string
}

func NewClient() (*Client, error) {
	ctr, err := containerd.New("/run/containerd/containerd.sock", containerd.WithDefaultNamespace("default"))
	if err != nil {
		return nil, err
	}

	c := &Client{
		CtrClient:  ctr,
		SnRootPath: "/tmp/di3fs/sn",
	}

	c.SnClient = c.CtrClient.SnapshotService("di3fs")
	c.SnImageStorePath = filepath.Join(c.SnRootPath, "images")

	return c, nil
}
