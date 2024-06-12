#!/bin/bash

set -eu


ROOT_DIR=$(cd $(dirname $0)/../; pwd)
BIN_CTR_CLI="$ROOT_DIR/ctr-cli"
BIN_FUSE="$ROOT_DIR/fuse-diff"

TEST_SCRIPT=$1
IMAGE_DIR=$2
RUN_NUM=$3
THREAD_NUM=${4:-1}
SCHED_MODE=${5:-"none"}
COMP_MODE=$6
DELTA_ENCODING=$7

source $TEST_SCRIPT

LABELS="threadNum:$THREAD_NUM,threadSchedMode:$SCHED_MODE,compressionMode:$COMP_MODE,imageName:$IMAGE_NAME,deltaEncoding:$DELTA_ENCODING"

IMAGE_DIR=$IMAGE_DIR/$IMAGE_NAME
mkdir -p $IMAGE_DIR
cd $IMAGE_DIR

for ((i=0; i < ${#IMAGE_VERSIONS[@]}; i++));do
	IMAGE=${IMAGE_VERSIONS[i]}
	if [ -e ./image-$IMAGE ]; then
		echo "base image for $IMAGE_NAME:$IMAGE already exists"
	else
		echo "Creating base image for $IMAGE_NAME:$IMAGE"
		$BIN_CTR_CLI convert2 --image $DOCKER_IMAGE:$IMAGE --output ./image-$IMAGE --dimg --threadNum 8
		mv ./image-$IMAGE/image.dimg $IMAGE.dimg
	fi
done

for ((i=0; i < $(expr ${#IMAGE_VERSIONS[@]} - 1); i++));do
	LOWER=${IMAGE_VERSIONS[i]}
	UPPER=${IMAGE_VERSIONS[$(expr $i + 1)]}
	DIFF_NAME=$LOWER-$UPPER-$DELTA_ENCODING

	# generating diff data with binary-diff
	for ((j=0; j < $RUN_NUM; j++));do
		NOW_COUNT=$(expr $j + 1)
		echo "Benchmark diff $DIFF_NAME binary-diff ($NOW_COUNT/$RUN_NUM)"
		$BIN_CTR_CLI --labels $LABELS,old:$LOWER,new:$UPPER,mode:binary-diff,out:$LOWER-$UPPER dimg diff --oldDimg=./$LOWER.dimg --newDimg=./$UPPER.dimg --outDimg=./diff_$DIFF_NAME.dimg --mode=binary-diff --benchmark --threadNum $THREAD_NUM --threadSchedMode $SCHED_MODE --compressionMode $COMP_MODE --deltaEncoding $DELTA_ENCODING
	done
done

MERGE_LOWER=$IMAGE_LOWER-$IMAGE_MIDDLE-$DELTA_ENCODING
MERGE_UPPER=$IMAGE_MIDDLE-$IMAGE_UPPER-$DELTA_ENCODING
MERGED=$IMAGE_LOWER-$IMAGE_UPPER-$DELTA_ENCODING
for ((j=0; j < $RUN_NUM; j++));do
	NOW_COUNT=$(expr $j + 1)
	echo "Benchmark merge $MERGE_LOWER and $MERGE_UPPER to $MERGED ($NOW_COUNT/$RUN_NUM)"
	$BIN_CTR_CLI --labels $LABELS,old:$MERGE_LOWER,new:$MERGE_UPPER,mode:binary-diff,out:$IMAGE_LOWER-$IMAGE_UPPER dimg merge --lowerDimg=./diff_$MERGE_LOWER.dimg --upperDimg=./diff_$MERGE_UPPER.dimg --outDimg=./diff_merged_$MERGED.dimg --benchmark --threadNum $THREAD_NUM
done