.PHONY: snapshotter ctr-cli server fuse-diff

all: snapshotter ctr-cli server fuse-diff

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

clean:
	rm -f snapshotter ctr-cli server fuse-diff
