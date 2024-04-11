#!/bin/bash

set -eu

mkdir -p /tmp/fuse

function err() {
    fusermount3 -u /tmp/fuse
    exit 1
}

trap err ERR

ROOT_DIR=$(cd $(dirname $0)/../; pwd)
BIN_CTR_CLI="$ROOT_DIR/ctr-cli"
BIN_FUSE="$ROOT_DIR/fuse-diff"
SCRIPT_DIR=$(cd $(dirname $0); pwd)

function test_convert() {
    TEST_SCRIPT=$1
    IMAGE_DIR=$2

    source $TEST_SCRIPT

    IMAGE_DIR=$IMAGE_DIR/$IMAGE_NAME
    mkdir -p $IMAGE_DIR

    pushd $IMAGE_DIR

    for ((i=0; i < ${#IMAGE_VERSIONS[@]}; i++));do
    	IMAGE=${IMAGE_VERSIONS[i]}
        mkdir ./image-$IMAGE
    	$BIN_CTR_CLI convert --image $DOCKER_IMAGE:$IMAGE --output ./image-$IMAGE
    	mkdir $IMAGE-convert
    	tar -xf ./image-$IMAGE/layer.tar -C ./$IMAGE-convert
        cd $IMAGE-convert
        rm -f $(find -name .wh..wh..opq)
        rm -f $(find . -type b)
        rm -f $(find . -type c)
        rm -f $(find . -type p)
        rm -f $(find . -type s)
        cd ../

    	$BIN_CTR_CLI convert2 --image $DOCKER_IMAGE:$IMAGE --output ./image-$IMAGE-2
    	mkdir $IMAGE-convert2
        $BIN_CTR_CLI dimg patch --outDir ./$IMAGE-convert2 --diffDimg ./image-$IMAGE-2/image.dimg
    	diff -r $IMAGE-convert $IMAGE-convert2 --no-dereference

        $BIN_FUSE --daemon --diffDimg ./image-$IMAGE-2/image.dimg /tmp/fuse
    	diff -r $IMAGE-convert /tmp/fuse --no-dereference
        fusermount3 -u /tmp/fuse
    done

    popd
    rm -rf $IMAGE_DIR
}

IMAGE_DIR_BASE="./benchmark_convert"
rm -rf $IMAGE_DIR_BASE

TESTS=("apache" "mysql" "nginx" "postgres" "redis")
for TEST in "${TESTS[@]}"; do
    cd $SCRIPT_DIR
    test_convert test_$TEST.sh $IMAGE_DIR_BASE
done
