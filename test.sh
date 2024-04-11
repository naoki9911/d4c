#!/bin/bash

set -eu

make

./ctr-cli util merge --lower templates.dat.1.24.0-1.25.0 --upper templates.dat.1.25.0-1.25.1 --out templates.dat.1.24.0-1.25.1 --base templates.dat.1.24.0 --updated templates.dat.1.25.1
./ctr-cli util patch --old templates.dat.1.24.0 --diff templates.dat.1.24.0-1.25.1 --new templates.dat.1.25.1.patched
diff -r templates.dat.1.25.1.patched templates.dat.1.25.1
