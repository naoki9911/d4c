#!/bin/bash
fusermount3 -u /tmp/fuse

set -eux

BIN_CTR_CLI="../ctr-cli"
BIN_FUSE="../fuse-diff"

IMAGE_DIR="./benchmark_2024-06-17-0554/images"
TARGET_IMAGE="postgres"
TARGET_IMAGE_DIR="$IMAGE_DIR/$TARGET_IMAGE"

LABELS="comporessionMode:bzip2,deltaEncoding:bsdiffx"

rm -rf /tmp/fuse
mkdir /tmp/fuse

$BIN_FUSE --label=$LABELS --daemon --parentDimg=$TARGET_IMAGE_DIR/13.1.dimg --diffDimg=$TARGET_IMAGE_DIR/diff_13.1-13.2-bsdiffx.dimg --benchmarkOpen=true /tmp/fuse

# ensure that page cache does not affect results.
echo 3 | sudo tee /proc/sys/vm/drop_caches

$BIN_CTR_CLI --labels $LABELS stat diff --benchmark --pathALabel native --pathBLabel di3fs --count 1 $TARGET_IMAGE_DIR/13.2 /tmp/fuse

fusermount3 -u /tmp/fuse
