module github.com/naoki9911/fuse-diff-containerd

go 1.21

toolchain go1.21.3

require (
	github.com/containerd/containerd v1.6.12
	github.com/containerd/continuity v0.3.0
	github.com/dsnet/compress v0.0.1
	github.com/google/go-containerregistry v0.19.1
	github.com/google/uuid v1.2.0
	github.com/hanwen/go-fuse/v2 v2.2.0
	github.com/icedream/go-bsdiff v1.0.1
	github.com/klauspost/compress v1.16.5
	github.com/moby/sys/mountinfo v0.5.0
	github.com/nine-lives-later/go-xdelta v0.3.2-0.20200813195159-a23b3640ca1a
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.1.0-rc3
	github.com/otiai10/copy v1.9.0
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.9.1
	github.com/stretchr/testify v1.9.0
	github.com/urfave/cli/v2 v2.23.7
	golang.org/x/sync v0.2.0
	google.golang.org/grpc v1.47.0
)

replace github.com/icedream/go-bsdiff v1.0.1 => github.com/naoki9911/go-bsdiff v1.0.3

replace github.com/nine-lives-later/go-xdelta v0.3.2-0.20200813195159-a23b3640ca1a => github.com/naoki9911/go-xdelta v0.0.2

require (
	github.com/Microsoft/go-winio v0.6.1 // indirect
	github.com/Microsoft/hcsshim v0.9.5 // indirect
	github.com/containerd/cgroups v1.0.3 // indirect
	github.com/containerd/fifo v1.0.0 // indirect
	github.com/containerd/stargz-snapshotter/estargz v0.14.3 // indirect
	github.com/containerd/ttrpc v1.1.0 // indirect
	github.com/containerd/typeurl v1.0.2 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.2 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/docker/cli v24.0.0+incompatible // indirect
	github.com/docker/distribution v2.8.2+incompatible // indirect
	github.com/docker/docker v24.0.0+incompatible // indirect
	github.com/docker/docker-credential-helpers v0.7.0 // indirect
	github.com/docker/go-events v0.0.0-20190806004212-e31b211e4f1c // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/gogo/googleapis v1.4.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/moby/locker v1.0.1 // indirect
	github.com/moby/sys/signal v0.6.0 // indirect
	github.com/opencontainers/runc v1.1.2 // indirect
	github.com/opencontainers/runtime-spec v1.0.3-0.20210326190908-1c3f411f0417 // indirect
	github.com/opencontainers/selinux v1.10.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/vbatts/tar-split v0.11.3 // indirect
	github.com/xrash/smetrics v0.0.0-20201216005158-039620a65673 // indirect
	go.etcd.io/bbolt v1.3.6 // indirect
	go.opencensus.io v0.23.0 // indirect
	golang.org/x/mod v0.10.0 // indirect
	golang.org/x/net v0.17.0 // indirect
	golang.org/x/sys v0.17.0 // indirect
	golang.org/x/text v0.13.0 // indirect
	golang.org/x/tools v0.9.1 // indirect
	google.golang.org/genproto v0.0.0-20220502173005-c8bf987b8c21 // indirect
	google.golang.org/protobuf v1.30.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
