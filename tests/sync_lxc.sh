#!/bin/bash

set -eu -o pipefail

echo "updating source code"
rm -rf ~/d4c
sudo cp -r /host ~/d4c
sudo chown -R ubuntu:ubuntu ~/d4c
cd ~/d4c
sudo PATH=$PATH:/usr/local/go/bin make all
echo "source code is updated"
