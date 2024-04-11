#!/bin/bash

fusermount3 -u /tmp/fuse
cd $(cd $(dirname $0); pwd)

set -eu

TEST_SCRIPT=$1
RUN_NUM=10

source $TEST_SCRIPT

mkdir -p /tmp/fuse

function err() {
    fusermount3 -u /tmp/fuse
    exit 1
}

THREAD_NUM=8

IMAGE_DIR="merge_images"
IMAGE_DIR=$IMAGE_DIR/$IMAGE_NAME
mkdir -p $IMAGE_DIR
IMAGE_DIR=$(cd $IMAGE_DIR; pwd)

ROOT_DIR=$(cd $(dirname $0)/../; pwd)
BIN_CTR_CLI="$ROOT_DIR/ctr-cli"
BIN_FUSE="$ROOT_DIR/fuse-diff"
BIN_SNAPSHOTTER="$ROOT_DIR/snapshotter"

function convert_image() {
    IMAGE_VERSION=$1
    EXTRACT=$2
    if [ -e $IMAGE_DIR/$IMAGE_VERSION.cdimg ]; then
        return
    fi
    $BIN_CTR_CLI convert --image $IMAGE_NAME:$IMAGE_VERSION --output $IMAGE_DIR/$IMAGE_VERSION --cdimg --excludes /dev --threadNum $THREAD_NUM
    mv $IMAGE_DIR/$IMAGE_VERSION/image.cdimg $IMAGE_DIR/$IMAGE_VERSION.cdimg
    if [ "$EXTRACT" == "true" ]; then
        mkdir $IMAGE_DIR/image-$IMAGE_VERSION
        tar -xf $IMAGE_DIR/$IMAGE_VERSION/layer.tar -C $IMAGE_DIR/image-$IMAGE_VERSION
        rm -rf $IMAGE_DIR/image-$IMAGE_VERSION/dev
    else
        rm -rf $IMAGE_DIR/$IMAGE_VERSION
    fi
}

function diff_image() {
    LOWER=$1
    UPPER=$2
    DIFF_NAME=$IMAGE_DIR/$LOWER-$UPPER.cdimg
    if [ -e $DIFF_NAME ]; then
        echo $DIFF_NAME
        return
    fi

    $BIN_CTR_CLI cdimg diff --oldCdimg $IMAGE_DIR/$LOWER.cdimg --newCdimg $IMAGE_DIR/$UPPER.cdimg --outCdimg $DIFF_NAME --threadNum $THREAD_NUM
    echo $DIFF_NAME
}

for ((i=0; i < ${#IMAGE_MORE_VERSIONS[@]}; i++));do
    VERSION=${IMAGE_MORE_VERSIONS[i]}
    if [ $i -eq $(expr ${#IMAGE_MORE_VERSIONS[@]} - 1) ]; then
        convert_image $VERSION true
    else
        convert_image $VERSION false
    fi
done


MERGED_CDIMGS=""
for ((i=0; i < $(expr ${#IMAGE_MORE_VERSIONS[@]} - 1); i++));do
    LOWER=${IMAGE_MORE_VERSIONS[i]}
    UPPER=${IMAGE_MORE_VERSIONS[$(expr $i + 1)]}
    DIFF_CDIMG_NAME=$(diff_image $LOWER $UPPER)
    if [ $i -eq 0 ]; then
        MERGED_CDIMGS=$DIFF_CDIMG_NAME
    else
        MERGED_CDIMGS=$MERGED_CDIMGS,$DIFF_CDIMG_NAME
    fi
done

echo $MERGED_CDIMGS

MOST_LOWER=${IMAGE_MORE_VERSIONS[0]}
MOST_UPPER=${IMAGE_MORE_VERSIONS[$(expr ${#IMAGE_MORE_VERSIONS[@]} - 1)]}

$BIN_CTR_CLI cdimg merge --cdimgs $MERGED_CDIMGS --outCdimg $IMAGE_DIR/$MOST_LOWER-$MOST_UPPER.cdimg --threadNum 8 --mergeDimgConcurrentNum 4 --mergeMode bisect
$BIN_FUSE --daemon --parentCdimg $IMAGE_DIR/$MOST_LOWER.cdimg --diffCdimg $IMAGE_DIR/$MOST_LOWER-$MOST_UPPER.cdimg /tmp/fuse
diff -r --no-dereference $IMAGE_DIR/image-$MOST_UPPER /tmp/fuse

CONCURRENCY=(1 4 8)
MERGE_MODES=("linear" "bisect")
for ((j=0; j < $RUN_NUM; j++)); do
    echo "Benchmark merge $j/$RUN_NUM"
    for MERGE_MODE in "${MERGE_MODES[@]}"; do
        if [ "$MERGE_MODE" == "bisect" ]; then
            for CONCURRENT in "${CONCURRENCY[@]}"; do
                $BIN_CTR_CLI cdimg merge --cdimgs $MERGED_CDIMGS --outCdimg $IMAGE_DIR/$MOST_LOWER-$MOST_UPPER.cdimg --threadNum 8 --mergeDimgConcurrentNum $CONCURRENT --mergeMode $MERGE_MODE --benchmark
            done
        else
            $BIN_CTR_CLI cdimg merge --cdimgs $MERGED_CDIMGS --outCdimg $IMAGE_DIR/$MOST_LOWER-$MOST_UPPER.cdimg --threadNum 8 --mergeDimgConcurrentNum 4 --mergeMode $MERGE_MODE --benchmark
        fi
    done
done
