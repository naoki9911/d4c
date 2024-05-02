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
mkdir -p /tmp/benchmark

TESTS=("apache" "mysql" "nginx" "postgres" "redis")
ENCODINGS=("bsdiffx" "xdelta3")
#TESTS=("pytorch" "tensorflow")
#TESTS=("pytorch")
for TEST in "${TESTS[@]}"; do
	for ENCODING in "${ENCODINGS[@]}"; do
		./bench_single.sh $RESULT_DIR $IMAGE_DIR $TEST 8 "none" "bzip2" $ENCODING
	done
done

cat $RESULT_DIR/$TEST-benchmark.log >> /tmp/benchmark/benchmark.log
cat $RESULT_DIR/$TEST-compare.log >> /tmp/benchmark/compare.log
