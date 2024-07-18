#!/bin/bash

set -eux

if [ $EUID -ne 0 ]; then
	echo "root privilege required"
	exit 1
fi

BIN_CTR="../../ctr-cli"

if [ ! -e 13.1/image.cdimg ]; then
	$BIN_CTR convert2 --image postgres:13.1 --output 13.1 --cdimg --threadNum 8
fi
if [ ! -e 13.2/image.cdimg ]; then
	$BIN_CTR convert2 --image postgres:13.2 --output 13.2 --cdimg --threadNum 8
fi

if [ ! -e 13.1-13.2-bsdiffx.cdimg ]; then
	$BIN_CTR cdimg diff --oldCdimg 13.1/image.cdimg --newCdimg 13.2/image.cdimg --outCdimg 13.1-13.2-bsdiffx.cdimg --deltaEncoding bsdiffx --threadNum 8
fi

