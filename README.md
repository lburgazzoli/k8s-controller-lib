# k8s-controller-lib

A Kubernetes controller library providing reusable utilities and patterns for building controllers with controller-runtime.

## Installation

```bash
go get github.com/lburgazzoli/k8s-controller-lib
```

## Packages

- **`pkg/reconciler/pipeline`** - Pipeline-based reconciliation orchestration
- **`pkg/reconciler/result`** - Reconciliation result helpers (RequeueAfter, Success, etc.)
- **`pkg/conditions`** - Condition management and aggregation utilities
- **`pkg/resources`** - Resource utilities (server-side apply, conversions, GVKs)
- **`pkg/status`** - Standard status types and accessors
- **`pkg/predicates`** - Event filtering predicates
- **`pkg/reconciler/watch`** - Automatic watch management for dynamic resources

## Quick Start

See [`examples/simple-controller`](examples/simple-controller/) for a complete working example demonstrating pipeline-based reconciliation.

## Documentation

- [Architecture Overview](docs/architecture.md) - High-level design and component interaction
- [Development Guidelines](docs/development.md) - Coding standards and patterns
- [Conditions Package](docs/design/conditions.md) - Condition management API
- [Auto-Watch Feature](docs/design/auto-watch.md) - Dynamic resource watching

## Testing

```bash
make test              # Run all tests
make test/unit         # Unit tests only
make test/integration  # Integration tests with k3s
make test/examples     # Example controller tests
```

## License

See [LICENSE](LICENSE) file for details.
