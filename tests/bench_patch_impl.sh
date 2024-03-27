#!/bin/bash

fusermount3 -u /tmp/fuse

set -eu

ROOT_DIR=$(cd $(dirname $0)/../; pwd)
BIN_CTR_CLI="$ROOT_DIR/ctr-cli"
BIN_DIFF="$ROOT_DIR/diff"
BIN_PACK="$ROOT_DIR/pack"
BIN_FUSE="$ROOT_DIR/fuse-diff"
BIN_MERGE="$ROOT_DIR/merge"

TEST_SCRIPT=$1
IMAGE_DIR=$2
RUN_NUM=$3

source $TEST_SCRIPT

mkdir -p /tmp/fuse

function err() {
    fusermount3 -u /tmp/fuse
    exit 1
}
trap err ERR

IMAGE_DIR=$IMAGE_DIR/$IMAGE_NAME
cd $IMAGE_DIR

for ((i=0; i < $(expr ${#IMAGE_VERSIONS[@]} - 1); i++));do
	LOWER=${IMAGE_VERSIONS[i]}
	UPPER=${IMAGE_VERSIONS[$(expr $i + 1)]}
	DIFF_NAME=$LOWER-$UPPER

	# patching diff data
	for ((j=0; j < $RUN_NUM; j++));do
		NOW_COUNT=$(expr $j + 1)
		echo "Benchmark patch $DIFF_NAME binary-diff ($NOW_COUNT/$RUN_NUM)"
		$BIN_CTR_CLI patch --baseDir=./$LOWER --outDir=./$UPPER-patched --diffDimg=./diff_$DIFF_NAME.dimg --benchmark
	done
	diff -r $UPPER $UPPER-patched --no-dereference

	# mount with di3fs
	for ((j=0; j < $RUN_NUM; j++));do
		NOW_COUNT=$(expr $j + 1)
		echo "Benchmark di3fs $DIFF_NAME binary-diff ($NOW_COUNT/$RUN_NUM)"
		$BIN_FUSE --baseDimg=./$LOWER.dimg --diffDimg=./diff_$DIFF_NAME.dimg --benchmark=true /tmp/fuse >/dev/null 2>&1 &
		sleep 5
		if [ $j -eq 0 ]; then
			diff -r $UPPER /tmp/fuse --no-dereference
		fi
		fusermount3 -u /tmp/fuse
		sleep 2
	done
done

