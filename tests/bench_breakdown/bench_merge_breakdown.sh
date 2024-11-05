#!/bin/bash
set -eux

BIN_CTR_CLI="../../ctr-cli"
BIN_FUSE="../../fuse-diff"

IMAGE_DIR="../benchmark_2024-06-17-0554/images"
TARGET_IMAGE="postgres"
TARGET_IMAGE_DIR="$IMAGE_DIR/$TARGET_IMAGE"

LOWER_IMAGE="diff_13.1-13.2-bsdiffx.dimg"
UPPER_IMAGE="diff_13.2-13.3-bsdiffx.dimg"
MERGED_IMAGE="diff_13.1-13.3-bsdiffx-merged.dimg"
LABELS="comporessionMode:bzip2,deltaEncoding:bsdiffx,old:$LOWER_IMAGE,new:$UPPER_IMAGE,out:$MERGED_IMAGE"
THREAD_NUM=1

# dry run
$BIN_CTR_CLI --labels $LABELS dimg merge --lowerDimg=$TARGET_IMAGE_DIR/$LOWER_IMAGE --upperDimg=$TARGET_IMAGE_DIR/$UPPER_IMAGE --outDimg=$MERGED_IMAGE --threadNum=$THREAD_NUM
$BIN_CTR_CLI --labels $LABELS dimg merge --lowerDimg=$TARGET_IMAGE_DIR/$LOWER_IMAGE --upperDimg=$TARGET_IMAGE_DIR/$UPPER_IMAGE --outDimg=$MERGED_IMAGE --benchmarkPerFile --threadNum $THREAD_NUM
$BIN_CTR_CLI --labels $LABELS dimg merge --lowerDimg=$TARGET_IMAGE_DIR/$LOWER_IMAGE --upperDimg=$TARGET_IMAGE_DIR/$UPPER_IMAGE --outDimg=$MERGED_IMAGE --mergeBreakdown --threadNum $THREAD_NUM


# dry run
$BIN_CTR_CLI --labels $LABELS dimg diff --oldDimg=$TARGET_IMAGE_DIR/13.1.dimg --newDimg=$TARGET_IMAGE_DIR/13.3.dimg --outDimg=./diff_13.1-13.3-bsdiffx.dimg --mode=binary-diff --threadNum=$THREAD_NUM 
$BIN_CTR_CLI --labels $LABELS dimg diff --oldDimg=$TARGET_IMAGE_DIR/13.1.dimg --newDimg=$TARGET_IMAGE_DIR/13.3.dimg --outDimg=./diff_13.1-13.3-bsdiffx.dimg --mode=binary-diff --threadNum=$THREAD_NUM --diffBreakdown
