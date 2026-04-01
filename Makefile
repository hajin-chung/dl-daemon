.PHONY: build install

build:
    go build -ldflags "-X main.commitHash=$(shell git rev-parse --short HEAD)" -o bin/dld cmd/dld/main.go

install: build
    cp bin/dld ~/.local/bin

