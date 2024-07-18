#!/bin/bash

set -eu
if [ $EUID -ne 0 ]; then
	echo "root previlige required"
	exit 1
fi

PATH=$PATH:/usr/local/go/bin

cd $(cd $(dirname $0); pwd)
pushd ../
make all
popd

RESULT_DIR=benchmark_`date +%Y-%m-%d-%H%M`
IMAGE_DIR=$RESULT_DIR/images
mkdir -p $IMAGE_DIR
mkdir -p /tmp/benchmark

TESTS=("nginx" "postgres" "redis" "pytorch")
ENCODINGS=("bsdiffx" "xdelta3")
for TEST in "${TESTS[@]}"; do
	for ENCODING in "${ENCODINGS[@]}"; do
		./bench_single.sh $RESULT_DIR $IMAGE_DIR $TEST 8 none bzip2 $ENCODING
	done
	cat $RESULT_DIR/$TEST-benchmark.log >> $RESULT_DIR/benchmark.log
	cat $RESULT_DIR/$TEST-benchmark-io.log >> $RESULT_DIR/benchmark-io.log
	cat $RESULT_DIR/$TEST-benchmark-merge.log >> $RESULT_DIR/benchmark-merge.log
	cat $RESULT_DIR/$TEST-benchmark-merge.log >> $RESULT_DIR/benchmark.log
	cat $RESULT_DIR/$TEST-compare.log >> $RESULT_DIR/compare.log
	rm $RESULT_DIR/$TEST-benchmark.log
	rm $RESULT_DIR/$TEST-benchmark-io.log
	rm $RESULT_DIR/$TEST-benchmark-merge.log
	rm $RESULT_DIR/$TEST-compare.log
done

TEST="postgres"
THREADS=("1" "2" "4" "8")
for THREAD in "${THREADS[@]}"; do
	for ENCODING in "${ENCODINGS[@]}"; do
		./bench_single.sh $RESULT_DIR $IMAGE_DIR $TEST $THREAD none bzip2 $ENCODING

	done
done
cat $RESULT_DIR/$TEST-benchmark.log >> $RESULT_DIR/mt-benchmark.log
cat $RESULT_DIR/$TEST-benchmark-io.log >> $RESULT_DIR/mt-benchmark-io.log
cat $RESULT_DIR/$TEST-benchmark-merge.log >> $RESULT_DIR/mt-benchmark-merge.log
cat $RESULT_DIR/$TEST-benchmark-merge.log >> $RESULT_DIR/mt-benchmark.log
cat $RESULT_DIR/$TEST-compare.log >> $RESULT_DIR/mt-compare.log
