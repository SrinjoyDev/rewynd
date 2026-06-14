# Common dev tasks. Run `make help` for the list.
SHELL := /bin/bash
.DEFAULT_GOAL := help

.PHONY: help build test vet fmt tidy install-deps clean

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-13s\033[0m %s\n", $$1, $$2}'

build: ## Build the rewynd binary into ./core/rewynd
	go -C core build -o rewynd ./cmd/rewynd

test: ## Run the Go test suite
	go -C core test ./...

vet: ## go vet the core
	go -C core vet ./...

fmt: ## Format Go code
	gofmt -w core/cmd core/internal

tidy: ## go mod tidy
	go -C core mod tidy

install-deps: ## Install the JS workspace deps
	pnpm install

clean: ## Remove build + release artifacts
	rm -f core/rewynd
	rm -rf dist packages/cli-platforms
