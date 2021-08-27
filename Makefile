.PHONY: clean
.DEFAULT_GOAL := build

build:
	go build -v ./...
