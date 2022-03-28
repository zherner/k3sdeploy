.PHONY: clean
.DEFAULT_GOAL := help

help: ## Display this help text
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

build: ## Builds the Go binary
	go build -v ./...

install: ## Builds the Go binary and puts it in your GOPATH
	go install -v ./...

clean: ## Cleanup the binary.
	rm -f k3sdeploy
