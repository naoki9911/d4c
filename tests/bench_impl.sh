#!/bin/bash

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

for ((i=0; i < ${#IMAGE_VERSIONS[@]}; i++));do
	IMAGE=${IMAGE_VERSIONS[i]}
	echo "Creating base image for $IMAGE"
	./diff "" $IMAGE $IMAGE-base $IMAGE-base.json binary-diff
	./pack $IMAGE-base $IMAGE-base.json "" $IMAGE-base.dimg
	./patch dimg "" $IMAGE-base-patched $IMAGE-base.dimg
	diff -r $IMAGE $IMAGE-base-patched --no-dereference
done


for ((i=0; i < $(expr ${#IMAGE_VERSIONS[@]} - 1); i++));do
	LOWER=${IMAGE_VERSIONS[i]}
	UPPER=${IMAGE_VERSIONS[$(expr $i + 1)]}
	DIFF_NAME=$LOWER-$UPPER

	# generating diff data with binary-diff
	for ((j=0; j < $RUN_NUM; j++));do
		NOW_COUNT=$(expr $j + 1)
		echo "Benchmark diff $DIFF_NAME binary-diff ($NOW_COUNT/$RUN_NUM)"
		./diff $LOWER $UPPER diff_$DIFF_NAME diff_$DIFF_NAME.json binary-diff benchmark
	done

	# packing diff data
	./pack diff_$DIFF_NAME diff_$DIFF_NAME.json $LOWER-base.dimg diff_$DIFF_NAME.dimg
	ls -l diff_$DIFF_NAME.dimg

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
		sleep 1
		if [ $j -eq 0 ]; then
			diff -r $UPPER /tmp/fuse --no-dereference
		fi
		fusermount3 -u /tmp/fuse
	done

	# generating diff data with file-dff
	for ((j=0; j < $RUN_NUM; j++));do
		NOW_COUNT=$(expr $j + 1)
		echo "Benchmark diff $DIFF_NAME file-diff ($NOW_COUNT/$RUN_NUM)"
		./diff $LOWER $UPPER diff_file_$DIFF_NAME diff_file_$DIFF_NAME.json file-diff benchmark
	done

	# packing diff data and test it
	echo "Testing packed $DIFF_NAME file-diff"
	./pack diff_file_$DIFF_NAME diff_file_$DIFF_NAME.json $LOWER-base.dimg diff_file_$DIFF_NAME.dimg
	ls -l diff_file_$DIFF_NAME.dimg
	./patch dimg $LOWER $UPPER-patched diff_file_$DIFF_NAME.dimg
	diff -r $UPPER $UPPER-patched --no-dereference
done

MERGE_LOWER=$IMAGE_LOWER-$IMAGE_MIDDLE
MERGE_UPPER=$IMAGE_MIDDLE-$IMAGE_UPPER
MERGED=$IMAGE_LOWER-$IMAGE_UPPER
for ((j=0; j < $RUN_NUM; j++));do
	NOW_COUNT=$(expr $j + 1)
	echo "Benchmark merge $MERGE_LOWER and $MERGE_UPPER to $MERGED ($NOW_COUNT/$RUN_NUM)"
	./merge dimg diff_$MERGE_LOWER.dimg diff_$MERGE_UPPER.dimg diff_merged_$MERGED.dimg benchmark
done

echo "Testing merged $MERGED"
./patch dimg $IMAGE_LOWER $IMAGE_UPPER-merged diff_merged_$MERGED.dimg
diff -r $IMAGE_UPPER $IMAGE_UPPER-merged --no-dereference
ls -l diff_merged_$MERGED.dimg
./fuse-diff --basedir=./$IMAGE_LOWER-base.dimg --patchdir=./diff_merged_$MERGED.dimg --mode=dimg /tmp/fuse >/dev/null 2>&1 &
sleep 1
diff -r $IMAGE_UPPER /tmp/fuse --no-dereference
fusermount3 -u /tmp/fuse

for ((j=0; j < $RUN_NUM; j++));do
	NOW_COUNT=$(expr $j + 1)
	echo "Benchmark regen-diff $MERGED binary-diff ($NOW_COUNT/$RUN_NUM)"
	./diff $IMAGE_LOWER $IMAGE_UPPER diff_$MERGED diff_$MERGED.json binary-diff benchmark
done
./pack diff_$MERGED diff_$MERGED.json $IMAGE_LOWER-base.dimg diff_$MERGED.dimg
ls -l diff_$MERGED.dimg

for ((j=0; j < $RUN_NUM; j++));do
	NOW_COUNT=$(expr $j + 1)
	echo "Benchmark regen-diff $MERGED file-diff ($NOW_COUNT/$RUN_NUM)"
	./diff $IMAGE_LOWER $IMAGE_UPPER diff_file_$MERGED diff_file_$MERGED.json file-diff benchmark
done
./pack diff_file_$MERGED diff_file_$MERGED.json $IMAGE_LOWER-base.dimg diff_file_$MERGED.dimg
ls -l diff_file_$MERGED.dimg

echo "Benchmark done"
