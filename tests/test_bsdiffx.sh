#!/bin/bash

set -eux

PATH=$PATH:/usr/local/go/bin

cd $(cd $(dirname $0); pwd)
pushd ../
make all
popd

ROOT_DIR=$(cd $(dirname $0)/../; pwd)
BIN_CTR_CLI="$ROOT_DIR/ctr-cli"

echo "THIS IS OLD FILE" > /tmp/old.txt
echo "THIS IS NEW FILE" > /tmp/new.txt
$BIN_CTR_CLI util diff --old=/tmp/old.txt --new=/tmp/new.txt --diff=/tmp/diff.diff
$BIN_CTR_CLI util patch --old=/tmp/old.txt --new=/tmp/new-patched.txt --diff=/tmp/diff.diff
diff /tmp/new.txt /tmp/new-patched.txt

