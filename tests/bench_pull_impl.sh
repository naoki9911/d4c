#!/bin/bash

systemctl stop d4c-server
systemctl stop d4c-snapshotter
mount | grep fuse-diff | awk '{print $3}' | while read MOUNT; do fusermount3 -u $MOUNT; done

set -eu

function validate_snapshots() {
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
}

TEST_SCRIPT=$1
IMAGE_DIR=$2
RUN_NUM=$3
SERVER_HOST=$4
THREAD_NUM=$5
SCHED_MODE=$6
COMP_MODE=$7

source $TEST_SCRIPT

LABELS="threadNum:$THREAD_NUM,threadSchedMode:$SCHED_MODE,compressionMode:$COMP_MODE,imageName:$IMAGE_NAME"

ctr image rm $IMAGE_NAME:$IMAGE_LOWER
ctr image rm $IMAGE_NAME:$IMAGE_MIDDLE
ctr image rm $IMAGE_NAME:$IMAGE_UPPER
ctr image rm $IMAGE_NAME-file:$IMAGE_LOWER
ctr image rm $IMAGE_NAME-file:$IMAGE_MIDDLE
ctr image rm $IMAGE_NAME-file:$IMAGE_UPPER

ROOT_DIR=$(cd $(dirname $0)/../; pwd)
BIN_CTR_CLI="$ROOT_DIR/ctr-cli"
BIN_SERVER="$ROOT_DIR/server"
BIN_SNAPSHOTTER="$ROOT_DIR/snapshotter"

IMAGE_DIR=$IMAGE_DIR/$IMAGE_NAME
cd $IMAGE_DIR
IMAGE_PATH=$(pwd)

systemd-run --unit=d4c-server $BIN_SERVER --threadNum $THREAD_NUM
systemd-run --unit=d4c-snapshotter $BIN_SNAPSHOTTER
systemctl restart containerd

sleep 2

$BIN_CTR_CLI pack --config $IMAGE_PATH/image-$IMAGE_LOWER/config.json --dimg $IMAGE_PATH/$IMAGE_LOWER.dimg --out $IMAGE_PATH/$IMAGE_LOWER.cdimg
$BIN_CTR_CLI pack --config $IMAGE_PATH/image-$IMAGE_MIDDLE/config.json --dimg $IMAGE_PATH/$IMAGE_MIDDLE.dimg --out $IMAGE_PATH/$IMAGE_MIDDLE.cdimg
$BIN_CTR_CLI pack --config $IMAGE_PATH/image-$IMAGE_MIDDLE/config.json --dimg $IMAGE_PATH/diff_$IMAGE_LOWER-$IMAGE_MIDDLE.dimg --out $IMAGE_PATH/diff_$IMAGE_LOWER-$IMAGE_MIDDLE.cdimg
$BIN_CTR_CLI pack --config $IMAGE_PATH/image-$IMAGE_UPPER/config.json --dimg $IMAGE_PATH/diff_$IMAGE_MIDDLE-$IMAGE_UPPER.dimg --out $IMAGE_PATH/diff_$IMAGE_MIDDLE-$IMAGE_UPPER.cdimg
$BIN_CTR_CLI pack --config $IMAGE_PATH/image-$IMAGE_MIDDLE/config.json --dimg $IMAGE_PATH/diff_file_$IMAGE_LOWER-$IMAGE_MIDDLE.dimg --out $IMAGE_PATH/diff_file_$IMAGE_LOWER-$IMAGE_MIDDLE.cdimg
$BIN_CTR_CLI pack --config $IMAGE_PATH/image-$IMAGE_UPPER/config.json --dimg $IMAGE_PATH/diff_file_$IMAGE_MIDDLE-$IMAGE_UPPER.dimg --out $IMAGE_PATH/diff_file_$IMAGE_MIDDLE-$IMAGE_UPPER.cdimg

curl -XDELETE http://$SERVER_HOST/cleanup

$BIN_CTR_CLI push --cdimg $IMAGE_PATH/$IMAGE_LOWER.cdimg --imageTag $IMAGE_NAME:$IMAGE_LOWER
$BIN_CTR_CLI push --cdimg $IMAGE_PATH/$IMAGE_MIDDLE.cdimg --imageTag $IMAGE_NAME:$IMAGE_MIDDLE
$BIN_CTR_CLI push --cdimg $IMAGE_PATH/diff_$IMAGE_LOWER-$IMAGE_MIDDLE.cdimg
$BIN_CTR_CLI push --cdimg $IMAGE_PATH/diff_$IMAGE_MIDDLE-$IMAGE_UPPER.cdimg --imageTag $IMAGE_NAME:$IMAGE_UPPER

$BIN_CTR_CLI pull --image $IMAGE_NAME:$IMAGE_MIDDLE --host $SERVER_HOST
for ((j=0; j < $RUN_NUM; j++));do
    NOW_COUNT=$(expr $j + 1)
    echo "Benchmark pull $IMAGE_NAME:$IMAGE_UPPER ($NOW_COUNT/$RUN_NUM)"
    $BIN_CTR_CLI --labels $LABELS,old:$IMAGE_MIDDLE,new:$IMAGE_UPPER,mode:binary-diff pull --image $IMAGE_NAME:$IMAGE_UPPER --benchmark --host $SERVER_HOST --expectedDimgsNum 1
    ctr image rm $IMAGE_NAME:$IMAGE_UPPER
done
ctr image rm $IMAGE_NAME:$IMAGE_MIDDLE

$BIN_CTR_CLI pull --image $IMAGE_NAME:$IMAGE_LOWER --host $SERVER_HOST
for ((j=0; j < $RUN_NUM; j++));do
    NOW_COUNT=$(expr $j + 1)
    echo "Benchmark pull $IMAGE_NAME:$IMAGE_MIDDLE ($NOW_COUNT/$RUN_NUM)"
    $BIN_CTR_CLI --labels $LABELS,old:$IMAGE_LOWER,new:$IMAGE_MIDDLE,mode:binary-diff pull --image $IMAGE_NAME:$IMAGE_MIDDLE --benchmark --host $SERVER_HOST --expectedDimgsNum 1
    ctr image rm $IMAGE_NAME:$IMAGE_MIDDLE
done

for ((j=0; j < $RUN_NUM; j++));do
    NOW_COUNT=$(expr $j + 1)
    echo "Benchmark pull $IMAGE_NAME:$IMAGE_UPPER ($NOW_COUNT/$RUN_NUM)"
    $BIN_CTR_CLI --labels $LABELS,old:$IMAGE_LOWER,new:$IMAGE_UPPER,mode:binary-diff pull --image $IMAGE_NAME:$IMAGE_UPPER --benchmark --host $SERVER_HOST --expectedDimgsNum 2
    ctr image rm $IMAGE_NAME:$IMAGE_UPPER
done
ctr image rm $IMAGE_NAME:$IMAGE_LOWER

sleep 2
validate_snapshots

curl -XDELETE http://$SERVER_HOST/cleanup

$BIN_CTR_CLI push --cdimg $IMAGE_PATH/$IMAGE_LOWER.cdimg --imageTag $IMAGE_NAME-file:$IMAGE_LOWER
$BIN_CTR_CLI push --cdimg $IMAGE_PATH/$IMAGE_MIDDLE.cdimg --imageTag $IMAGE_NAME-file:$IMAGE_MIDDLE
$BIN_CTR_CLI push --cdimg $IMAGE_PATH/diff_file_$IMAGE_LOWER-$IMAGE_MIDDLE.cdimg
$BIN_CTR_CLI push --cdimg $IMAGE_PATH/diff_file_$IMAGE_MIDDLE-$IMAGE_UPPER.cdimg --imageTag $IMAGE_NAME-file:$IMAGE_UPPER

$BIN_CTR_CLI pull --image $IMAGE_NAME-file:$IMAGE_MIDDLE --host $SERVER_HOST
for ((j=0; j < $RUN_NUM; j++));do
    NOW_COUNT=$(expr $j + 1)
    echo "Benchmark pull $IMAGE_NAME-file:$IMAGE_UPPER ($NOW_COUNT/$RUN_NUM)"
    $BIN_CTR_CLI --labels $LABELS,old:$IMAGE_MIDDLE,new:$IMAGE_UPPER,mode:file-diff pull --image $IMAGE_NAME-file:$IMAGE_UPPER --benchmark --host $SERVER_HOST --expectedDimgsNum 1
    ctr image rm $IMAGE_NAME-file:$IMAGE_UPPER
done
ctr image rm $IMAGE_NAME-file:$IMAGE_MIDDLE

$BIN_CTR_CLI pull --image $IMAGE_NAME-file:$IMAGE_LOWER --host $SERVER_HOST
for ((j=0; j < $RUN_NUM; j++));do
    NOW_COUNT=$(expr $j + 1)
    echo "Benchmark pull $IMAGE_NAME-file:$IMAGE_MIDDLE ($NOW_COUNT/$RUN_NUM)"
    $BIN_CTR_CLI --labels $LABELS,old:$IMAGE_LOWER,new:$IMAGE_MIDDLE,mode:file-diff pull --image $IMAGE_NAME-file:$IMAGE_MIDDLE --benchmark --host $SERVER_HOST --expectedDimgsNum 1
    ctr image rm $IMAGE_NAME-file:$IMAGE_MIDDLE
done

for ((j=0; j < $RUN_NUM; j++));do
    NOW_COUNT=$(expr $j + 1)
    echo "Benchmark pull $IMAGE_NAME-file:$IMAGE_UPPER ($NOW_COUNT/$RUN_NUM)"
    $BIN_CTR_CLI --labels $LABELS,old:$IMAGE_LOWER,new:$IMAGE_UPPER,mode:file-diff pull --image $IMAGE_NAME-file:$IMAGE_UPPER --benchmark --host $SERVER_HOST --expectedDimgsNum 2
    ctr image rm $IMAGE_NAME-file:$IMAGE_UPPER
done
ctr image rm $IMAGE_NAME-file:$IMAGE_LOWER
validate_snapshots

systemctl stop d4c-server
systemctl stop d4c-snapshotter
