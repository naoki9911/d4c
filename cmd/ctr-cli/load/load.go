package load

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/log"
	"github.com/naoki9911/fuse-diff-containerd/pkg/image"
	sns "github.com/naoki9911/fuse-diff-containerd/pkg/snapshotter"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var Flags = []cli.Flag{
	&cli.StringFlag{
		Name:     "image",
		Usage:    "image name to be loaded",
		Required: true,
	},
	&cli.StringFlag{
		Name:     "dimg",
		Usage:    "path to dimg to be loaded",
		Required: true,
	},
}

func LoadImage(snClient *sns.Client, ctx context.Context, imageName, imageVersion string, image *image.Di3FSImage) error {
	cs := snClient.CtrClient.ContentStore()
	cs.Delete(ctx, image.Header.ManifestDigest)
	err := content.WriteBlob(ctx, cs, image.Header.ManifestDigest.Hex(), bytes.NewReader(image.ManifestBytes),
		v1.Descriptor{
			Size:   int64(len(image.ManifestBytes)),
			Digest: image.Header.ManifestDigest,
		},
		content.WithLabels(map[string]string{
			sns.NerverGC:         "hoghoge",
			sns.ImageLabelPuller: "di3fs",
			fmt.Sprintf("%s.config", sns.ContentLabelContainerdGC): image.Manifest.Config.Digest.String(),
			//fmt.Sprintf("%s.di3fs", ContentLabelContainerdGC):  dId.String(),
		}),
	)
	if err != nil {
		return err
	}
	log.G(ctx).Debug("load manifest done")

	err = content.WriteBlob(
		ctx, cs, image.Manifest.Config.Digest.Hex(), bytes.NewReader(image.ConfigBytes),
		v1.Descriptor{
			Size:   int64(len(image.ConfigBytes)),
			Digest: image.Manifest.Config.Digest,
		},
		content.WithLabels(map[string]string{
			sns.NerverGC: "hoghoge",
		}),
	)
	if err != nil {
		return err
	}
	log.G(ctx).Debug("load config done")

	// register image
	is := snClient.CtrClient.ImageService()
	is.Delete(ctx, "test-image")
	_, err = is.Create(ctx, images.Image{
		Name: imageName + ":" + imageVersion,
		Target: v1.Descriptor{
			MediaType: sns.ImageMediaTypeManifestV2,
			Digest:    image.Header.ManifestDigest,
			Size:      int64(len(image.ManifestBytes)),
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Labels: map[string]string{
			sns.TargetSnapshotLabel:       "di3fs",
			sns.SnapshotLabelImageName:    imageName,
			sns.SnapshotLabelImageVersion: imageVersion,
		},
	})
	if err != nil {
		return err
	}

	// now ready to create snapshot
	err = sns.CreateSnapshot(ctx, snClient.SnClient, &image.Header.ManifestDigest, &image.DImgDigest)
	if err != nil {
		return err
	}

	log.G(ctx).WithFields(logrus.Fields{
		"header":   image.Header,
		"manifest": image.Manifest,
		"config":   image.Config,
	}).Debugf("image loaded")

	return nil
}

func Load(ctx context.Context, imgNameWithVersion, imgPath string) error {
	snClient, err := sns.NewClient()
	if err != nil {
		return err
	}

	imgNames := strings.Split(imgNameWithVersion, ":")
	if len(imgNames) != 2 {
		return fmt.Errorf("invalid image name %s", imgNameWithVersion)
	}
	imgName := imgNames[0]
	imgVersion := imgNames[1]
	log.G(ctx).WithFields(logrus.Fields{"imageName": imgName, "imageVersion": imgVersion}).Infof("loading image from %q", imgPath)
	// load image
	image, err := image.Load(imgPath)
	if err != nil {
		return err
	}
	log.G(ctx).Info("loaded image")

	// extract dimg
	imagePath := filepath.Join(snClient.SnImageStorePath, image.DImgDigest.String()+".dimg")
	_, err = image.Image.Seek(image.DImgOffset, 0)
	if err != nil {
		return err
	}
	dimgFile, err := os.Create(imagePath)
	if err != nil {
		return err
	}
	defer dimgFile.Close()
	_, err = io.Copy(dimgFile, image.Image)
	if err != nil {
		return err
	}

	err = LoadImage(snClient, ctx, imgName, imgVersion, image)
	if err != nil {
		return err
	}

	return nil
}

func Action(c *cli.Context) error {
	imgName := c.String("image")
	imgPath := c.String("dimg")
	err := Load(context.TODO(), imgName, imgPath)
	if err != nil {
		return err
	}
	return nil
}

func Command() *cli.Command {
	cmd := cli.Command{
		Name:  "load",
		Usage: "Load dimg",
		Action: func(context *cli.Context) error {
			return Action(context)
		},
		Flags: Flags,
	}

	return &cmd
}
