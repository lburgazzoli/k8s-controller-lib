MKFILE_PATH := $(abspath $(lastword $(MAKEFILE_LIST)))
PROJECT_PATH := $(patsubst %/,%,$(dir $(MKFILE_PATH)))
LOCAL_BIN_PATH := ${PROJECT_PATH}/bin

LINT_GOGC := 10
LINT_TIMEOUT := 10m

## Tools
GOLANGCI_VERSION ?= v2.8.0
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

## Container Runtime Detection
# Auto-detect Docker or Podman and configure environment for testcontainers
# Priority: pre-configured DOCKER_HOST > Docker > Podman > Error
define configure_container_runtime
	@echo "Detecting container runtime..."; \
	if [ -n "$$DOCKER_HOST" ]; then \
		echo "DOCKER_HOST already set: $$DOCKER_HOST"; \
		case "$$DOCKER_HOST" in \
			*podman*) \
				echo "✓ Using Podman (pre-configured via DOCKER_HOST)"; \
				export TESTCONTAINERS_RYUK_DISABLED=true; \
				;; \
			*) \
				echo "✓ Using Docker (pre-configured via DOCKER_HOST)"; \
				;; \
		esac; \
	elif docker info >/dev/null 2>&1; then \
		echo "✓ Using Docker (auto-detected)"; \
	elif command -v podman >/dev/null 2>&1; then \
		if podman machine inspect >/dev/null 2>&1; then \
			echo "✓ Using Podman (auto-detected via podman machine)"; \
			export DOCKER_HOST="unix://$$(podman machine inspect --format '{{.ConnectionInfo.PodmanSocket.Path}}')"; \
			export TESTCONTAINERS_RYUK_DISABLED=true; \
		elif [ -S "$${XDG_RUNTIME_DIR}/podman/podman.sock" ]; then \
			echo "✓ Using Podman (auto-detected via XDG_RUNTIME_DIR)"; \
			export DOCKER_HOST="unix://$${XDG_RUNTIME_DIR}/podman/podman.sock"; \
			export TESTCONTAINERS_RYUK_DISABLED=true; \
		else \
			echo "ERROR: Podman found but not running."; \
			echo "  - macOS/Windows: Run 'podman machine start'"; \
			echo "  - Linux: Ensure Podman socket exists"; \
			exit 1; \
		fi; \
	else \
		echo "ERROR: Neither Docker nor Podman is available"; \
		echo "  Install Docker: https://docs.docker.com/get-docker/"; \
		echo "  Install Podman: https://podman.io/getting-started/installation"; \
		exit 1; \
	fi
endef

.PHONY: container-runtime
container-runtime: ## Detect and display container runtime configuration
	@$(configure_container_runtime)

.PHONY: test/unit
test/unit:
	go test -v ./pkg/...

.PHONY: test/integration
test/integration:
	@$(configure_container_runtime) && go test -v ./tests/integration/...

.PHONY: test/examples
test/examples:
	@$(configure_container_runtime) && \
	cd examples/simple-controller && go test -v ./... && \
	cd ../cleanup-controller && go test -v ./...

.PHONY: test
test: test/unit test/integration test/examples

## Benchmark configuration
BENCH_COUNT ?= 5
BENCH_TIME ?= 1s

.PHONY: bench
bench: ## Run all benchmarks
	go test -bench=. -benchmem -count=$(BENCH_COUNT) -benchtime=$(BENCH_TIME) ./pkg/...

.PHONY: bench/builder
bench/builder: ## Run builder conversion benchmarks
	go test -bench=BenchmarkConversion -benchmem -count=$(BENCH_COUNT) -benchtime=$(BENCH_TIME) ./pkg/builder/...

.PHONY: bench/compare
bench/compare: ## Run benchmarks and save output for comparison (use with benchstat)
	go test -bench=BenchmarkConversion -benchmem -count=$(BENCH_COUNT) -benchtime=$(BENCH_TIME) ./pkg/builder/... | tee bench.txt
	@echo "Results saved to bench.txt. Compare with: benchstat old.txt bench.txt"

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