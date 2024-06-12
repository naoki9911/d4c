#!/bin/bash

systemctl stop d4c-server
systemctl stop d4c-snapshotter
systemctl reset-failed

set -eu

THREAD_NUM=8
SERVER_HOST="localhost:8081"
IMAGE_DIR="merge_images"
IMAGE_DIR=$(cd $IMAGE_DIR; pwd)
mkdir -p $IMAGE_DIR

ROOT_DIR=$(cd $(dirname $0)/../; pwd)
BIN_CTR_CLI="$ROOT_DIR/ctr-cli"
BIN_SERVER="$ROOT_DIR/server"
BIN_SNAPSHOTTER="$ROOT_DIR/snapshotter"

systemd-run --unit=d4c-server $BIN_SERVER --threadNum $THREAD_NUM
systemd-run --unit=d4c-snapshotter $BIN_SNAPSHOTTER
systemctl restart containerd

function convert_image() {
    IMAGE_VERSION=$1
    if [ -e $IMAGE_DIR/$IMAGE_VERSION.cdimg ]; then
        return
    fi
    $BIN_CTR_CLI convert --image nginx:$IMAGE_VERSION --output $IMAGE_DIR/$IMAGE_VERSION --cdimg --threadNum $THREAD_NUM
    mv $IMAGE_DIR/$IMAGE_VERSION/image.cdimg $IMAGE_DIR/$IMAGE_VERSION.cdimg
    mkdir $IMAGE_DIR/image-$IMAGE_VERSION
    tar -xf $IMAGE_DIR/$IMAGE_VERSION/layer.tar -C $IMAGE_DIR/image-$IMAGE_VERSION
    cd $IMAGE_DIR/image-$IMAGE_VERSION
    rm -f $(find -name .wh..wh..opq)
    rm -f $(find . -type b)
    rm -f $(find . -type c)
    rm -f $(find . -type p)
    rm -f $(find . -type s)
    cd ../../
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

convert_image 1.20.1
convert_image 1.20.2
convert_image 1.21.0
convert_image 1.21.1
convert_image 1.21.3
convert_image 1.21.4
convert_image 1.21.5
convert_image 1.21.6
convert_image 1.22.0
convert_image 1.22.1
convert_image 1.23.0
convert_image 1.23.1
convert_image 1.23.2
convert_image 1.23.3
convert_image 1.23.4
convert_image 1.24.0
convert_image 1.25.0
convert_image 1.25.1
convert_image 1.25.2
convert_image 1.25.3
convert_image 1.25.4

RUN_NUM=2

MERGED_CDIMGS=$(diff_image 1.20.1 1.20.2)
MERGED_CDIMGS=$MERGED_CDIMGS,$(diff_image 1.20.2 1.21.0)
MERGED_CDIMGS=$MERGED_CDIMGS,$(diff_image 1.21.0 1.21.1)
MERGED_CDIMGS=$MERGED_CDIMGS,$(diff_image 1.21.1 1.21.3) # 4
for ((j=0; j < $RUN_NUM; j++)); do
    $BIN_CTR_CLI cdimg merge --cdimgs $MERGED_CDIMGS --outCdimg $IMAGE_DIR/merged.cdimg --threadNum 8 --mergeDimgConcurrentNum 4 --mergeMode linear --benchmark
    $BIN_CTR_CLI cdimg merge --cdimgs $MERGED_CDIMGS --outCdimg $IMAGE_DIR/merged.cdimg --threadNum 8 --mergeDimgConcurrentNum 4 --mergeMode bisect --benchmark
done
MERGED_CDIMGS=$MERGED_CDIMGS,$(diff_image 1.21.3 1.21.4)
MERGED_CDIMGS=$MERGED_CDIMGS,$(diff_image 1.21.4 1.21.5)
MERGED_CDIMGS=$MERGED_CDIMGS,$(diff_image 1.21.5 1.21.6)
MERGED_CDIMGS=$MERGED_CDIMGS,$(diff_image 1.21.6 1.22.0) #8
for ((j=0; j < $RUN_NUM; j++)); do
    $BIN_CTR_CLI cdimg merge --cdimgs $MERGED_CDIMGS --outCdimg $IMAGE_DIR/merged.cdimg --threadNum 8 --mergeDimgConcurrentNum 4 --mergeMode linear --benchmark
    $BIN_CTR_CLI cdimg merge --cdimgs $MERGED_CDIMGS --outCdimg $IMAGE_DIR/merged.cdimg --threadNum 8 --mergeDimgConcurrentNum 4 --mergeMode bisect --benchmark
done
MERGED_CDIMGS=$MERGED_CDIMGS,$(diff_image 1.22.0 1.22.1)
MERGED_CDIMGS=$MERGED_CDIMGS,$(diff_image 1.22.1 1.23.0)
MERGED_CDIMGS=$MERGED_CDIMGS,$(diff_image 1.23.0 1.23.1)
MERGED_CDIMGS=$MERGED_CDIMGS,$(diff_image 1.23.1 1.23.2) # 12
for ((j=0; j < $RUN_NUM; j++)); do
    $BIN_CTR_CLI cdimg merge --cdimgs $MERGED_CDIMGS --outCdimg $IMAGE_DIR/merged.cdimg --threadNum 8 --mergeDimgConcurrentNum 4 --mergeMode linear --benchmark
    $BIN_CTR_CLI cdimg merge --cdimgs $MERGED_CDIMGS --outCdimg $IMAGE_DIR/merged.cdimg --threadNum 8 --mergeDimgConcurrentNum 4 --mergeMode bisect --benchmark
done
MERGED_CDIMGS=$MERGED_CDIMGS,$(diff_image 1.23.2 1.23.3)
MERGED_CDIMGS=$MERGED_CDIMGS,$(diff_image 1.23.3 1.23.4)
MERGED_CDIMGS=$MERGED_CDIMGS,$(diff_image 1.23.4 1.24.0)
MERGED_CDIMGS=$MERGED_CDIMGS,$(diff_image 1.24.0 1.25.0) # 16
for ((j=0; j < $RUN_NUM; j++)); do
    $BIN_CTR_CLI cdimg merge --cdimgs $MERGED_CDIMGS --outCdimg $IMAGE_DIR/merged.cdimg --threadNum 8 --mergeDimgConcurrentNum 4 --mergeMode linear --benchmark
    $BIN_CTR_CLI cdimg merge --cdimgs $MERGED_CDIMGS --outCdimg $IMAGE_DIR/merged.cdimg --threadNum 8 --mergeDimgConcurrentNum 4 --mergeMode bisect --benchmark
done
MERGED_CDIMGS=$MERGED_CDIMGS,$(diff_image 1.25.0 1.25.1)
MERGED_CDIMGS=$MERGED_CDIMGS,$(diff_image 1.25.1 1.25.2)
MERGED_CDIMGS=$MERGED_CDIMGS,$(diff_image 1.25.2 1.25.3)
MERGED_CDIMGS=$MERGED_CDIMGS,$(diff_image 1.25.3 1.25.4) # 20
for ((j=0; j < $RUN_NUM; j++)); do 
    $BIN_CTR_CLI cdimg merge --cdimgs $MERGED_CDIMGS --outCdimg $IMAGE_DIR/merged.cdimg --threadNum 8 --mergeDimgConcurrentNum 4 --mergeMode linear --benchmark
    $BIN_CTR_CLI cdimg merge --cdimgs $MERGED_CDIMGS --outCdimg $IMAGE_DIR/merged.cdimg --threadNum 8 --mergeDimgConcurrentNum 4 --mergeMode bisect --benchmark
done

for ((j=0; j < $RUN_NUM; j++)); do
    $BIN_CTR_CLI cdimg merge --cdimgs $MERGED_CDIMGS --outCdimg $IMAGE_DIR/merged.cdimg --threadNum 8 --mergeDimgConcurrentNum 4 --mergeMode linear --benchmark
    $BIN_CTR_CLI cdimg merge --cdimgs $MERGED_CDIMGS --outCdimg $IMAGE_DIR/merged.cdimg --threadNum 8 --mergeDimgConcurrentNum 4 --mergeMode bisect --benchmark
done

rm -rf $IMAGE_DIR/merged-patched
exit 0
mkdir $IMAGE_DIR/merged-patched

$BIN_CTR_CLI cdimg patch --baseDir $IMAGE_DIR/image-1.21.0 --outDir $IMAGE_DIR/merged-patched --diffCdimg $IMAGE_DIR/merged.cdimg
diff -r --no-dereference $IMAGE_DIR/image-1.25.4 $IMAGE_DIR/merged-patched

$BIN_CTR_CLI push --cdimg $IMAGE_DIR/1.23.1.cdimg --imageTag nginx:1.23.1
$BIN_CTR_CLI push --cdimg $IMAGE_DIR/1.23.1-1.23.2.cdimg
$BIN_CTR_CLI push --cdimg $IMAGE_DIR/1.23.2-1.23.3.cdimg --imageTag nginx:1.23.3
$BIN_CTR_CLI push --cdimg $IMAGE_DIR/1.23.3-1.23.4.cdimg --imageTag nginx:1.23.4
$BIN_CTR_CLI push --cdimg $IMAGE_DIR/1.23.4-1.24.0.cdimg --imageTag nginx:1.24.0
$BIN_CTR_CLI push --cdimg $IMAGE_DIR/1.24.0-1.25.0.cdimg --imageTag nginx:1.25.0
$BIN_CTR_CLI push --cdimg $IMAGE_DIR/1.25.0-1.25.1.cdimg --imageTag nginx:1.25.1
$BIN_CTR_CLI push --cdimg $IMAGE_DIR/1.25.1-1.25.2.cdimg --imageTag nginx:1.25.2
$BIN_CTR_CLI push --cdimg $IMAGE_DIR/1.25.2-1.25.3.cdimg --imageTag nginx:1.25.3
$BIN_CTR_CLI push --cdimg $IMAGE_DIR/1.25.3-1.25.4.cdimg --imageTag nginx:1.25.4

sleep 2
$BIN_CTR_CLI pull --image nginx:1.23.1 --host $SERVER_HOST --expectedDimgsNum 1
$BIN_CTR_CLI pull --image nginx:1.25.4 --host $SERVER_HOST --expectedDimgsNum 9

function validate_snapshots() {
    ctr snapshot --snapshotter=di3fs tree | while read SNP; do 
        SNP_IMAGE_TAG=$(ctr snapshot --snapshotter=di3fs info $SNP | jq -r '.Labels."containerd.io/snapshot/di3fs.image.name"')
        MOUNT_PATH=$(ctr snapshot --snapshotter=di3fs info $SNP | jq -r '.Labels."containerd.io/snapshot/di3fs.mount"')
        IMAGE_TAG=(${SNP_IMAGE_TAG//:/ })
        SNP_IMAGE_NAME=${IMAGE_TAG[0]}
        SNP_IMAGE_NAME=(${SNP_IMAGE_NAME//-/ })
        SNP_IMAGE_NAME=${SNP_IMAGE_NAME[0]}
        SNP_IMAGE_VERSION=${IMAGE_TAG[1]}
    
        if [ "$SNP_IMAGE_NAME" == "nginx" ]; then
            echo "Checking $SNP_IMAGE_TAG at $MOUNT_PATH"
            sudo diff -r $IMAGE_DIR/image-$SNP_IMAGE_VERSION $MOUNT_PATH --no-dereference
        fi
    done
}

validate_snapshots
