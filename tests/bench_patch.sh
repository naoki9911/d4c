#!/bin/bash

set -eu
if [ $EUID -ne 0 ]; then
	echo "root previlige required"
	exit 1
fi

RESULT_DIR=benchmark_`date +%Y-%m-%d-%H%M`
mkdir -p $RESULT_DIR

RUN_NUM=1
PATH=$PATH:"/usr/local/go/bin"

IMAGES=("apach-root" "mysql-root" "nginx-root" "postgres-root" "redis-root")
for IMAGE in "${IMAGES[@]}"
do
	echo "Benchmarking $IMAGE"
	cd $IMAGE
	rm -f benchmark.log
	ln -sf ../bench_patch_impl.sh ./bench_patch.sh
	./bench_patch.sh $RUN_NUM
	cp benchmark.log ../$RESULT_DIR/$IMAGE-benchmark-patch.log
	cd ../
done

cd $RESULT_DIR
python ../bench-log.py benchmark-patch
python ../bench-agg-di3fs.py di3fs_log.csv > di3fs.csv
python ../bench-agg-patch.py patch_log.csv > patch.csv
