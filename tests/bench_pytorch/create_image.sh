#!/bin/bash

set -eux

if [ $EUID -ne 0 ]; then
	echo "root privilege required"
	exit 1
fi

BIN_CTR="../../ctr-cli"

if [ ! -e 2.2.0/image.cdimg ]; then
	$BIN_CTR convert2 --load --image pytorch-bench-2.2.0.tar --output 2.2.0 --cdimg --threadNum 8
fi
if [ ! -e 2.2.1/image.cdimg ]; then
	$BIN_CTR convert2 --load --image pytorch-bench-2.2.1.tar --output 2.2.1 --cdimg --threadNum 8
fi

if [ ! -e 2.2.0-2.2.1-bsdiffx.cdimg ]; then
	$BIN_CTR cdimg diff --oldCdimg 2.2.0/image.cdimg --newCdimg 2.2.1/image.cdimg --outCdimg 2.2.0-2.2.1-bsdiffx.cdimg --deltaEncoding bsdiffx --threadNum 8
fi

