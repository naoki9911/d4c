#!/bin/bash

systemctl stop d4c-server
systemctl stop d4c-snapshotter
mount | grep fuse-diff | awk '{print $3}' | while read MOUNT; do fusermount3 -u $MOUNT; done

set -eu

TEST_SCRIPT=$1
IMAGE_DIR=$2
RUN_NUM=$3
SERVER_HOST=$4

source $TEST_SCRIPT

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

systemd-run --unit=d4c-server $BIN_SERVER
systemd-run --unit=d4c-snapshotter $BIN_SNAPSHOTTER

curl -XDELETE http://$SERVER_HOST/diffData/cleanup

curl -XPOST http://$SERVER_HOST/diffData/add \
     -d @- <<EOF
{
        "imageName": "$IMAGE_NAME",
        "fileName":"$IMAGE_PATH/$IMAGE_LOWER-base.dimg",
        "configPath":"$IMAGE_PATH/image-$IMAGE_LOWER/config.json",
        "version":"$IMAGE_LOWER",
        "baseVersion":""
}
EOF

curl -XPOST http://$SERVER_HOST/diffData/add \
     -d @- <<EOF
{
        "imageName": "$IMAGE_NAME",
        "fileName":"$IMAGE_PATH/$IMAGE_MIDDLE-base.dimg",
        "configPath":"$IMAGE_PATH/image-$IMAGE_MIDDLE/config.json",
        "version":"$IMAGE_MIDDLE",
        "baseVersion":""
}
EOF

curl -XPOST http://$SERVER_HOST/diffData/add \
     -d @- <<EOF
{
        "imageName": "$IMAGE_NAME",
        "fileName":"$IMAGE_PATH/diff_$IMAGE_LOWER-$IMAGE_MIDDLE.dimg",
        "configPath":"$IMAGE_PATH/image-$IMAGE_MIDDLE/config.json",
        "version":"$IMAGE_MIDDLE",
        "baseVersion":"$IMAGE_LOWER"
}
EOF

curl -XPOST http://$SERVER_HOST/diffData/add \
     -d @- <<EOF
{
        "imageName": "$IMAGE_NAME",
        "fileName":"$IMAGE_PATH/diff_$IMAGE_MIDDLE-$IMAGE_UPPER.dimg",
        "configPath":"$IMAGE_PATH/image-$IMAGE_UPPER/config.json",
        "version":"$IMAGE_UPPER",
        "baseVersion":"$IMAGE_MIDDLE"
}
EOF


curl -XPOST http://$SERVER_HOST/diffData/add \
     -d @- <<EOF
{
        "imageName": "$IMAGE_NAME-file",
        "fileName":"$IMAGE_PATH/$IMAGE_LOWER-base.dimg",
        "configPath":"$IMAGE_PATH/image-$IMAGE_LOWER/config.json",
        "version":"$IMAGE_LOWER",
        "baseVersion":""
}
EOF

curl -XPOST http://$SERVER_HOST/diffData/add \
     -d @- <<EOF
{
        "imageName": "$IMAGE_NAME-file",
        "fileName":"$IMAGE_PATH/$IMAGE_MIDDLE-base.dimg",
        "configPath":"$IMAGE_PATH/image-$IMAGE_MIDDLE/config.json",
        "version":"$IMAGE_MIDDLE",
        "baseVersion":""
}
EOF

curl -XPOST http://$SERVER_HOST/diffData/add \
     -d @- <<EOF
{
        "imageName": "$IMAGE_NAME-file",
        "fileName":"$IMAGE_PATH/diff_file_$IMAGE_LOWER-$IMAGE_MIDDLE.dimg",
        "configPath":"$IMAGE_PATH/image-$IMAGE_MIDDLE/config.json",
        "version":"$IMAGE_MIDDLE",
        "baseVersion":"$IMAGE_LOWER"
}
EOF

curl -XPOST http://$SERVER_HOST/diffData/add \
     -d @- <<EOF
{
        "imageName": "$IMAGE_NAME-file",
        "fileName":"$IMAGE_PATH/diff_file_$IMAGE_MIDDLE-$IMAGE_UPPER.dimg",
        "configPath":"$IMAGE_PATH/image-$IMAGE_UPPER/config.json",
        "version":"$IMAGE_UPPER",
        "baseVersion":"$IMAGE_MIDDLE"
}
EOF

$BIN_CTR_CLI pull --image $IMAGE_NAME:$IMAGE_MIDDLE --host $SERVER_HOST
for ((j=0; j < $RUN_NUM; j++));do
    NOW_COUNT=$(expr $j + 1)
    echo "Benchmark pull $IMAGE_NAME:$IMAGE_UPPER ($NOW_COUNT/$RUN_NUM)"
    $BIN_CTR_CLI pull --image $IMAGE_NAME:$IMAGE_UPPER --benchmark --host $SERVER_HOST
    ctr image rm $IMAGE_NAME:$IMAGE_UPPER
done
ctr image rm $IMAGE_NAME:$IMAGE_MIDDLE

$BIN_CTR_CLI pull --image $IMAGE_NAME:$IMAGE_LOWER --host $SERVER_HOST
for ((j=0; j < $RUN_NUM; j++));do
    NOW_COUNT=$(expr $j + 1)
    echo "Benchmark pull $IMAGE_NAME:$IMAGE_MIDDLE ($NOW_COUNT/$RUN_NUM)"
    $BIN_CTR_CLI pull --image $IMAGE_NAME:$IMAGE_MIDDLE --benchmark --host $SERVER_HOST
    ctr image rm $IMAGE_NAME:$IMAGE_MIDDLE
done

for ((j=0; j < $RUN_NUM; j++));do
    NOW_COUNT=$(expr $j + 1)
    echo "Benchmark pull $IMAGE_NAME:$IMAGE_UPPER ($NOW_COUNT/$RUN_NUM)"
    $BIN_CTR_CLI pull --image $IMAGE_NAME:$IMAGE_UPPER --benchmark --host $SERVER_HOST
    ctr image rm $IMAGE_NAME:$IMAGE_UPPER
done
ctr image rm $IMAGE_NAME:$IMAGE_LOWER

$BIN_CTR_CLI pull --image $IMAGE_NAME-file:$IMAGE_MIDDLE --host $SERVER_HOST
for ((j=0; j < $RUN_NUM; j++));do
    NOW_COUNT=$(expr $j + 1)
    echo "Benchmark pull $IMAGE_NAME-file:$IMAGE_UPPER ($NOW_COUNT/$RUN_NUM)"
    $BIN_CTR_CLI pull --image $IMAGE_NAME-file:$IMAGE_UPPER --benchmark --host $SERVER_HOST
    ctr image rm $IMAGE_NAME-file:$IMAGE_UPPER
done
ctr image rm $IMAGE_NAME-file:$IMAGE_MIDDLE

$BIN_CTR_CLI pull --image $IMAGE_NAME-file:$IMAGE_LOWER --host $SERVER_HOST
for ((j=0; j < $RUN_NUM; j++));do
    NOW_COUNT=$(expr $j + 1)
    echo "Benchmark pull $IMAGE_NAME-file:$IMAGE_MIDDLE ($NOW_COUNT/$RUN_NUM)"
    $BIN_CTR_CLI pull --image $IMAGE_NAME-file:$IMAGE_MIDDLE --benchmark --host $SERVER_HOST
    ctr image rm $IMAGE_NAME-file:$IMAGE_MIDDLE
done

for ((j=0; j < $RUN_NUM; j++));do
    NOW_COUNT=$(expr $j + 1)
    echo "Benchmark pull $IMAGE_NAME-file:$IMAGE_UPPER ($NOW_COUNT/$RUN_NUM)"
    $BIN_CTR_CLI pull --image $IMAGE_NAME-file:$IMAGE_UPPER --benchmark --host $SERVER_HOST
    ctr image rm $IMAGE_NAME-file:$IMAGE_UPPER
done
ctr image rm $IMAGE_NAME-file:$IMAGE_LOWER

systemctl stop d4c-server
systemctl stop d4c-snapshotter
