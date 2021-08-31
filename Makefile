.PHONY: clean
.DEFAULT_GOAL := build

build:
	go build -v ./...

clean:
	rm -f k3sdeploy
