#!/bin/bash

set -eu


ROOT_DIR=$(cd $(dirname $0)/../; pwd)
BIN_CTR_CLI="$ROOT_DIR/ctr-cli"
BIN_DIFF="$ROOT_DIR/diff"
BIN_PACK="$ROOT_DIR/pack"
BIN_PATCH="$ROOT_DIR/patch"
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
mkdir -p $IMAGE_DIR
cd $IMAGE_DIR

for ((i=0; i < ${#IMAGE_VERSIONS[@]}; i++));do
	IMAGE=${IMAGE_VERSIONS[i]}
	echo "Creating base image for $IMAGE_NAME:$IMAGE"
	$BIN_CTR_CLI convert --image $DOCKER_IMAGE:$IMAGE --output ./image-$IMAGE >/dev/null 2>&1
	mkdir $IMAGE
	sudo tar -xf ./image-$IMAGE/layer.tar -C ./$IMAGE

	# remove '/dev' dir
	sudo rm -rf ./$IMAGE/dev

	$BIN_DIFF "" $IMAGE $IMAGE-base $IMAGE-base.json binary-diff
	$BIN_PACK $IMAGE-base $IMAGE-base.json "" $IMAGE-base.dimg
	$BIN_PATCH dimg "" $IMAGE-base-patched $IMAGE-base.dimg
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
		$BIN_DIFF $LOWER $UPPER diff_$DIFF_NAME diff_$DIFF_NAME.json binary-diff benchmark
	done

	# packing diff data
	$BIN_PACK diff_$DIFF_NAME diff_$DIFF_NAME.json $LOWER-base.dimg diff_$DIFF_NAME.dimg
	ls -l diff_$DIFF_NAME.dimg

	# patching diff data
	for ((j=0; j < $RUN_NUM; j++));do
		NOW_COUNT=$(expr $j + 1)
		echo "Benchmark patch $DIFF_NAME binary-diff ($NOW_COUNT/$RUN_NUM)"
		$BIN_PATCH dimg $LOWER $UPPER-patched diff_$DIFF_NAME.dimg benchmark
	done
	diff -r $UPPER $UPPER-patched --no-dereference

	# mount with di3fs
	for ((j=0; j < $RUN_NUM; j++));do
		NOW_COUNT=$(expr $j + 1)
		echo "Benchmark di3fs $DIFF_NAME binary-diff ($NOW_COUNT/$RUN_NUM)"
		$BIN_FUSE --basedir=./$LOWER-base.dimg --patchdir=./diff_$DIFF_NAME.dimg --mode=dimg --benchmark=true /tmp/fuse >/dev/null 2>&1 &
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
		$BIN_DIFF $LOWER $UPPER diff_file_$DIFF_NAME diff_file_$DIFF_NAME.json file-diff benchmark
	done

	# packing diff data and test it
	echo "Testing packed $DIFF_NAME file-diff"
	$BIN_PACK diff_file_$DIFF_NAME diff_file_$DIFF_NAME.json $LOWER-base.dimg diff_file_$DIFF_NAME.dimg
	ls -l diff_file_$DIFF_NAME.dimg
	$BIN_PATCH dimg $LOWER $UPPER-patched diff_file_$DIFF_NAME.dimg
	diff -r $UPPER $UPPER-patched --no-dereference
done

MERGE_LOWER=$IMAGE_LOWER-$IMAGE_MIDDLE
MERGE_UPPER=$IMAGE_MIDDLE-$IMAGE_UPPER
MERGED=$IMAGE_LOWER-$IMAGE_UPPER
for ((j=0; j < $RUN_NUM; j++));do
	NOW_COUNT=$(expr $j + 1)
	echo "Benchmark merge $MERGE_LOWER and $MERGE_UPPER to $MERGED ($NOW_COUNT/$RUN_NUM)"
	$BIN_MERGE dimg diff_$MERGE_LOWER.dimg diff_$MERGE_UPPER.dimg diff_merged_$MERGED.dimg benchmark
done

echo "Testing merged $MERGED"
$BIN_PATCH dimg $IMAGE_LOWER $IMAGE_UPPER-merged diff_merged_$MERGED.dimg
diff -r $IMAGE_UPPER $IMAGE_UPPER-merged --no-dereference
ls -l diff_merged_$MERGED.dimg
$BIN_FUSE --basedir=./$IMAGE_LOWER-base.dimg --patchdir=./diff_merged_$MERGED.dimg --mode=dimg /tmp/fuse >/dev/null 2>&1 &
sleep 1
diff -r $IMAGE_UPPER /tmp/fuse --no-dereference
fusermount3 -u /tmp/fuse

for ((j=0; j < $RUN_NUM; j++));do
	NOW_COUNT=$(expr $j + 1)
	echo "Benchmark regen-diff $MERGED binary-diff ($NOW_COUNT/$RUN_NUM)"
	$BIN_DIFF $IMAGE_LOWER $IMAGE_UPPER diff_$MERGED diff_$MERGED.json binary-diff benchmark
done
$BIN_PACK diff_$MERGED diff_$MERGED.json $IMAGE_LOWER-base.dimg diff_$MERGED.dimg
ls -l diff_$MERGED.dimg

for ((j=0; j < $RUN_NUM; j++));do
	NOW_COUNT=$(expr $j + 1)
	echo "Benchmark regen-diff $MERGED file-diff ($NOW_COUNT/$RUN_NUM)"
	$BIN_DIFF $IMAGE_LOWER $IMAGE_UPPER diff_file_$MERGED diff_file_$MERGED.json file-diff benchmark
done
$BIN_PACK diff_file_$MERGED diff_file_$MERGED.json $IMAGE_LOWER-base.dimg diff_file_$MERGED.dimg
ls -l diff_file_$MERGED.dimg

echo "Benchmark done"
