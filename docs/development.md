# Development Guidelines

This document outlines the development standards and practices for the k8s-controller-lib project.

## Core Principles

### Consistency with controller-runtime

Follow patterns established by the Kubernetes controller-runtime library:
- Use similar patterns for controller setup and reconciliation
- Align with controller-runtime's event handling and workqueue management
- Maintain compatibility with standard controller-runtime interfaces where applicable

### Functional Options Pattern

Follow controller-runtime's interface-based options pattern:

**Option Interface:**
```go
// Define an interface for each option type
type GetOption interface {
    ApplyToGet(*GetOptions)
}

type GetOptions struct {
    Raw *metav1.GetOptions
}

// GetOptions implements GetOption interface
func (o *GetOptions) ApplyToGet(opts *GetOptions) {
    if o.Raw != nil {
        opts.Raw = o.Raw
    }
}

func (o *GetOptions) ApplyOptions(opts []GetOption) *GetOptions {
    for _, opt := range opts {
        opt.ApplyToGet(o)
    }
    return o
}
```

**Concrete Option Types:**
```go
// Option types implement the interface method
type Namespace string

func (n Namespace) ApplyToGet(opts *GetOptions) {
    // Apply namespace to options
    if opts.Raw == nil {
        opts.Raw = &metav1.GetOptions{}
    }
    opts.Raw.Namespace = string(n)
}

type ResourceVersion string

func (rv ResourceVersion) ApplyToGet(opts *GetOptions) {
    if opts.Raw == nil {
        opts.Raw = &metav1.GetOptions{}
    }
    opts.Raw.ResourceVersion = string(rv)
}

// Usage
client.Get(ctx, key, obj, Namespace("default"), ResourceVersion("12345"))
```

### File Naming Conventions

- Options definitions must use the `_opts.go` suffix
- Example: `controller_opts.go`, `reconciler_opts.go`

## Testing

### Gomega Without Ginkgo

- Use vanilla Gomega assertions for all tests
- Always use dot imports for Gomega:
  ```go
  import . "github.com/onsi/gomega"
  import . "github.com/onsi/gomega/gstruct"
  ```
- Use standard Go testing with `testing.T`
- Structure tests using table-driven patterns when appropriate

**Example:**
```go
func TestReconcile(t *testing.T) {
    g := NewWithT(t)

    result, err := reconciler.Reconcile(ctx, req)
    g.Expect(err).ToNot(HaveOccurred())
    g.Expect(result.Requeue).To(BeFalse())
}
```

### Prefer MatchFields for Multi-Field Assertions

When testing multiple fields of a struct, use `gstruct.MatchFields` instead of individual field expectations:
- Makes tests more concise and easier to read
- Clearly shows which fields are being validated
- Use `gstruct.IgnoreExtras` to validate only the fields you care about

**Good:**
```go
import (
    . "github.com/onsi/gomega"
    . "github.com/onsi/gomega/gstruct"
)

g.Expect(result).To(PointTo(MatchFields(IgnoreExtras, Fields{
    "Type":   Equal("Ready"),
    "Status": Equal(metav1.ConditionTrue),
    "Reason": Equal("Initialized"),
})))
```

**Avoid:**
```go
g.Expect(result.Type).To(Equal("Ready"))
g.Expect(result.Status).To(Equal(metav1.ConditionTrue))
g.Expect(result.Reason).To(Equal("Initialized"))
```

## Design Philosophy

### Functions Over Interfaces

- Prefer concrete function types over interfaces when possible
- Use interfaces only when:
  - Multiple implementations need to be swapped at runtime
  - The interface is part of the public API contract
  - Testing requires mocking complex external dependencies

**Good:**
```go
type ProcessFunc func(context.Context, *Resource) error
```

**Use sparingly:**
```go
type Processor interface {
    Process(context.Context, *Resource) error
}
```

### Comments and Documentation

- Comments should clarify **why** something is done, not **what** is being done
- Focus on:
  - Non-obvious business logic or algorithm choices
  - Edge cases and their handling
  - Relationships between components
  - Concurrency concerns or guarantees
- Avoid redundant comments that merely restate the code

**Bad:**
```go
// Set the timeout to 5 seconds
timeout := 5 * time.Second
```

**Good:**
```go
// Use shorter timeout for init containers to fail fast during cluster bootstrap
timeout := 5 * time.Second
```

### Dependency Management

**Reuse Kubernetes Libraries:**
- Leverage existing k8s.io packages whenever possible
- Prefer `client-go`, `apimachinery`, and `controller-runtime` utilities
- Don't reinvent functionality that exists in the ecosystem

**Minimize External Dependencies:**
- Avoid introducing new dependencies unless they provide significant value
- Evaluate the maintenance status and compatibility of any new dependency
- Consider vendoring critical dependencies for stability
- Document the rationale for each major dependency in go.mod comments

## Code Organization

### Package Structure

- Keep packages focused and cohesive
- Avoid circular dependencies
- Use internal packages for implementation details not meant for public use

### Error Handling

- Return errors as the last return value
- Use `fmt.Errorf` with `%w` for error wrapping to maintain error chains
- Provide context in error messages about what operation failed

### Naming Conventions

- Use Go standard naming conventions
- Prefer clarity over brevity for exported APIs
- Use short names only in limited scopes (loop variables, closures)

## Version Compatibility

- Maintain compatibility with supported Kubernetes versions
- Clearly document minimum required versions for dependencies
- Test against multiple Kubernetes versions when possible
