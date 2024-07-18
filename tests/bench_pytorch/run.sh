#!/bin/bash

set -eux

if [ $EUID -ne 0 ]; then
    echo "root privilege required"
    exit 1
fi

RUN_NUM=10
RESULT_LOG=benchmark_`date +%Y-%m-%d-%H%M`.log

set +e
systemctl stop d4c-snapshotter
set -e


sudo systemctl start containerd

for ((j=0; j < $RUN_NUM; j++));do
    docker run -it --rm -v $(pwd):/bench pytorch-bench:2.2.1 /bench/bench.sh 5 $RESULT_LOG "native-bs-1-num-1000-epoch-$j"
done

sudo systemctl stop containerd
sudo rm -rf /var/lib/containerd

for ((j=0; j < $RUN_NUM; j++));do
    systemd-run --unit=d4c-snapshotter ../../snapshotter
    systemctl restart containerd
    sleep 2

    ../../ctr-cli load --image pytorch-bench:2.2.0 --cdimg ./2.2.0/image.cdimg 
    ../../ctr-cli load --image pytorch-bench:2.2.1 --cdimg ./2.2.0-2.2.1-bsdiffx.cdimg 
    CNI_PATH=../../nerdctl ./../../nerdctl/nerdctl run --snapshotter=di3fs -it --rm -v $(pwd):/bench pytorch-bench:2.2.1 /bench/bench.sh 5 $RESULT_LOG "di3fs-bs-1-num-1000-epoch-$j"

    # TODO: this intended to avoid error: 'cannot remove snapshot with child: failed precondition'
    sudo systemctl stop d4c-snapshotter
    sudo systemctl stop containerd
    sudo rm -rf /var/lib/containerd
done
