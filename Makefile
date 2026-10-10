# Use bash so we can rely on pipefail in recipes.
SHELL := /bin/bash

V ?= 0
ifeq ($(V), 1)
override VERBOSE_TAG := -v
endif

TEST_TIMEOUT := 5m
GOLANGCI     ?= golangci-lint

.DEFAULT_GOAL := help

##@ General

help: ## Print this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m [VAR=value ...]\n"} \
		/^[a-zA-Z0-9_.-]+:.*?##/ { printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2 } \
		/^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) }' $(MAKEFILE_LIST)

##@ Lint

lint: ## Run golangci-lint (version pinned in mise.toml)
	@$(GOLANGCI) run

fmt: ## Apply golangci-lint fmt
	@$(GOLANGCI) fmt

vet: ## Run go vet
	@go vet ./...

check: lint vet ## Run lint + vet

docs-check: ## Check docs/ against .agents/rules/450-doc-lifecycle.md
	@python3 scripts/docs-check.py

data-check: ## Check that git tracks nothing under data/ but its README.md and .gitignore
	@python3 scripts/data-check.py

hooks: ## Enable the repository's git hooks (data-check before every commit); once per clone
	@git config core.hooksPath scripts/hooks

##@ Test

test: ## Run tests with -short and -race
	@CGO_ENABLED=1 go test ./... -short -race -timeout=$(TEST_TIMEOUT) $(VERBOSE_TAG)

bench: ## Run benchmarks
	@go test ./... -run=^$$ -bench=. -benchmem

##@ CI

ci: check docs-check data-check test ## Every gate: lint, vet, docs check, data check, tests

.PHONY: help lint fmt vet check docs-check data-check hooks test bench ci
