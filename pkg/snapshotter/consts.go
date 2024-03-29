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
const SnapshotLabelMount = "containerd.io/snapshot/di3fs.mount"
const SnapshotLabelTempDimg = "containerd.io/snapshot/di3fs.tempDimg"
const NerverGC = "containerd.io/gc.root"
const TargetSnapshotLabel = "containerd.io/snapshot.ref"

func CreateSnapshot(ctx context.Context, ss snapshots.Snapshotter, manifestDigest, dimgDigest *digest.Digest, imageName string, dimgPath string) error {
	opts := snapshots.WithLabels(map[string]string{
		NerverGC:                     "hogehoge",
		SnapshotLabelRefImage:        manifestDigest.String(),
		SnapshotLabelRefLayer:        fmt.Sprintf("%d", 0),
		SnapshotLabelRefUncompressed: dimgDigest.String(),
		SnapshotLabelImageName:       imageName,
		SnapshotLabelTempDimg:        dimgPath,
		//targetSnapshotLabel:          chain.Hex(),
		//remoteLabel:                  "true",
	})

	log.G(ctx).Infof("IMAGE[%s] digest=%s", imageName, dimgDigest)
	randId := utils.GetRandomId("di3fs")
	// ignore error
	// TODO: handle this correctly
	_ = ss.Remove(ctx, dimgDigest.String())

	mounts, err := ss.Prepare(ctx, randId, "", opts)
	if err != nil {
		log.G(ctx).WithField("opts", opts).Error("failed to prepare")
		return err
	}
	log.G(ctx).Infof("mounts=%v", mounts)
	mountPath := ""
	if len(mounts) > 0 {
		mountPath = mounts[0].Source
	}
	optsWithMount := snapshots.WithLabels(map[string]string{
		SnapshotLabelMount: mountPath,
	})
	err = ss.Commit(ctx, dimgDigest.String(), randId, opts, optsWithMount)
	if err != nil {
		log.G(ctx).Errorf("failed to commit snapshot : %v", err)
		return err
	}
	log.G(ctx).Debug("commit done")
	return nil
}
