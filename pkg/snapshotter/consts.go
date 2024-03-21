package snapshotter

import (
	"context"
	"fmt"

	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/snapshots"
	"github.com/naoki9911/fuse-diff-containerd/pkg/utils"
	"github.com/opencontainers/go-digest"
)

const ImageMediaTypeManifestV2 = "application/vnd.docker.distribution.manifest.v2+json"
const ContentLabelContainerdGC = "containerd.io/gc.ref.content"
const ImageLabelPuller = "puller.containerd.io"
const SnapshotLabelRefImage = "containerd.io/snapshot/di3fs.ref.image"
const SnapshotLabelRefUncompressed = "containerd.io/snapshot/di3fs.ref.uncompressed"
const SnapshotLabelRefImagePath = "containerd.io/snapshot/di3fs.ref.imagepath"
const SnapshotLabelRefLayer = "containerd.io/snapshot/di3fs.ref.layer"
const SnapshotLabelImageName = "containerd.io/snapshot/di3fs.image.name"
const SnapshotLabelImageVersion = "containerd.io/snapshot/di3fs.image.version"
const NerverGC = "containerd.io/gc.root"
const TargetSnapshotLabel = "containerd.io/snapshot.ref"

func CreateSnapshot(ctx context.Context, ss snapshots.Snapshotter, manifestDigest, dimgDigest *digest.Digest) error {
	opts := snapshots.WithLabels(map[string]string{
		NerverGC:                     "hogehoge",
		SnapshotLabelRefImage:        manifestDigest.String(),
		SnapshotLabelRefLayer:        fmt.Sprintf("%d", 0),
		SnapshotLabelRefUncompressed: dimgDigest.String(),
		//targetSnapshotLabel:          chain.Hex(),
		//remoteLabel:                  "true",
	})

	randId := utils.GetRandomId("di3fs")
	// ignore error
	// TODO: handle this correctly
	_ = ss.Remove(ctx, dimgDigest.String())

	_, err := ss.Prepare(ctx, randId, "", opts)
	if err != nil {
		log.G(ctx).WithField("opts", opts).Error("failed to prepare")
		return err
	}
	err = ss.Commit(ctx, dimgDigest.String(), randId, opts)
	if err != nil {
		log.G(ctx).Errorf("failed to commit snapshot :%w", err)
		return err
	}
	log.G(ctx).Debug("commit done")
	return nil
}
