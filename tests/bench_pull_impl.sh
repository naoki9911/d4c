#!/bin/bash

set -eu

source ./version.sh
SERVER_HOST=$1
RUN_NUM=$2

ctr image rm $IMAGE_NAME:$IMAGE_LOWER
ctr image rm $IMAGE_NAME:$IMAGE_MIDDLE
ctr image rm $IMAGE_NAME:$IMAGE_UPPER
ctr image rm $IMAGE_NAME-file:$IMAGE_LOWER
ctr image rm $IMAGE_NAME-file:$IMAGE_MIDDLE
ctr image rm $IMAGE_NAME-file:$IMAGE_UPPER

go build ../../cmd/./ctr-cli

./ctr-cli pull --image $IMAGE_NAME:$IMAGE_MIDDLE --host $SERVER_HOST
for ((j=0; j < $RUN_NUM; j++));do
    NOW_COUNT=$(expr $j + 1)
    echo "Benchmark pull $IMAGE_NAME:$IMAGE_UPPER ($NOW_COUNT/$RUN_NUM)"
    ./ctr-cli pull --image $IMAGE_NAME:$IMAGE_UPPER --benchmark --host $SERVER_HOST
    ctr image rm $IMAGE_NAME:$IMAGE_UPPER
done
ctr image rm $IMAGE_NAME:$IMAGE_MIDDLE

./ctr-cli pull --image $IMAGE_NAME:$IMAGE_LOWER --host $SERVER_HOST
for ((j=0; j < $RUN_NUM; j++));do
    NOW_COUNT=$(expr $j + 1)
    echo "Benchmark pull $IMAGE_NAME:$IMAGE_MIDDLE ($NOW_COUNT/$RUN_NUM)"
    ./ctr-cli pull --image $IMAGE_NAME:$IMAGE_MIDDLE --benchmark --host $SERVER_HOST
    ctr image rm $IMAGE_NAME:$IMAGE_MIDDLE
done

for ((j=0; j < $RUN_NUM; j++));do
    NOW_COUNT=$(expr $j + 1)
    echo "Benchmark pull $IMAGE_NAME:$IMAGE_UPPER ($NOW_COUNT/$RUN_NUM)"
    ./ctr-cli pull --image $IMAGE_NAME:$IMAGE_UPPER --benchmark --host $SERVER_HOST
    ctr image rm $IMAGE_NAME:$IMAGE_UPPER
done

./ctr-cli pull --image $IMAGE_NAME-file:$IMAGE_MIDDLE --host $SERVER_HOST
for ((j=0; j < $RUN_NUM; j++));do
    NOW_COUNT=$(expr $j + 1)
    echo "Benchmark pull $IMAGE_NAME-file:$IMAGE_UPPER ($NOW_COUNT/$RUN_NUM)"
    ./ctr-cli pull --image $IMAGE_NAME-file:$IMAGE_UPPER --benchmark --host $SERVER_HOST
    ctr image rm $IMAGE_NAME-file:$IMAGE_UPPER
done
ctr image rm $IMAGE_NAME-file:$IMAGE_MIDDLE

./ctr-cli pull --image $IMAGE_NAME-file:$IMAGE_LOWER --host $SERVER_HOST
for ((j=0; j < $RUN_NUM; j++));do
    NOW_COUNT=$(expr $j + 1)
    echo "Benchmark pull $IMAGE_NAME-file:$IMAGE_MIDDLE ($NOW_COUNT/$RUN_NUM)"
    ./ctr-cli pull --image $IMAGE_NAME-file:$IMAGE_MIDDLE --benchmark --host $SERVER_HOST
    ctr image rm $IMAGE_NAME-file:$IMAGE_MIDDLE
done

for ((j=0; j < $RUN_NUM; j++));do
    NOW_COUNT=$(expr $j + 1)
    echo "Benchmark pull $IMAGE_NAME-file:$IMAGE_UPPER ($NOW_COUNT/$RUN_NUM)"
    ./ctr-cli pull --image $IMAGE_NAME-file:$IMAGE_UPPER --benchmark --host $SERVER_HOST
    ctr image rm $IMAGE_NAME-file:$IMAGE_UPPER
done