#!/bin/bash

set -eux

set -eux

RUN_NUM=$1
LOG_NAME=$2
LABEL=$3

for ((j=0; j < $RUN_NUM; j++));do
    python3 /bench/bench.py $LABEL-run-$j | grep '{' >> /bench/$LOG_NAME
done

#sudo docker run -it --rm -v $(pwd):/bench pytorch-bench:2.2.1 python3 /bench/bench.py "bs-1-num-10000-native"
#sudo CNI_PATH=/home/naoki/d4c/nerdctl ./../../nerdctl/nerdctl run --snapshotter=di3fs -it --rm -v $(pwd):/bench pytorch-bench:2.2.1 python3 /bench/bench.py

# install nerdctl and CNI plugins
# https://github.com/containerd/nerdctl/releases/tag/v1.7.6
# https://github.com/containernetworking/plugins/releases/tag/v1.5.1