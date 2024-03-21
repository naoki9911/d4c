#!/bin/bash

set -eu
if [ $EUID -ne 0 ]; then
	echo "root previlige required"
	exit 1
fi

RUN_NUM=1
TEST=$1
RESULT_DIR=$2
IMAGE_DIR=$RESULT_DIR/images

PATH=$PATH:"/usr/local/go/bin"
cd $(cd $(dirname $0); pwd)
pushd ../
make all
popd

echo "Benchmarking $TEST"
rm -f $IMAGE_DIR/$TEST/benchmark.log
./bench_patch_impl.sh test_$TEST.sh $IMAGE_DIR $RUN_NUM
cat $IMAGE_DIR/$TEST/benchmark.log >> ./$RESULT_DIR/$TEST-benchmark.log
