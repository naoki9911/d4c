#!/bin/bash

set -eu
if [ $EUID -ne 0 ]; then
	echo "root previlige required"
	exit 1
fi

RUN_NUM=1
RESULT_DIR=$1
IMAGE_DIR=$2
TEST=$3
THREAD_NUM=$4
SCHED_MODE=$5
COMP_MODE=$6

PATH=$PATH:/usr/local/go/bin

cd $(cd $(dirname $0); pwd)
pushd ../
make all
popd

mkdir -p $RESULT_DIR
mkdir -p $IMAGE_DIR

echo "Benchmarking $TEST thread=$THREAD_NUM sched=$SCHED_MODE comp=$COMP_MODE"
./bench_impl.sh test_$TEST.sh $IMAGE_DIR $RUN_NUM $THREAD_NUM $SCHED_MODE $COMP_MODE
./bench_patch_impl.sh test_$TEST.sh $IMAGE_DIR $RUN_NUM $THREAD_NUM $SCHED_MODE $COMP_MODE
./bench_pull_impl.sh test_$TEST.sh $IMAGE_DIR $RUN_NUM localhost:8081 $THREAD_NUM $SCHED_MODE $COMP_MODE
cat $IMAGE_DIR/$TEST/benchmark.log >> ./$RESULT_DIR/$TEST-benchmark.log
mv $IMAGE_DIR/$TEST/benchmark-io.log ./$RESULT_DIR/$TEST-benchmark-io.log
mv $IMAGE_DIR/$TEST/compare.log ./$RESULT_DIR/$TEST-compare.log
rm $IMAGE_DIR/$TEST/benchmark.log
