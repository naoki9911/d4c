#!/bin/bash

systemctl stop d4c-snapshotter
mount | grep fuse-diff | awk '{print $3}' | while read MOUNT; do fusermount3 -u $MOUNT; done

set -eu

RESULT_DIR=$1
TEST_SCRIPT=$2
RUN_NUM=$3
SERVER_HOST=$4
DELTA_ENCODING=$5
REMOTE_IMAGE_DIR=$6
LOCAL_IMAGE_DIR=$(cd $7; pwd)

source $TEST_SCRIPT

LABELS="threadNum:8,threadSchedMode:none,compressionMode:bzip2,imageName:$IMAGE_NAME,deltaEncoding:$DELTA_ENCODING"

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

systemd-run --unit=d4c-snapshotter $BIN_SNAPSHOTTER
systemctl restart containerd

sleep 2

cd $RESULT_DIR

curl -XDELETE http://$SERVER_HOST/cleanup
$BIN_CTR_CLI push --cdimg $REMOTE_IMAGE_DIR/diff_$IMAGE_LOWER-$IMAGE_MIDDLE-$DELTA_ENCODING.cdimg --imageTag $IMAGE_NAME:$IMAGE_MIDDLE --serverHost $SERVER_HOST
$BIN_CTR_CLI push --cdimg $REMOTE_IMAGE_DIR/diff_$IMAGE_MIDDLE-$IMAGE_UPPER-$DELTA_ENCODING.cdimg --imageTag $IMAGE_NAME:$IMAGE_UPPER --serverHost $SERVER_HOST

$BIN_CTR_CLI load --image $IMAGE_NAME:$IMAGE_MIDDLE --cdimg $LOCAL_IMAGE_DIR/$IMAGE_MIDDLE.cdimg
for ((j=0; j < $RUN_NUM; j++));do
    NOW_COUNT=$(expr $j + 1)
    echo "Benchmark pull $IMAGE_NAME:$IMAGE_UPPER ($NOW_COUNT/$RUN_NUM)"
    $BIN_CTR_CLI --labels $LABELS,old:$IMAGE_MIDDLE,new:$IMAGE_UPPER,mode:binary-diff pull --image $IMAGE_NAME:$IMAGE_UPPER --benchmark --host $SERVER_HOST --expectedDimgsNum 1
    ctr image rm $IMAGE_NAME:$IMAGE_UPPER
    sleep 5
done
ctr image rm $IMAGE_NAME:$IMAGE_MIDDLE


$BIN_CTR_CLI load --image $IMAGE_NAME:$IMAGE_LOWER --cdimg $LOCAL_IMAGE_DIR/$IMAGE_LOWER.cdimg
for ((j=0; j < $RUN_NUM; j++));do
    NOW_COUNT=$(expr $j + 1)
    echo "Benchmark pull $IMAGE_NAME:$IMAGE_MIDDLE ($NOW_COUNT/$RUN_NUM)"
    $BIN_CTR_CLI --labels $LABELS,old:$IMAGE_LOWER,new:$IMAGE_MIDDLE,mode:binary-diff pull --image $IMAGE_NAME:$IMAGE_MIDDLE --benchmark --host $SERVER_HOST --expectedDimgsNum 1
    ctr image rm $IMAGE_NAME:$IMAGE_MIDDLE
    sleep 5
done

for ((j=0; j < $RUN_NUM; j++));do
    NOW_COUNT=$(expr $j + 1)
    echo "Benchmark pull $IMAGE_NAME:$IMAGE_UPPER ($NOW_COUNT/$RUN_NUM)"
    $BIN_CTR_CLI --labels $LABELS,old:$IMAGE_LOWER,new:$IMAGE_UPPER,mode:binary-diff pull --image $IMAGE_NAME:$IMAGE_UPPER --benchmark --host $SERVER_HOST --expectedDimgsNum 2
    ctr image rm $IMAGE_NAME:$IMAGE_UPPER
    sleep 5
done
ctr image rm $IMAGE_NAME:$IMAGE_LOWER

sleep 2

curl -XDELETE http://$SERVER_HOST/cleanup
$BIN_CTR_CLI push --cdimg $REMOTE_IMAGE_DIR/diff_file_$IMAGE_LOWER-$IMAGE_MIDDLE.cdimg --imageTag $IMAGE_NAME-file:$IMAGE_MIDDLE --serverHost $SERVER_HOST
$BIN_CTR_CLI push --cdimg $REMOTE_IMAGE_DIR/diff_file_$IMAGE_MIDDLE-$IMAGE_UPPER.cdimg --imageTag $IMAGE_NAME-file:$IMAGE_UPPER --serverHost $SERVER_HOST

$BIN_CTR_CLI load --image $IMAGE_NAME-file:$IMAGE_MIDDLE --cdimg $LOCAL_IMAGE_DIR/$IMAGE_MIDDLE.cdimg
for ((j=0; j < $RUN_NUM; j++));do
    NOW_COUNT=$(expr $j + 1)
    echo "Benchmark pull $IMAGE_NAME-file:$IMAGE_UPPER ($NOW_COUNT/$RUN_NUM)"
    $BIN_CTR_CLI --labels $LABELS,old:$IMAGE_MIDDLE,new:$IMAGE_UPPER,mode:file-diff pull --image $IMAGE_NAME-file:$IMAGE_UPPER --benchmark --host $SERVER_HOST --expectedDimgsNum 1
    ctr image rm $IMAGE_NAME-file:$IMAGE_UPPER
    sleep 5
done
ctr image rm $IMAGE_NAME-file:$IMAGE_MIDDLE

$BIN_CTR_CLI load --image $IMAGE_NAME-file:$IMAGE_LOWER --cdimg $LOCAL_IMAGE_DIR/$IMAGE_LOWER.cdimg
for ((j=0; j < $RUN_NUM; j++));do
    NOW_COUNT=$(expr $j + 1)
    echo "Benchmark pull $IMAGE_NAME-file:$IMAGE_MIDDLE ($NOW_COUNT/$RUN_NUM)"
    $BIN_CTR_CLI --labels $LABELS,old:$IMAGE_LOWER,new:$IMAGE_MIDDLE,mode:file-diff pull --image $IMAGE_NAME-file:$IMAGE_MIDDLE --benchmark --host $SERVER_HOST --expectedDimgsNum 1
    ctr image rm $IMAGE_NAME-file:$IMAGE_MIDDLE
    sleep 5
done

for ((j=0; j < $RUN_NUM; j++));do
    NOW_COUNT=$(expr $j + 1)
    echo "Benchmark pull $IMAGE_NAME-file:$IMAGE_UPPER ($NOW_COUNT/$RUN_NUM)"
    $BIN_CTR_CLI --labels $LABELS,old:$IMAGE_LOWER,new:$IMAGE_UPPER,mode:file-diff pull --image $IMAGE_NAME-file:$IMAGE_UPPER --benchmark --host $SERVER_HOST --expectedDimgsNum 2
    ctr image rm $IMAGE_NAME-file:$IMAGE_UPPER
    sleep 5
done
ctr image rm $IMAGE_NAME-file:$IMAGE_LOWER

systemctl stop d4c-snapshotter
