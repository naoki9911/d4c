#!/bin/bash

set -eu
if [ $EUID -ne 0 ]; then
	echo "root previlige required"
	exit 1
fi
RUN_NUM=1

PATH=$PATH:/usr/local/go/bin

cd $(cd $(dirname $0); pwd)
pushd ../
make all
popd

RESULT_DIR=benchmark_`date +%Y-%m-%d-%H%M`
IMAGE_DIR=$RESULT_DIR/images
mkdir -p $IMAGE_DIR

TESTS=("apache" "mysql" "nginx" "postgres" "redis")
COMP_MODES=("bzip2" "zstd")
for TEST in "${TESTS[@]}"; do
	for COMP_MODE in "${COMP_MODES[@]}"; do
		./bench_file_impl.sh test_$TEST.sh $IMAGE_DIR 1 4 none $COMP_MODE
	done
	cat $IMAGE_DIR/$TEST/benchmark.log >> /tmp/benchmark.log
done
