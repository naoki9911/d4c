.PHONY: snapshotter ctr-cli diff-tools server

all: snapshotter ctr-cli diff-tools server

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
	go build -o pack ./cmd/pack

server:
	go build -o server ./cmd/server

clean:
	rm -f snapshotter ctr-cli diff pack server
