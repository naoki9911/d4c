#!/bin/bash

set -eux

if [ $EUID -ne 0 ]; then
    echo "root privilege required"
    exit 1
fi

RESULT_LOG=benchmark_`date +%Y-%m-%d-%H%M`.log

for i in $(seq 1 10); do
    echo "benchmarking $i/10"

    sudo systemctl restart containerd
    docker run --net host --name postgres-bench -d -e POSTGRES_PASSWORD=bench -e POSTGRES_USER=bench postgres:13.2

    sleep 6

    docker run --rm --net host postgres:13.2 pgbench -i -s 100 -q -h localhost -U bench bench
    echo "=== native postgres:13.2 ===" >> $RESULT_LOG
    docker run --rm --net host postgres:13.2 pgbench -c 10 -T 120 -h localhost -U bench bench >> $RESULT_LOG
    docker rm -f postgres-bench

    set +e
    systemctl stop d4c-snapshotter
    set -e

    systemctl stop containerd
    rm -rf /var/lib/containerd
    systemd-run --unit=d4c-snapshotter ../../snapshotter
    systemctl restart containerd
    sleep 2

    ../../ctr-cli load --image postgres:13.1 --cdimg ./13.1/image.cdimg 
    ../../ctr-cli load --image postgres:13.2 --cdimg ./13.1-13.2-bsdiffx.cdimg 
    CNI_PATH=../../nerdctl ./../../nerdctl/nerdctl run --snapshotter=di3fs --net host --name postgres-bench -d -e POSTGRES_PASSWORD=bench -e POSTGRES_USER=bench postgres:13.2

    sleep 6
    docker run --rm --net host postgres:13.2 pgbench -i -s 100 -q -h localhost -U bench bench
    echo "=== Di3FS(bsdiff) postgres:13.2 ===" >> $RESULT_LOG
    docker run --rm --net host postgres:13.2 pgbench -c 10 -T 120 -h localhost -U bench bench >> $RESULT_LOG
    ./../../nerdctl/nerdctl rm -f postgres-bench
    systemctl stop d4c-snapshotter
done