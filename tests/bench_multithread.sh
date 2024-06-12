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

TESTS=("postgres")
THREADS=("1" "2" "4" "8")
ENCODINGS=("bsdiffx" "xdelta3")
for TEST in "${TESTS[@]}"; do
	for THREAD in "${THREADS[@]}"; do
		for ENCODING in "${ENCODINGS[@]}"; do
			./bench_multithread_impl.sh test_$TEST.sh $IMAGE_DIR $RUN_NUM $THREAD none bzip2 $ENCODING

		done
	done
done

cat $RESULT_DIR/images/$TEST/benchmark.log >> ./benchmark-diff-multithread.log
cat $RESULT_DIR/images/$TEST/benchmark-merge.log >> ./benchmark-diff-multithread.log
