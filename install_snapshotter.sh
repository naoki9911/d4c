#!/bin/bash

cat <<EOT | sudo tee -a /etc/containerd/config.toml > /dev/null
[proxy_plugins]
  [proxy_plugins.di3fs]
    type = "snapshot"
    address = "/run/di3fs/snapshotter.sock"
EOT
