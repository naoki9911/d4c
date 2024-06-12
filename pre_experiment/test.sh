#!/bin/bash

set -eux

RESULT_FILE="./benchmark_`date +%Y-%m-%d-%H%M`.log"

function benchmark() {
    BIN=$1
    OLD_VERSION=$2
    NEW_VERSION=$3
    NEW_SIZE=$(ls -l ./$BIN-$NEW_VERSION | awk '{print $5}')

    MEM_KB_MAX=$(/usr/bin/time -v ../ctr-cli util diff --old ./$BIN-$OLD_VERSION --new ./$BIN-$NEW_VERSION --diff ./$BIN-diff-xdelta3 --algorithm xdelta3 2>&1 | grep "Maximum resident" | awk '{print $6}')
    RESULT=$(multitime -n 10 ../ctr-cli util diff --old ./$BIN-$OLD_VERSION --new ./$BIN-$NEW_VERSION --diff ./$BIN-diff-xdelta3 --algorithm xdelta3 2>&1 | grep real)
    DIFF_SIZE=$(ls -l $BIN-diff-xdelta3 | awk '{print $5}')
    echo "{\"target\": \"$BIN $OLD_VERSION-$NEW_VERSION\", \"algorithm\": \"xdelta3\", \"result\": \"${RESULT}\", \"size\": $DIFF_SIZE, \"newSize\": $NEW_SIZE, \"maxMemUse(KB)\": $MEM_KB_MAX}" >> $RESULT_FILE

    MEM_KB_MAX=$(/usr/bin/time -v ../ctr-cli util diff --old ./$BIN-$OLD_VERSION --new ./$BIN-$NEW_VERSION --diff ./$BIN-diff-bsdiffx --algorithm bsdiffx 2>&1 | grep "Maximum resident" | awk '{print $6}')
    RESULT=$(multitime -n 10 ../ctr-cli util diff --old ./$BIN-$OLD_VERSION --new ./$BIN-$NEW_VERSION --diff ./$BIN-diff-bsdiffx --algorithm bsdiffx 2>&1 | grep real)
    DIFF_SIZE=$(ls -l $BIN-diff-bsdiffx | awk '{print $5}')
    echo "{\"target\": \"$BIN $OLD_VERSION-$NEW_VERSION\", \"algorithm\": \"bsdiffx\", \"result\": \"${RESULT}\", \"size\": $DIFF_SIZE, \"newSize\": $NEW_SIZE, \"maxMemUse(KB)\": $MEM_KB_MAX}" >> $RESULT_FILE
}

benchmark postgres 13.1 13.2
benchmark redis 7.0.5.tar 7.0.6.tar