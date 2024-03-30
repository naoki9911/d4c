#!/bin/bash

set -eu


ROOT_DIR=$(cd $(dirname $0)/../; pwd)
BIN_CTR_CLI="$ROOT_DIR/ctr-cli"
BIN_FUSE="$ROOT_DIR/fuse-diff"

TEST_SCRIPT=$1
IMAGE_DIR=$2
RUN_NUM=$3
THREAD_NUM=${4:-1}

source $TEST_SCRIPT

mkdir -p /tmp/fuse

function err() {
    fusermount3 -u /tmp/fuse
    exit 1
}

trap err ERR

IMAGE_DIR=$IMAGE_DIR/$IMAGE_NAME
mkdir -p $IMAGE_DIR
cd $IMAGE_DIR

for ((i=0; i < ${#IMAGE_VERSIONS[@]}; i++));do
	IMAGE=${IMAGE_VERSIONS[i]}
	echo "Creating base image for $IMAGE_NAME:$IMAGE"
	$BIN_CTR_CLI convert --image $DOCKER_IMAGE:$IMAGE --output ./image-$IMAGE --dimg --excludes /dev --threadNum $THREAD_NUM
	mkdir $IMAGE
	tar -xf ./image-$IMAGE/layer.tar -C ./$IMAGE
	rm -rf ./$IMAGE/dev

	mv ./image-$IMAGE/image.dimg $IMAGE.dimg
	$BIN_CTR_CLI dimg patch --outDir=./$IMAGE-base-patched --diffDimg=./$IMAGE.dimg
	diff -r $IMAGE $IMAGE-base-patched --no-dereference
done

for ((i=0; i < $(expr ${#IMAGE_VERSIONS[@]} - 1); i++));do
	LOWER=${IMAGE_VERSIONS[i]}
	UPPER=${IMAGE_VERSIONS[$(expr $i + 1)]}
	DIFF_NAME=$LOWER-$UPPER

	# generating diff data with binary-diff
	for ((j=0; j < $RUN_NUM; j++));do
		NOW_COUNT=$(expr $j + 1)
		echo "Benchmark diff $DIFF_NAME binary-diff ($NOW_COUNT/$RUN_NUM)"
		$BIN_CTR_CLI dimg diff --oldDimg=./$LOWER.dimg --newDimg=./$UPPER.dimg --outDimg=./diff_$DIFF_NAME.dimg --mode=binary-diff --benchmark --threadNum $THREAD_NUM
	done

	# packing diff data
	ls -l diff_$DIFF_NAME.dimg

	# patching diff data
	for ((j=0; j < $RUN_NUM; j++));do
		NOW_COUNT=$(expr $j + 1)
		echo "Benchmark patch $DIFF_NAME binary-diff ($NOW_COUNT/$RUN_NUM)"
		$BIN_CTR_CLI dimg patch --baseDir=./$LOWER --outDir=./$UPPER-patched --diffDimg=./diff_$DIFF_NAME.dimg --benchmark
	done
	diff -r $UPPER $UPPER-patched --no-dereference

	# mount with di3fs
	for ((j=0; j < $RUN_NUM; j++));do
		NOW_COUNT=$(expr $j + 1)
		echo "Benchmark di3fs $DIFF_NAME binary-diff ($NOW_COUNT/$RUN_NUM)"
		$BIN_FUSE --parentDimg=./$LOWER.dimg --diffDimg=./diff_$DIFF_NAME.dimg --benchmark=true /tmp/fuse >/dev/null 2>&1 &
		sleep 1
		if [ $j -eq 0 ]; then
			diff -r $UPPER /tmp/fuse --no-dereference
		fi
		fusermount3 -u /tmp/fuse
	done

	# generating diff data with file-dff
	for ((j=0; j < $RUN_NUM; j++));do
		NOW_COUNT=$(expr $j + 1)
		echo "Benchmark diff $DIFF_NAME file-diff ($NOW_COUNT/$RUN_NUM)"
		$BIN_CTR_CLI dimg diff --oldDimg=./$LOWER.dimg --newDimg=./$UPPER.dimg --outDimg=./diff_file_$DIFF_NAME.dimg --mode=file-diff --benchmark --threadNum $THREAD_NUM
	done

	# packing diff data and test it
	ls -l diff_file_$DIFF_NAME.dimg
	$BIN_CTR_CLI dimg patch --baseDir=./$LOWER --outDir=./$UPPER-patched --diffDimg=./diff_file_$DIFF_NAME.dimg
	diff -r $UPPER $UPPER-patched --no-dereference
done

MERGE_LOWER=$IMAGE_LOWER-$IMAGE_MIDDLE
MERGE_UPPER=$IMAGE_MIDDLE-$IMAGE_UPPER
MERGED=$IMAGE_LOWER-$IMAGE_UPPER
for ((j=0; j < $RUN_NUM; j++));do
	NOW_COUNT=$(expr $j + 1)
	echo "Benchmark merge $MERGE_LOWER and $MERGE_UPPER to $MERGED ($NOW_COUNT/$RUN_NUM)"
	$BIN_CTR_CLI dimg merge --lowerDimg=./diff_$MERGE_LOWER.dimg --upperDimg=./diff_$MERGE_UPPER.dimg --outDimg=./diff_merged_$MERGED.dimg --benchmark
done

echo "Testing merged $MERGED"
$BIN_CTR_CLI dimg patch --baseDir=./$IMAGE_LOWER --outDir=./$IMAGE_UPPER-merged --diffDimg=./diff_merged_$MERGED.dimg
diff -r $IMAGE_UPPER $IMAGE_UPPER-merged --no-dereference
ls -l diff_merged_$MERGED.dimg
$BIN_FUSE --parentDimg=./$IMAGE_LOWER.dimg --diffDimg=./diff_merged_$MERGED.dimg /tmp/fuse >/dev/null 2>&1 &
sleep 1
diff -r $IMAGE_UPPER /tmp/fuse --no-dereference
fusermount3 -u /tmp/fuse

for ((j=0; j < $RUN_NUM; j++));do
	NOW_COUNT=$(expr $j + 1)
	echo "Benchmark regen-diff $MERGED binary-diff ($NOW_COUNT/$RUN_NUM)"
	$BIN_CTR_CLI dimg diff --oldDimg=./$IMAGE_LOWER.dimg --newDimg=./$IMAGE_UPPER.dimg --outDimg=./diff_$MERGED.dimg --mode=binary-diff --benchmark --threadNum $THREAD_NUM
done
ls -l diff_$MERGED.dimg

for ((j=0; j < $RUN_NUM; j++));do
	NOW_COUNT=$(expr $j + 1)
	echo "Benchmark regen-diff $MERGED file-diff ($NOW_COUNT/$RUN_NUM)"
	$BIN_CTR_CLI dimg diff --oldDimg=./$IMAGE_LOWER.dimg --newDimg=./$IMAGE_UPPER.dimg --outDimg=./diff_file_$MERGED.dimg --mode=file-diff --benchmark --threadNum $THREAD_NUM
done
ls -l diff_file_$MERGED.dimg

echo "Benchmark done"
