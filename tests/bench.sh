#!/bin/bash

set -eu
if [ $EUID -ne 0 ]; then
	echo "root previlige required"
	exit 1
fi
RUN_NUM=1

RESULT_DIR=benchmark_`date +%Y-%m-%d-%H%M`
mkdir -p $RESULT_DIR

IMAGES=("apach-root" "mysql-root" "nginx-root" "postgres-root" "redis-root")
for IMAGE in "${IMAGES[@]}"
do
	echo "Benchmarking $IMAGE"
	cd $IMAGE
	rm -f benchmark.log
	ln -sf ../bench_impl.sh ./bench.sh
	./bench.sh $RUN_NUM
	cp benchmark.log ../$RESULT_DIR/$IMAGE-benchmark.log
	cd ../
done

cd $RESULT_DIR
python ../bench-log.py benchmark
python ../bench-agg-di3fs.py di3fs_log.csv > di3fs.csv
python ../bench-agg-patch.py patch_log.csv > patch.csv
python ../bench-agg-merge.py merge_log.csv > merge.csv
python ../bench-agg-diff.py diff_binary_log.csv > diff_binary.csv
python ../bench-agg-diff.py diff_file_log.csv > diff_file.csv
