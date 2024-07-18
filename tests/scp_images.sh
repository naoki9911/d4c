#!/bin/bash

set -eux

ROOT_DIR=$(cd $(dirname $0)/../; pwd)
BIN_CTR_CLI="$ROOT_DIR/ctr-cli"

PARENT_IMAGE_DIR=$1
REMOTE=$2

function scp_images() {
    IMAGE_DIR=$1
    TEST=$2
    REMOTE=$3
    source ./test_$TEST.sh

    REMOTE_IMAGE_DIR="~/d4c-server/$TEST"

    ssh $REMOTE mkdir -p $REMOTE_IMAGE_DIR

    pushd $IMAGE_DIR

    $BIN_CTR_CLI pack --dimg $IMAGE_LOWER.dimg --config image-$IMAGE_LOWER/config.json --out $IMAGE_LOWER.cdimg
    $BIN_CTR_CLI pack --dimg $IMAGE_MIDDLE.dimg --config image-$IMAGE_MIDDLE/config.json --out $IMAGE_MIDDLE.cdimg
    $BIN_CTR_CLI pack --dimg diff_$IMAGE_LOWER-$IMAGE_MIDDLE-bsdiffx.dimg --config image-$IMAGE_MIDDLE/config.json --out diff_$IMAGE_LOWER-$IMAGE_MIDDLE-bsdiffx.cdimg
    $BIN_CTR_CLI pack --dimg diff_$IMAGE_LOWER-$IMAGE_MIDDLE-xdelta3.dimg --config image-$IMAGE_MIDDLE/config.json --out diff_$IMAGE_LOWER-$IMAGE_MIDDLE-xdelta3.cdimg
    $BIN_CTR_CLI pack --dimg diff_$IMAGE_MIDDLE-$IMAGE_UPPER-bsdiffx.dimg --config image-$IMAGE_UPPER/config.json --out diff_$IMAGE_MIDDLE-$IMAGE_UPPER-bsdiffx.cdimg
    $BIN_CTR_CLI pack --dimg diff_$IMAGE_MIDDLE-$IMAGE_UPPER-xdelta3.dimg --config image-$IMAGE_UPPER/config.json --out diff_$IMAGE_MIDDLE-$IMAGE_UPPER-xdelta3.cdimg
    $BIN_CTR_CLI pack --dimg diff_file_$IMAGE_LOWER-$IMAGE_MIDDLE-bsdiffx.dimg --config image-$IMAGE_MIDDLE/config.json --out diff_file_$IMAGE_LOWER-$IMAGE_MIDDLE-bsdiffx.cdimg
    $BIN_CTR_CLI pack --dimg diff_file_$IMAGE_MIDDLE-$IMAGE_UPPER-bsdiffx.dimg --config image-$IMAGE_UPPER/config.json --out diff_file_$IMAGE_MIDDLE-$IMAGE_UPPER-bsdiffx.cdimg

    scp $IMAGE_LOWER.cdimg $REMOTE:$REMOTE_IMAGE_DIR/.
    scp $IMAGE_MIDDLE.cdimg $REMOTE:$REMOTE_IMAGE_DIR/.
    scp diff_$IMAGE_LOWER-$IMAGE_MIDDLE-bsdiffx.cdimg $REMOTE:$REMOTE_IMAGE_DIR/.
    scp diff_$IMAGE_LOWER-$IMAGE_MIDDLE-xdelta3.cdimg $REMOTE:$REMOTE_IMAGE_DIR/.
    scp diff_$IMAGE_MIDDLE-$IMAGE_UPPER-bsdiffx.cdimg $REMOTE:$REMOTE_IMAGE_DIR/.
    scp diff_$IMAGE_MIDDLE-$IMAGE_UPPER-xdelta3.cdimg $REMOTE:$REMOTE_IMAGE_DIR/.
    scp diff_file_$IMAGE_LOWER-$IMAGE_MIDDLE-bsdiffx.cdimg $REMOTE:$REMOTE_IMAGE_DIR/diff_file_$IMAGE_LOWER-$IMAGE_MIDDLE.cdimg
    scp diff_file_$IMAGE_MIDDLE-$IMAGE_UPPER-bsdiffx.cdimg $REMOTE:$REMOTE_IMAGE_DIR/diff_file_$IMAGE_MIDDLE-$IMAGE_UPPER.cdimg

    popd
}


#TESTS=("nginx" "postgres" "redis" "pytorch")
TESTS=("nginx")
for TEST in "${TESTS[@]}"; do
    scp_images $PARENT_IMAGE_DIR/$TEST $TEST $REMOTE
done
