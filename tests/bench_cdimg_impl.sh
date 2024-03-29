#!/bin/bash

systemctl stop d4c-server
systemctl stop d4c-snapshotter
set -eu

ROOT_DIR=$(cd $(dirname $0)/../; pwd)
BIN_CTR_CLI="$ROOT_DIR/ctr-cli"
BIN_FUSE="$ROOT_DIR/fuse-diff"
BIN_SNAPSHOTTER="$ROOT_DIR/snapshotter"

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
	$BIN_CTR_CLI convert --image $DOCKER_IMAGE:$IMAGE --output ./image-$IMAGE --cdimg --excludes /dev >/dev/null 2>&1
	mkdir $IMAGE
	tar -xf ./image-$IMAGE/layer.tar -C ./$IMAGE
	rm -rf ./$IMAGE/dev

	mv ./image-$IMAGE/image.cdimg $IMAGE.cdimg
	$BIN_CTR_CLI cdimg patch --outDir=./$IMAGE-base-patched --diffCdimg=./$IMAGE.cdimg
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
		$BIN_CTR_CLI cdimg diff --oldCdimg=./$LOWER.cdimg --newCdimg=./$UPPER.cdimg --outCdimg=./diff_$DIFF_NAME.cdimg --mode=binary-diff --benchmark
	done

	# packing diff data
	ls -l diff_$DIFF_NAME.cdimg

	# patching diff data
	for ((j=0; j < $RUN_NUM; j++));do
		NOW_COUNT=$(expr $j + 1)
		echo "Benchmark patch $DIFF_NAME binary-diff ($NOW_COUNT/$RUN_NUM)"
		$BIN_CTR_CLI cdimg patch --baseDir=./$LOWER --outDir=./$UPPER-patched --diffCdimg=./diff_$DIFF_NAME.cdimg --benchmark
	done
	diff -r $UPPER $UPPER-patched --no-dereference

	# mount with di3fs
	for ((j=0; j < $RUN_NUM; j++));do
		NOW_COUNT=$(expr $j + 1)
		echo "Benchmark di3fs $DIFF_NAME binary-diff ($NOW_COUNT/$RUN_NUM)"
		$BIN_FUSE --parentCdimg=./$LOWER.cdimg --diffCdimg=./diff_$DIFF_NAME.cdimg --benchmark=true /tmp/fuse >/dev/null 2>&1 &
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
		$BIN_CTR_CLI cdimg diff --oldCdimg=./$LOWER.cdimg --newCdimg=./$UPPER.cdimg --outCdimg=./diff_file_$DIFF_NAME.cdimg --mode=file-diff --benchmark
	done

	# packing diff data and test it
	ls -l diff_file_$DIFF_NAME.cdimg
	$BIN_CTR_CLI cdimg patch --baseDir=./$LOWER --outDir=./$UPPER-patched --diffCdimg=./diff_file_$DIFF_NAME.cdimg
	diff -r $UPPER $UPPER-patched --no-dereference
done

MERGE_LOWER=$IMAGE_LOWER-$IMAGE_MIDDLE
MERGE_UPPER=$IMAGE_MIDDLE-$IMAGE_UPPER
MERGED=$IMAGE_LOWER-$IMAGE_UPPER
for ((j=0; j < $RUN_NUM; j++));do
	NOW_COUNT=$(expr $j + 1)
	echo "Benchmark merge $MERGE_LOWER and $MERGE_UPPER to $MERGED ($NOW_COUNT/$RUN_NUM)"
	$BIN_CTR_CLI cdimg merge --lowerCdimg=./diff_$MERGE_LOWER.cdimg --upperCdimg=./diff_$MERGE_UPPER.cdimg --outCdimg=./diff_merged_$MERGED.cdimg --benchmark
done

echo "Testing merged $MERGED"
$BIN_CTR_CLI cdimg patch --baseDir=./$IMAGE_LOWER --outDir=./$IMAGE_UPPER-merged --diffCdimg=./diff_merged_$MERGED.cdimg
diff -r $IMAGE_UPPER $IMAGE_UPPER-merged --no-dereference
ls -l diff_merged_$MERGED.cdimg
$BIN_FUSE --parentCdimg=./$IMAGE_LOWER.cdimg --diffCdimg=./diff_merged_$MERGED.cdimg /tmp/fuse >/dev/null 2>&1 &
sleep 1
diff -r $IMAGE_UPPER /tmp/fuse --no-dereference
fusermount3 -u /tmp/fuse

for ((j=0; j < $RUN_NUM; j++));do
	NOW_COUNT=$(expr $j + 1)
	echo "Benchmark regen-diff $MERGED binary-diff ($NOW_COUNT/$RUN_NUM)"
	$BIN_CTR_CLI cdimg diff --oldCdimg=./$IMAGE_LOWER.cdimg --newCdimg=./$IMAGE_UPPER.cdimg --outCdimg=./diff_$MERGED.cdimg --mode=binary-diff --benchmark
done
ls -l diff_$MERGED.cdimg

for ((j=0; j < $RUN_NUM; j++));do
	NOW_COUNT=$(expr $j + 1)
	echo "Benchmark regen-diff $MERGED file-diff ($NOW_COUNT/$RUN_NUM)"
	$BIN_CTR_CLI cdimg diff --oldCdimg=./$IMAGE_LOWER.cdimg --newCdimg=./$IMAGE_UPPER.cdimg --outCdimg=./diff_file_$MERGED.cdimg --mode=file-diff --benchmark
done
ls -l diff_file_$MERGED.cdimg

echo "Benchmark done"

ctr image rm $IMAGE_NAME:$IMAGE_LOWER
ctr image rm $IMAGE_NAME:$IMAGE_MIDDLE
ctr image rm $IMAGE_NAME:$IMAGE_UPPER
ctr image rm $IMAGE_NAME-file:$IMAGE_LOWER
ctr image rm $IMAGE_NAME-file:$IMAGE_MIDDLE
ctr image rm $IMAGE_NAME-file:$IMAGE_UPPER

systemd-run --unit=d4c-snapshotter $BIN_SNAPSHOTTER
systemctl restart containerd
sleep 2

$BIN_CTR_CLI load --image=$IMAGE_NAME:$IMAGE_LOWER --cdimg=./$IMAGE_LOWER.cdimg
$BIN_CTR_CLI load --image=$IMAGE_NAME:$IMAGE_MIDDLE --cdimg=./diff_$IMAGE_LOWER-$IMAGE_MIDDLE.cdimg
$BIN_CTR_CLI load --image=$IMAGE_NAME:$IMAGE_UPPER --cdimg=./diff_$IMAGE_MIDDLE-$IMAGE_UPPER.cdimg

$BIN_CTR_CLI load --image=$IMAGE_NAME-file:$IMAGE_LOWER --cdimg=./$IMAGE_LOWER.cdimg
$BIN_CTR_CLI load --image=$IMAGE_NAME-file:$IMAGE_MIDDLE --cdimg=./diff_file_$IMAGE_LOWER-$IMAGE_MIDDLE.cdimg
$BIN_CTR_CLI load --image=$IMAGE_NAME-file:$IMAGE_UPPER --cdimg=./diff_file_$IMAGE_MIDDLE-$IMAGE_UPPER.cdimg

sleep 1
ctr snapshot --snapshotter=di3fs tree | while read SNP; do 
    SNP_IMAGE_TAG=$(ctr snapshot --snapshotter=di3fs info $SNP | jq -r '.Labels."containerd.io/snapshot/di3fs.image.name"')
    MOUNT_PATH=$(ctr snapshot --snapshotter=di3fs info $SNP | jq -r '.Labels."containerd.io/snapshot/di3fs.mount"')
    IMAGE_TAG=(${SNP_IMAGE_TAG//:/ })
    SNP_IMAGE_NAME=${IMAGE_TAG[0]}
    SNP_IMAGE_NAME=(${SNP_IMAGE_NAME//-/ })
    SNP_IMAGE_NAME=${SNP_IMAGE_NAME[0]}
    SNP_IMAGE_VERSION=${IMAGE_TAG[1]}

    if [ "$SNP_IMAGE_NAME" == "$IMAGE_NAME" ]; then
        echo "Checking $SNP_IMAGE_TAG at $MOUNT_PATH"
        sudo diff -r $SNP_IMAGE_VERSION $MOUNT_PATH --no-dereference
    fi
done

systemctl stop d4c-snapshotter
