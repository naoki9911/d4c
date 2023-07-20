#!/bin/bash

fusermount3 -u /tmp/fuse

set -eu

source ./version.sh
RUN_NUM=$1

mkdir -p /tmp/fuse

function err() {
    fusermount3 -u /tmp/fuse
    exit 1
}

trap err ERR

rm -f diff patch pack fuse-diff merge
go build ../../cmd/diff
go build ../../cmd/patch
go build ../../cmd/pack
go build ../../cmd/fuse-diff
go build ../../cmd/merge

for ((i=0; i < $(expr ${#IMAGE_VERSIONS[@]} - 1); i++));do
	LOWER=${IMAGE_VERSIONS[i]}
	UPPER=${IMAGE_VERSIONS[$(expr $i + 1)]}
	DIFF_NAME=$LOWER-$UPPER

	# patching diff data
	for ((j=0; j < $RUN_NUM; j++));do
		NOW_COUNT=$(expr $j + 1)
		echo "Benchmark patch $DIFF_NAME binary-diff ($NOW_COUNT/$RUN_NUM)"
		./patch dimg $LOWER $UPPER-patched diff_$DIFF_NAME.dimg benchmark
	done
	diff -r $UPPER $UPPER-patched --no-dereference

	# mount with di3fs
	for ((j=0; j < $RUN_NUM; j++));do
		NOW_COUNT=$(expr $j + 1)
		echo "Benchmark di3fs $DIFF_NAME binary-diff ($NOW_COUNT/$RUN_NUM)"
		./fuse-diff --basedir=./$LOWER-base.dimg --patchdir=./diff_$DIFF_NAME.dimg --mode=dimg --benchmark=true /tmp/fuse >/dev/null 2>&1 &
		sleep 5
		if [ $j -eq 0 ]; then
			diff -r $UPPER /tmp/fuse --no-dereference
		fi
		fusermount3 -u /tmp/fuse
		sleep 2
	done
done

