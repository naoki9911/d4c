#!/bin/bash

set -eux

IMAGE_NAME="d4c-nginx"
IMAGE_LOWER="1.23.1"
IMAGE_MIDDLE="1.23.2"
IMAGE_UPPER="1.23.3"

SERVER_HOST="localhost:8081"
IMAGE_PATH="./images"

curl -XDELETE http://$SERVER_HOST/diffData/cleanup

curl -XPOST http://$SERVER_HOST/diffData/add \
     -d @- <<EOF
{
        "imageName": "$IMAGE_NAME",
        "fileName":"$IMAGE_PATH/diff_nginx-$IMAGE_LOWER-2.dimg",
        "configPath":"$IMAGE_PATH/nginx-$IMAGE_MIDDLE/config.json",
        "version":"$IMAGE_MIDDLE",
        "baseVersion":"$IMAGE_LOWER"
}
EOF
curl -XPOST http://$SERVER_HOST/diffData/add \
     -d @- <<EOF
{
        "imageName": "$IMAGE_NAME",
        "fileName":"$IMAGE_PATH/base_nginx-$IMAGE_LOWER.dimg",
        "configPath":"$IMAGE_PATH/nginx-$IMAGE_LOWER/config.json",
        "version":"$IMAGE_LOWER",
        "baseVersion":""
}
EOF
curl -XPOST http://$SERVER_HOST/diffData/add \
     -d @- <<EOF
{
        "imageName": "$IMAGE_NAME",
        "fileName":"$IMAGE_PATH/diff_nginx-$IMAGE_MIDDLE-3.dimg",
        "configPath":"$IMAGE_PATH/nginx-$IMAGE_UPPER/config.json",
        "version":"$IMAGE_UPPER",
        "baseVersion":"$IMAGE_MIDDLE"
}
EOF
