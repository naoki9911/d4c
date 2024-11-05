#!/bin/bash
set -eux

RESULT_DIR=benchmark_`date +%Y-%m-%d-%H%M`
mkdir $RESULT_DIR
cd $RESULT_DIR

BIN_CTR_CLI="../../../ctr-cli"
BIN_FUSE="../../../fuse-diff"
IMAGE_DIR="../../benchmark_2024-06-17-0554/images"
THREAD_NUM=1

IMAGE=("postgres" "nginx" "redis" "pytorch")
LOWER=("13.1-13.2" "1.23.1-1.23.2" "7.0.5-7.0.6" "2.2.0-cuda12.1-cudnn8-runtime-2.2.1-cuda12.1-cudnn8-runtime")
UPPER=("13.2-13.3" "1.23.2-1.23.3" "7.0.6-7.0.7" "2.2.1-cuda12.1-cudnn8-runtime-2.2.2-cuda12.1-cudnn8-runtime")
MERGED=("13.1-13.3" "1.23.1-1.23.3" "7.0.5-7.0.7" "2.2.0-cuda12.1-cudnn8-runtime-2.2.2-cuda12.1-cudnn8-runtime")
for i in $(seq 1 10); do
    for ix in ${!IMAGE[@]}; do
        TARGET_IMAGE=${IMAGE[ix]}
        TARGET_IMAGE_DIR="$IMAGE_DIR/$TARGET_IMAGE"
        LOWER_IMAGE=diff_${LOWER[ix]}-bsdiffx.dimg
        UPPER_IMAGE=diff_${UPPER[ix]}-bsdiffx.dimg
        MERGED_IMAGE=diff_${MERGED[ix]}-bsdiffx-merged.dimg
        LABELS="count:$i,image:$TARGET_IMAGE,comporessionMode:bzip2,deltaEncoding:bsdiffx,old:$LOWER_IMAGE,new:$UPPER_IMAGE,out:$MERGED_IMAGE"

        # dry run
        $BIN_CTR_CLI --labels $LABELS dimg merge --lowerDimg=$TARGET_IMAGE_DIR/$LOWER_IMAGE --upperDimg=$TARGET_IMAGE_DIR/$UPPER_IMAGE --outDimg=$MERGED_IMAGE --threadNum=$THREAD_NUM
        $BIN_CTR_CLI --labels $LABELS dimg merge --lowerDimg=$TARGET_IMAGE_DIR/$LOWER_IMAGE --upperDimg=$TARGET_IMAGE_DIR/$UPPER_IMAGE --outDimg=$MERGED_IMAGE --benchmarkPerFile --threadNum $THREAD_NUM
        $BIN_CTR_CLI --labels $LABELS dimg merge --lowerDimg=$TARGET_IMAGE_DIR/$LOWER_IMAGE --upperDimg=$TARGET_IMAGE_DIR/$UPPER_IMAGE --outDimg=$MERGED_IMAGE --mergeBreakdown --threadNum $THREAD_NUM
    done
done