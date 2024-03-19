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
for TEST in "${TESTS[@]}"
do
	echo "Benchmarking $TEST"
	./bench_impl.sh test_$TEST.sh $IMAGE_DIR $RUN_NUM
	cp $IMAGE_DIR/$TEST/benchmark.log ./$RESULT_DIR/$TEST-benchmark.log
done

cd $RESULT_DIR
python3 ../bench-log.py benchmark
python3 ../bench-agg-di3fs.py di3fs_log.csv > di3fs.csv
python3 ../bench-agg-patch.py patch_log.csv > patch.csv
python3 ../bench-agg-merge.py merge_log.csv > merge.csv
python3 ../bench-agg-diff.py diff_binary_log.csv > diff_binary.csv
python3 ../bench-agg-diff.py diff_file_log.csv > diff_file.csv
