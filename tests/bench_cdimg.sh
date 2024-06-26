#!/bin/bash

set -eu
if [ $EUID -ne 0 ]; then
	echo "root previlige required"
	exit 1
fi
RUN_NUM=1
THREAD_NUM=${1:-1}
SCHED_MODE=${2,-"none"}
COMP_MODE=$3

PATH=$PATH:/usr/local/go/bin

cd $(cd $(dirname $0); pwd)
pushd ../
make all
popd

RESULT_DIR=benchmark_`date +%Y-%m-%d-%H%M`
IMAGE_DIR=$RESULT_DIR/images
mkdir -p $IMAGE_DIR

TESTS=("apache" "mysql" "nginx" "postgres" "redis")
for TEST in "${TESTS[@]}"
do
	echo "Benchmarking $TEST"
	./bench_cdimg_impl.sh test_$TEST.sh $IMAGE_DIR $RUN_NUM $THREAD_NUM $SCHED_MODE $COMP_MODE
	cp $IMAGE_DIR/$TEST/benchmark.log ./$RESULT_DIR/$TEST-benchmark.log
done
