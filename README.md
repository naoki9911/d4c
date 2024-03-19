# D4C: Delta updating for Container images
[![Test](https://github.com/naoki9911/d4c/actions/workflows/test.yaml/badge.svg)](https://github.com/naoki9911/d4c/actions/workflows/test.yaml)
This repository is PoC implementation of delta updating for containers.

**D4C is in early development stage and more works are left.**

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

## Squash docker image layers
```sh
mkdir images
./ctr-cli convert --image nginx:1.23.1 --output ./images/nginx-1.23.1
./ctr-cli convert --image nginx:1.23.2 --output ./images/nginx-1.23.2
./ctr-cli convert --image nginx:1.23.3 --output ./images/nginx-1.23.3
./ctr-cli convert --image nginx:1.23.4 --output ./images/nginx-1.23.4
```

## Extract files
```sh
mkdir ./images/nginx-1.23.1/root
sudo tar -xf ./images/nginx-1.23.1/layer.tar -C ./images/nginx-1.23.1/root
mkdir ./images/nginx-1.23.2/root
sudo tar -xf ./images/nginx-1.23.2/layer.tar -C ./images/nginx-1.23.2/root
mkdir ./images/nginx-1.23.3/root
sudo tar -xf ./images/nginx-1.23.3/layer.tar -C ./images/nginx-1.23.3/root
mkdir ./images/nginx-1.23.4/root
sudo tar -xf ./images/nginx-1.23.4/layer.tar -C ./images/nginx-1.23.4/root
```

## Generate deltas
```sh
sudo ./diff "" images/nginx-1.23.1/root images/base_nginx-1.23.1 images/base_nginx-1.23.1.json binary-diff
sudo ./diff images/nginx-1.23.1/root images/nginx-1.23.2/root images/diff_nginx-1.23.1-2 images/diff_nginx-1.23.1-2.json binary-diff
sudo ./diff images/nginx-1.23.2/root images/nginx-1.23.3/root images/diff_nginx-1.23.2-3 images/diff_nginx-1.23.2-3.json binary-diff
sudo ./diff images/nginx-1.23.3/root images/nginx-1.23.4/root images/diff_nginx-1.23.3-4 images/diff_nginx-1.23.3-4.json binary-diff
```

## Pack deltas
`.dimg` files are the body of delta files
```sh
sudo ./pack images/base_nginx-1.23.1 images/base_nginx-1.23.1.json "" images/base_nginx-1.23.1.dimg
sudo ./pack images/diff_nginx-1.23.1-2 images/diff_nginx-1.23.1-2.json images/base_nginx-1.23.1.dimg images/diff_nginx-1.23.1-2.dimg
sudo ./pack images/diff_nginx-1.23.2-3 images/diff_nginx-1.23.2-3.json images/diff_nginx-1.23.1-2.dimg images/diff_nginx-1.23.2-3.dimg
sudo ./pack images/diff_nginx-1.23.3-4 images/diff_nginx-1.23.3-4.json images/diff_nginx-1.23.2-3.dimg images/diff_nginx-1.23.3-4.dimg
```

## Pack deltas as portable format
`.cdimg` files are portable delta format.
Users can simply load container images with them.
```sh
./ctr-cli pack --manifest=./images/nginx-1.23.1/manifset.json --config=./images/nginx-1.23.1/config.json --dimg=./images/base_nginx-1.23.1.dimg --out=./images/base_nginx-1.23.1.cdimg
./ctr-cli pack --manifest=./images/nginx-1.23.2/manifset.json --config=./images/nginx-1.23.2/config.json --dimg=./images/diff_nginx-1.23.1-2.dimg --out=./images/diff_nginx-1.23.1-2.cdimg
./ctr-cli pack --manifest=./images/nginx-1.23.3/manifset.json --config=./images/nginx-1.23.3/config.json --dimg=./images/diff_nginx-1.23.2-3.dimg --out=./images/diff_nginx-1.23.2-3.cdimg
./ctr-cli pack --manifest=./images/nginx-1.23.4/manifset.json --config=./images/nginx-1.23.4/config.json --dimg=./images/diff_nginx-1.23.3-4.dimg --out=./images/diff_nginx-1.23.3-4.cdimg
```

## Run snapshotter plugin
```sh
sudo ./snapshotter
```

## Load images
```sh
sudo ./ctr-cli load --image=d4c-nginx:1.23.1 --dimg=./images/base_nginx-1.23.1.cdimg
sudo ./ctr-cli load --image=d4c-nginx:1.23.2 --dimg=./images/diff_nginx-1.23.1-2.cdimg
sudo ./ctr-cli load --image=d4c-nginx:1.23.3 --dimg=./images/diff_nginx-1.23.2-3.cdimg
sudo ./ctr-cli load --image=d4c-nginx:1.23.4 --dimg=./images/diff_nginx-1.23.3-4.cdimg
```

## Run container
```sh
sudo ctr run --rm --snapshotter=di3fs --net-host d4c-nginx:1.23.4 test-nginx-1.23.4
```


## Push container image deltas
```sh
./push_nginx.sh
```

## Pull container images
```sh
sudo ./ctr-cli pull --image d4c-nginx:1.23.1 --host localhost:8081
sudo ./ctr-cli pull --image d4c-nginx:1.23.3 --host localhost:8081
```
