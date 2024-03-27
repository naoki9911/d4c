.PHONY: snapshotter ctr-cli diff-tools server fuse-diff

all: snapshotter ctr-cli diff-tools server fuse-diff

run:
	make clean
	make snapshotter
	sudo ./snapshotter

snapshotter:
	go build -o snapshotter ./cmd/snapshotter

ctr-cli:
	go build -o ctr-cli ./cmd/ctr-cli

diff-tools:
	go build -o diff ./cmd/diff

server:
	go build -o server ./cmd/server

fuse-diff:
	go build -o fuse-diff ./cmd/fuse-diff

clean:
	rm -f snapshotter ctr-cli diff server fuse-diff
