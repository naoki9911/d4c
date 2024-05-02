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
THREADS=("1" "8")
SCHED_MODES=("none")
COMP_MODES=("bzip2")
ENCODINGS=("bsdiffx" "xdelta3")
for TEST in "${TESTS[@]}"; do
	for THREAD in "${THREADS[@]}"; do
		for ENCODING in "${ENCODINGS[@]}"; do
			./bench_single.sh $RESULT_DIR $IMAGE_DIR $TEST $THREAD none bzip2 $ENCODING

			cat $RESULT_DIR/$TEST-benchmark.log >> /tmp/benchmark/benchmark.log
			cat $RESULT_DIR/$TEST-benchmark-io.log >> /tmp/benchmark/benchmark-io.log
			cat $RESULT_DIR/$TEST-compare.log >> /tmp/benchmark/compare.log
		done
	done
done

python3 ./plot_diff.py /tmp/benchmark/benchmark.log /tmp/benchmark/diff.png
python3 ./plot_pull.py /tmp/benchmark/benchmark.log /tmp/benchmark/pull.png
python3 ./plot_merge.py /tmp/benchmark/benchmark.log /tmp/benchmark/merge.png
python3 ./plot_patch.py /tmp/benchmark/benchmark.log /tmp/benchmark/patch.png
python3 ./plot_file_diff.py /tmp/benchmark/benchmark.log /tmp/benchmark/file_diff.png
python3 ./plot_file_compare.py /tmp/benchmark/compare.log /tmp/benchmark/file_compare.png
python3 ./plot_file_io.py /tmp/benchmark/benchmark-io.log /tmp/benchmark/file_io.png
python3 ./plot_file_io_with_type.py /tmp/benchmark/compare.log /tmp/benchmark/benchmark-io.log /tmp/benchmark/file_io_with_type.png
