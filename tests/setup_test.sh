#!/bin/bash

TEST_USER=ubuntu
set -eu -o pipefail

if [ "$(whoami)" != "$TEST_USER" ]; then
    su $TEST_USER -c $0
    exit 0
fi

GO_VERSION="1.21.8"

echo "===== Prepare ====="
(
  set -x
 
  # for lxc
  if [ -d /host ]; then
    sudo cp -r /host ~/d4c
  fi

  sudo chown -R $TEST_USER:$TEST_USER ~/d4c

  sudo apt-get update
  sudo DEBIAN_FRONTEND=noninteractive apt-get install -y apt-transport-https ca-certificates curl software-properties-common
  curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add -
  sudo add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable"

  sudo apt-get update
  sudo DEBIAN_FRONTEND=noninteractive apt-get install -y dbus-user-session docker-ce containerd.io fuse3 python3 python3-pip jq
  sudo DEBIAN_FRONTEND=noninteractive apt-get install -y git build-essential automake # for xdelta3
  sudo pip3 install docker-squash matplotlib

  git clone https://github.com/naoki9911/go-xdelta
  cd go-xdelta && git checkout v0.0.3 && ./build_install.sh

  systemctl --user start dbus

  curl -fsSL https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz | sudo tar Cxz /usr/local

  cd ~/d4c
  ./install_snapshotter.sh
  sudo PATH=$PATH:/usr/local/go/bin make all

  sudo systemctl enable --now containerd
)
