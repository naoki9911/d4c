#!/bin/bash

set -eu
if [ $EUID -ne 0 ]; then
	echo "root previlige required"
	exit 1
fi

RUN_NUM=1
TEST=$1
THREAD_NUM=${2:-1}
SCHED_MODE=${3,-"none"}

PATH=$PATH:/usr/local/go/bin

cd $(cd $(dirname $0); pwd)
pushd ../
make all
popd

RESULT_DIR=benchmark
IMAGE_DIR=$RESULT_DIR/images
mkdir -p $IMAGE_DIR

echo "Benchmarking $TEST"
./bench_impl.sh test_$TEST.sh $IMAGE_DIR $RUN_NUM $THREAD_NUM $SCHED_MODE
cp $IMAGE_DIR/$TEST/benchmark.log ./$RESULT_DIR/$TEST-benchmark.log

./bench_patch_lxc.sh $TEST $RESULT_DIR
./bench_pull_impl.sh test_$TEST.sh $IMAGE_DIR $RUN_NUM localhost:8081
