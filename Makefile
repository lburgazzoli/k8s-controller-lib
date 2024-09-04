MKFILE_PATH := $(abspath $(lastword $(MAKEFILE_LIST)))
PROJECT_PATH := $(patsubst %/,%,$(dir $(MKFILE_PATH)))
LOCAL_BIN_PATH := ${PROJECT_PATH}/bin

CGO_ENABLED := 0
BUILD_TAGS := -tags all

LINT_GOGC ?= 10
LINT_TIMEOUT ?= 10m

## Tools
GOIMPORT ?= $(LOCALBIN)/goimports
GOIMPORT_VERSION ?= latest
GOLANGCI ?= $(LOCALBIN)/golangci-lint
GOLANGCI_VERSION ?= v1.60.3


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
fmt: goimport
	$(GOIMPORT) -l -w .
	go fmt ./...

.PHONY: test
test:
	go test $(BUILD_TAGS) ./...

.PHONY: deps
deps:
	go mod tidy

.PHONY: check/lint
check: check/lint

.PHONY: check/lint
check/lint: golangci-lint
	@$(GOLANGCI) run \
		--config .golangci.yml \
		--out-format tab \
		--timeout $(LINT_TIMEOUT)

LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	@mkdir -p $(LOCALBIN)

.PHONY: goimport
goimport: $(GOIMPORT)
$(GOIMPORT): $(LOCALBIN)
	@test -s $(GOIMPORT) || \
	GOBIN=$(LOCALBIN) go install golang.org/x/tools/cmd/goimports@$(GOIMPORT_VERSION)

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI)
$(GOLANGCI): $(LOCALBIN)
	@test -s $(GOLANGCI) || \
	GOBIN=$(LOCALBIN) go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_VERSION)
