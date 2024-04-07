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
    $BIN_CTR_CLI convert --image nginx:$IMAGE_VERSION --output $IMAGE_DIR/$IMAGE_VERSION --cdimg --excludes /dev --threadNum $THREAD_NUM
    mv $IMAGE_DIR/$IMAGE_VERSION/image.cdimg $IMAGE_DIR/$IMAGE_VERSION.cdimg
    mkdir $IMAGE_DIR/image-$IMAGE_VERSION
    tar -xf $IMAGE_DIR/$IMAGE_VERSION/layer.tar -C $IMAGE_DIR/image-$IMAGE_VERSION
    rm -rf $IMAGE_DIR/image-$IMAGE_VERSION/dev
}

function diff_image() {
    LOWER=$1
    UPPER=$2
    if [ -e $IMAGE_DIR/$LOWER-$UPPER.cdimg ]; then
        return
    fi

    $BIN_CTR_CLI cdimg diff --oldCdimg $IMAGE_DIR/$LOWER.cdimg --newCdimg $IMAGE_DIR/$UPPER.cdimg --outCdimg $IMAGE_DIR/$LOWER-$UPPER.cdimg --threadNum $THREAD_NUM
}

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


diff_image 1.23.1 1.23.2
diff_image 1.23.2 1.23.3
diff_image 1.23.3 1.23.4
diff_image 1.23.4 1.24.0
diff_image 1.24.0 1.25.0
diff_image 1.25.0 1.25.1
diff_image 1.25.1 1.25.2
diff_image 1.25.2 1.25.3
diff_image 1.25.3 1.25.4

$BIN_CTR_CLI cdimg merge --cdimgs $IMAGE_DIR/1.25.3-1.25.4.cdimg,$IMAGE_DIR/1.25.2-1.25.3.cdimg,$IMAGE_DIR/1.25.1-1.25.2.cdimg,$IMAGE_DIR/1.25.0-1.25.1.cdimg,$IMAGE_DIR/1.24.0-1.25.0.cdimg,$IMAGE_DIR/1.23.4-1.24.0.cdimg,$IMAGE_DIR/1.23.3-1.23.4.cdimg,$IMAGE_DIR/1.23.2-1.23.3.cdimg,$IMAGE_DIR/1.23.1-1.23.2.cdimg --outCdimg $IMAGE_DIR/1.23.1-1.25.4.cdimg --threadNum 8 --mergeDimgConcurrentNum 4 --mergeMode bisect
rm -rf $IMAGE_DIR/1.25.4-patched
mkdir $IMAGE_DIR/1.25.4-patched

$BIN_CTR_CLI cdimg patch --baseDir $IMAGE_DIR/image-1.23.1 --outDir $IMAGE_DIR/1.25.4-patched --diffCdimg $IMAGE_DIR/1.23.1-1.25.4.cdimg
diff -r --no-dereference $IMAGE_DIR/image-1.25.4 $IMAGE_DIR/1.25.4-patched

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
