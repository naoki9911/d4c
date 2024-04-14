.PHONY: snapshotter ctr-cli server fuse-diff plugins

all: snapshotter ctr-cli server fuse-diff plugins

run:
	make clean
	make snapshotter
	sudo ./snapshotter

snapshotter:
	go build -o snapshotter ./cmd/snapshotter

ctr-cli:
	go build -o ctr-cli ./cmd/ctr-cli

server:
	go build -o server ./cmd/server

fuse-diff:
	go build -o fuse-diff ./cmd/fuse-diff

plugins:
	go build -buildmode=plugin -o plugin_basic.so ./cmd/plugins/basic/main.go

clean:
	rm -f snapshotter ctr-cli server fuse-diff plugin_gz.so
