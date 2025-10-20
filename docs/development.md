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

### Use t.Context() in Tests

Always use `t.Context()` instead of `context.Background()` when creating contexts in tests:
- Automatically cancelled when the test completes or fails
- Helps detect context leaks and goroutine issues
- Follows modern Go testing patterns (Go 1.21+)
- Provides better test cleanup and resource management

**Good:**
```go
func TestReconcile(t *testing.T) {
    g := NewWithT(t)

    ctx := t.Context()
    result, err := reconciler.Reconcile(ctx, req)

    g.Expect(err).ToNot(HaveOccurred())
}
```

**Avoid:**
```go
func TestReconcile(t *testing.T) {
    g := NewWithT(t)

    ctx := context.Background()  // Don't use this in tests
    result, err := reconciler.Reconcile(ctx, req)

    g.Expect(err).ToNot(HaveOccurred())
}
```

### Gomega Matchers Best Practices

Use Gomega's rich set of matchers to write clear, expressive test assertions.

#### Struct Matchers

Use `gstruct.MatchFields` for validating multiple struct fields:
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

#### Slice Matchers

Use specialized slice matchers instead of manual iteration:

```go
// Check slice contains exactly these elements (order-independent)
g.Expect(objects).To(ConsistOf(obj1, obj2, obj3))

// Check slice contains a specific element
g.Expect(pods).To(ContainElement(MatchFields(IgnoreExtras, Fields{
    "Name": Equal("my-pod"),
})))

// Check slice length
g.Expect(items).To(HaveLen(5))

// Check slice is empty
g.Expect(conditions).To(BeEmpty())
```

#### JQ Matcher for Complex Nested Assertions

For complex nested structures, use the JQ matcher from `github.com/lburgazzoli/gomega-matchers`:
- More readable than deeply nested `MatchFields`
- Powerful JSON path expressions
- Ideal for Kubernetes objects with deep nesting

```go
import (
    jqmatcher "github.com/lburgazzoli/gomega-matchers/pkg/matchers/jq"
)

// Assert on deeply nested fields using JQ expressions
g.Expect(pod).To(jqmatcher.Match(`
    .spec.containers[0].name == "nginx" and
    .spec.containers[0].image == "nginx:latest" and
    .metadata.labels.app == "my-app"
`))

// Check array lengths and nested conditions
g.Expect(deployment).To(jqmatcher.Match(`
    .spec.replicas == 3 and
    (.spec.template.spec.containers | length) == 2 and
    .status.conditions[] | select(.type == "Available") | .status == "True"
`))
```

**When to use JQ matcher:**
- Validating multiple nested fields across different levels
- Complex array filtering or transformations
- Conditional assertions based on field values
- When MatchFields becomes too verbose or hard to read

## Code Quality

### Control Flow Best Practices

Write clean, readable control flow by avoiding deep nesting and using early returns.

**Avoid Nested If Statements**

Deeply nested conditionals make code harder to read and maintain. Use early returns to flatten the logic.

**Bad:**
```go
func Process(obj *Object) error {
    if obj != nil {
        if obj.IsValid() {
            if obj.Status == "active" {
                // Main logic here
                return doWork(obj)
            } else {
                return fmt.Errorf("inactive object")
            }
        } else {
            return fmt.Errorf("invalid object")
        }
    } else {
        return fmt.Errorf("nil object")
    }
}
```

**Good:**
```go
func Process(obj *Object) error {
    // Handle edge cases first with early returns
    if obj == nil {
        return fmt.Errorf("nil object")
    }

    if !obj.IsValid() {
        return fmt.Errorf("invalid object")
    }

    if obj.Status != "active" {
        return fmt.Errorf("inactive object")
    }

    // Happy path at the end, no nesting
    return doWork(obj)
}
```

**Return Early (Fail Fast)**

Check preconditions and error cases first, then handle the happy path. This keeps the main logic at the lowest indentation level.

**Bad:**
```go
func Reconcile(obj client.Object) error {
    if !obj.GetDeletionTimestamp().IsZero() {
        if hasFinalizer(obj) {
            if err := cleanup(obj); err != nil {
                return err
            }
            removeFinalizer(obj)
        }
    } else {
        if !hasFinalizer(obj) {
            addFinalizer(obj)
        }
        return reconcile(obj)
    }
    return nil
}
```

**Good:**
```go
func Reconcile(obj client.Object) error {
    // Handle deletion case first
    if !obj.GetDeletionTimestamp().IsZero() {
        if !hasFinalizer(obj) {
            return nil  // Nothing to clean up
        }

        if err := cleanup(obj); err != nil {
            return err
        }

        removeFinalizer(obj)
        return nil
    }

    // Handle normal reconciliation
    if !hasFinalizer(obj) {
        addFinalizer(obj)
    }

    return reconcile(obj)
}
```

**Key Principles:**
- Handle edge cases and errors first
- Use early returns to avoid nesting
- Keep happy path at the lowest indentation level
- Make control flow read top-to-bottom

### Linting

Always run the linter before committing changes:

```bash
make lint
```

- Address all linter errors before submitting code
- Linter configuration is defined in `.golangci.yml`
- The project uses golangci-lint with a comprehensive set of checks
- If you need to disable a linter for a specific case, document why with a comment

## Git Workflow

### Conventional Commits

Use conventional commit messages to maintain a clear and structured git history:

**Format:**
```
<type>: <description>

[optional body]

[optional footer]
```

**Common types:**
- `feat`: A new feature
- `fix`: A bug fix
- `docs`: Documentation changes
- `test`: Adding or updating tests
- `refactor`: Code changes that neither fix bugs nor add features
- `chore`: Maintenance tasks, dependency updates, etc.

**Examples:**
```
feat: add server-side apply support for resources

fix: handle nil pointer in reconciler status update

docs: update installation instructions

test: add field manager verification tests

refactor: simplify error handling in controller
```

For more details, see the [Conventional Commits specification](https://www.conventionalcommits.org/).

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
