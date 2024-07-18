#!/bin/bash

set -eux

if [ $EUID -ne 0 ]; then
	echo "root  privilege required"
	exit 1
fi

docker build -f ./Dockerfile.2.2.0 -t pytorch-bench:2.2.0 .
docker build -f ./Dockerfile.2.2.1 -t pytorch-bench:2.2.1 .
docker save pytorch-bench:2.2.0 -o pytorch-bench-2.2.0.tar
docker save pytorch-bench:2.2.1 -o pytorch-bench-2.2.1.tar

