#!/bin/bash

set -eu
if [ $EUID -ne 0 ]; then
	echo "root previlige required"
	exit 1
fi

RUN_NUM=20
REMOTE_HOST=$1
REMOTE_HOST_ADDR=$2:8081
LOCAL_IMAGE_DIR=$3

PATH=$PATH:/usr/local/go/bin

cd $(cd $(dirname $0); pwd)
pushd ../
make all
popd

RESULT_DIR=benchmark-pull-remote_`date +%Y-%m-%d-%H%M`
mkdir -p $RESULT_DIR

REMOTE_IMAGE_DIR=$(ssh $REMOTE_HOST 'cd ~/d4c-server; pwd')

TESTS=("nginx" "postgres" "redis" "pytorch")
ENCODINGS=("bsdiffx" "xdelta3")
for TEST in "${TESTS[@]}"; do
	for ENCODING in "${ENCODINGS[@]}"; do
		./bench_pull_remote_impl.sh $RESULT_DIR test_$TEST.sh $RUN_NUM $REMOTE_HOST_ADDR $ENCODING $REMOTE_IMAGE_DIR/$TEST $LOCAL_IMAGE_DIR/$TEST
	done
done
