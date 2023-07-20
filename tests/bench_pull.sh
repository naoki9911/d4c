#!/bin/bash

set -eu
#!/bin/bash

set -eu

if [ $EUID -ne 0 ]; then
	echo "root previlige required"
	exit 1
fi

SERVER_HOST="localhost:8081"
RUN_NUM=1

RESULT_DIR=benchmark_`date +%Y-%m-%d-%H%M`
mkdir -p $RESULT_DIR

IMAGES=("apach-root" "mysql-root" "nginx-root" "postgres-root" "redis-root")
for IMAGE in "${IMAGES[@]}"
do
	cd $IMAGE
    sudo rm -f benchmark.log
    ln -sf ../push_impl.sh ./push.sh
    ln -sf ../bench_pull_impl.sh ./bench_pull.sh

    ./push.sh $SERVER_HOST `pwd`
    ./bench_pull.sh $SERVER_HOST $RUN_NUM
    cp benchmark.log ../$RESULT_DIR/$IMAGE-benchmark-pull.log
	cd ../
done

cd $RESULT_DIR
python ../bench-log.py benchmark-pull
python ../bench-agg-pull.py pull_log.csv > pull.csv
python ../bench-agg-pull.py pull_download_log.csv > pull_download.csv
