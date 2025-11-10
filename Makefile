MKFILE_PATH := $(abspath $(lastword $(MAKEFILE_LIST)))
PROJECT_PATH := $(patsubst %/,%,$(dir $(MKFILE_PATH)))
LOCAL_BIN_PATH := ${PROJECT_PATH}/bin

LINT_GOGC := 10
LINT_TIMEOUT := 10m

## Tools
GOLANGCI_VERSION ?= v2.6.0
GOLANGCI ?= go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_VERSION)
GOVULNCHECK_VERSION ?= latest
GOVULNCHECK ?= go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)
CONTROLLER_GEN ?= go tool controller-gen

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif


ifndef ignore-not-found
  ignore-not-found = false
endif

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: test

.PHONY: clean
clean:
	go clean -x
	go clean -x -testcache

.PHONY: fmt
fmt:
	@$(GOLANGCI) fmt --config .golangci.yml
	go fmt ./...

.PHONY: test/unit
test/unit:
	go test -v ./pkg/...

.PHONY: test/integration
test/integration:
	go test -v ./tests/integration/...

.PHONY: test/examples
test/examples:
	cd examples/simple-controller && go test -v ./...

.PHONY: test
test: test/unit test/integration test/examples

.PHONY: deps
deps:
	go mod tidy

.PHONY: lint
lint:
	@$(GOLANGCI) run --config .golangci.yml --timeout $(LINT_TIMEOUT)

.PHONY: lint/fix
lint/fix:
	@$(GOLANGCI) run --config .golangci.yml --timeout $(LINT_TIMEOUT) --fix

.PHONY: vulncheck
vulncheck:
	@$(GOVULNCHECK) ./...

LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	@mkdir -p $(LOCALBIN)

.PHONY: generate
generate:
	$(CONTROLLER_GEN) object:headerFile=./hack/boilerplate.go.txt paths="./pkg/status/..."

.PHONY: verify-generate
verify-generate: generate
	@if [ -n "$$(git status --porcelain)" ]; then \
		echo "Generated files are out of date. Run 'make generate'"; \
		git diff; \
		exit 1; \
	fi

.PHONY: check
check: lint vulncheck