# D4C: Delta updating for Container images

[![Test](https://github.com/naoki9911/d4c/actions/workflows/test.yaml/badge.svg)](https://github.com/naoki9911/d4c/actions/workflows/test.yaml)

This repository is PoC implementation of delta updating for containers.

**D4C is in early development stage and more works are left.**

# Articles
- [N. Matsumoto, D. Kotani and Y. Okabe, "Efficient Container Image Updating in Low-bandwidth Networks with Delta Encoding," 2023 IEEE International Conference on Cloud Engineering (IC2E), Boston, MA, USA, 2023, pp. 1-10, doi: 10.1109/IC2E59103.2023.00009.](https://ieeexplore.ieee.org/document/10305845)

# How to use
## Dependency
D4C depends on the below software.

- go
- docker
- docker-squash
- containerd
- fuse3

For Ubuntu 22.04, install these packages.
```sh
sudo apt install -y docker-ce containerd.io golang-go fuse3
pip install docker-squash
```

## Configure containerd
D4C provides snapshotter plugin for containerd.
Configure containerd with `install_snapshotter.sh`
```sh
./install_snapshotter.sh
```

## Build binaries
```sh
make all
```

## Convert docker image into Cdimg format
```sh
mkdir images
sudo ./ctr-cli convert --image nginx:1.23.1 --output ./images/nginx-1.23.1 --cdimg
sudo ./ctr-cli convert --image nginx:1.23.2 --output ./images/nginx-1.23.2 --cdimg
sudo ./ctr-cli convert --image nginx:1.23.3 --output ./images/nginx-1.23.3 --cdimg
sudo ./ctr-cli convert --image nginx:1.23.4 --output ./images/nginx-1.23.4 --cdimg
```

## Generate deltas
```sh
./ctr-cli cdimg diff --oldCdimg ./images/nginx-1.23.1/image.cdimg --newCdimg ./images/nginx-1.23.2/image.cdimg --outCdimg ./images/diff_nginx-1.23.1-2.cdimg --threadNum 8
./ctr-cli cdimg diff --oldCdimg ./images/nginx-1.23.2/image.cdimg --newCdimg ./images/nginx-1.23.3/image.cdimg --outCdimg ./images/diff_nginx-1.23.2-3.cdimg --threadNum 8
./ctr-cli cdimg diff --oldCdimg ./images/nginx-1.23.3/image.cdimg --newCdimg ./images/nginx-1.23.4/image.cdimg --outCdimg ./images/diff_nginx-1.23.3-4.cdimg --threadNum 8
```

## Run snapshotter plugin in the other terminal
```sh
sudo ./snapshotter
```

## Load images
```sh
sudo ./ctr-cli load --image d4c-nginx:1.23.1 --cdimg ./images/nginx-1.23.1/image.cdimg
sudo ./ctr-cli load --image d4c-nginx:1.23.2 --cdimg ./images/diff_nginx-1.23.1-2.cdimg
sudo ./ctr-cli load --image d4c-nginx:1.23.3 --cdimg ./images/diff_nginx-1.23.2-3.cdimg
sudo ./ctr-cli load --image d4c-nginx:1.23.4 --cdimg ./images/diff_nginx-1.23.3-4.cdimg
```

## Run container
```sh
sudo ctr run --rm --snapshotter=di3fs --net-host d4c-nginx:1.23.4 test-nginx-1.23.4
```


## Push container image deltas
```sh
./ctr-cli push --cdimg ./images/nginx-1.23.1/image.cdimg --imageTag d4c-nginx:1.23.1
./ctr-cli push --cdimg ./images/diff_nginx-1.23.1-2.cdimg
./ctr-cli push --cdimg ./images/diff_nginx-1.23.2-3.cdimg
./ctr-cli push --cdimg ./images/diff_nginx-1.23.3-4.cdimg --imageTag d4c-nginx:1.23.4
```

## Pull container images
```sh
sudo ./ctr-cli pull --image d4c-nginx:1.23.1 --host localhost:8081
sudo ./ctr-cli pull --image d4c-nginx:1.23.4 --host localhost:8081
```
