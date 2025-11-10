# Conditions Package Design

## Overview

The `pkg/conditions` package provides utilities for managing Kubernetes conditions on resource status objects. It follows a functional approach, offering simple utility functions that work with any type implementing the `Accessor` interface.

## Motivation

Kubernetes resources commonly use conditions to communicate detailed status information. Managing conditions involves repetitive boilerplate code for:
- Setting condition status, reason, and message
- Checking condition states
- Aggregating multiple conditions into a summary condition

This package provides a consistent, reusable API for condition management that:
- Uses standard `metav1.Condition` types
- Delegates to existing `k8s.io/apimachinery/pkg/api/meta` utilities where possible
- Follows controller-runtime's interface-based options pattern
- Works with any struct through the `Accessor` interface
- Avoids unnecessary abstractions (no Manager type, just functions)

## Core Concepts

### Accessor Interface

The `Accessor` interface provides a uniform way to get and set conditions:

```go
type Accessor interface {
    GetConditions() []metav1.Condition
    SetConditions([]metav1.Condition)
}
```

Resources that implement this interface can use the package directly. For resources that don't, the `NewAccessor` helper creates a wrapper around a conditions slice pointer.

### Functional Options Pattern

Following controller-runtime conventions, the package uses interface-based options:

```go
type ConditionOption interface {
    ApplyToCondition(opts *ConditionOptions)
}
```

Concrete option types (`Reason`, `Message`, `ObservedGeneration`) implement this interface, allowing flexible configuration:

```go
MarkTrue(accessor, "Ready",
    WithReason("Initialized"),
    WithMessage("Component is ready"),
    WithObservedGeneration(obj.Generation))
```

## API Design

### Standard Condition Types and Reasons

The package provides standard condition type and reason constants following Kubernetes conventions:

**Condition Types:**
- `ConditionTypeReady` - Overall operational readiness
- `ConditionTypeAvailable` - Resource available for use
- `ConditionTypeProgressing` - Actively being reconciled
- `ConditionTypeDegraded` - Operating with reduced functionality
- `ConditionTypeDependenciesReady` - All dependencies ready

**Condition Reasons:**
- `ReasonReconciling` - Reconciliation actively in progress
- `ReasonReconcileSuccess` - Reconciliation completed successfully
- `ReasonReconcileError` - Reconciliation failed with error
- `ReasonInitializing` - Initial setup in progress
- `ReasonResourcesProvisioned` - All managed resources created/updated

### Setting Conditions

Three primary functions set condition status:

- `MarkTrue(accessor, conditionType, opts...)` - Set condition to True
- `MarkFalse(accessor, conditionType, opts...)` - Set condition to False
- `MarkUnknown(accessor, conditionType, opts...)` - Set condition to Unknown

These functions:
- Automatically set `LastTransitionTime` to the current time
- Apply the `ObservedGeneration` if provided via options
- Delegate to `meta.SetStatusCondition` for actual updates

**Convenience Helpers:**

For common condition types, use the convenience helpers:

- `MarkAvailable(accessor, reason)` - Sets Available=True
- `MarkProgressing(accessor, reason, message)` - Sets Progressing=True
- `MarkDegraded(accessor, reason, message)` - Sets Degraded=True

These helpers are simple wrappers around `MarkTrue()` that use the standard condition type constants.
For additional options (e.g., `WithObservedGeneration`), use `MarkTrue()` directly with the condition type constants.

### Querying Conditions

Convenience functions provide a consistent naming scheme while delegating to the `meta` package:

- `Get(accessor, conditionType)` → `meta.FindStatusCondition`
- `Has(accessor, conditionType)` - Check if condition exists
- `IsTrue(accessor, conditionType)` → `meta.IsStatusConditionTrue`
- `IsFalse(accessor, conditionType)` → `meta.IsStatusConditionFalse`
- `Remove(accessor, conditionType)` → `meta.RemoveStatusCondition`
- `FirstFalse(accessor, conditionTypes)` - Returns first False condition from list
- `FirstUnknown(accessor, conditionTypes)` - Returns first Unknown condition from list

### Aggregating Conditions

The `Aggregate` function computes a target condition's status based on contributing conditions:

```go
Aggregate(accessor, "Ready", []string{"DatabaseReady", "CacheReady", "APIReady"})
```

**Aggregation logic:**
- If any contributing condition is `False`, the target is `False` (uses first False condition's reason/message)
- If any contributing condition is `Unknown` (and none are `False`), the target is `Unknown` (uses first Unknown condition's reason/message)
- If all contributing conditions are `True`, the target is `True`
- If no contributing conditions are provided (empty list), the target is `True`
- Missing conditions are treated as `Unknown`

**Reason and Message propagation:**
- When a `False` or `Unknown` condition is found, its `Reason` and `Message` are copied to the target condition
- If no `Reason`/`Message` is found from contributing conditions, defaults to the target condition type name
- Defaults can be customized using `WithDefaultReason()` and `WithDefaultMessage()` options

**Customization options:**
```go
// Custom default reason/message when all conditions are True
Aggregate(accessor, "Ready", []string{"DatabaseReady", "CacheReady"},
    WithDefaultReason("AllHealthy"),
    WithDefaultMessage("All components are operational"))

// Empty contributing list with custom defaults
Aggregate(accessor, "Ready", []string{},
    WithDefaultReason("NoComponents"),
    WithDefaultMessage("No components configured"))
```

**Future extensibility:**

The `AggregateOption` interface allows for future customization:
- Negative polarity handling (e.g., treating `ErrorFound=False` as positive)
- Custom merge strategies
- Fallback status for missing conditions
- Priority-based aggregation

## Usage Examples

### Basic Resource with Accessor

```go
type MyResource struct {
    metav1.TypeMeta
    metav1.ObjectMeta
    Status MyResourceStatus
}

type MyResourceStatus struct {
    Conditions []metav1.Condition
}

func (s *MyResourceStatus) GetConditions() []metav1.Condition {
    return s.Conditions
}

func (s *MyResourceStatus) SetConditions(conditions []metav1.Condition) {
    s.Conditions = conditions
}

// Usage
resource := &MyResource{}
MarkTrue(&resource.Status, "Ready", WithReason("Initialized"))
```

### Resource Without Native Accessor

```go
type LegacyResource struct {
    Status LegacyStatus
}

type LegacyStatus struct {
    Conditions []metav1.Condition
}

// Create accessor wrapper
resource := &LegacyResource{}
accessor := NewAccessor(&resource.Status.Conditions)

MarkTrue(accessor, "Ready", WithReason("Initialized"))
MarkFalse(accessor, "Progressing",
    WithReason("WaitingForDependency"),
    WithMessage("Waiting for database to be ready"))
```

### Condition Aggregation

```go
type Controller struct {
    Status ControllerStatus
}

type ControllerStatus struct {
    Conditions []metav1.Condition
}

func (s *ControllerStatus) GetConditions() []metav1.Condition {
    return s.Conditions
}

func (s *ControllerStatus) SetConditions(conditions []metav1.Condition) {
    s.Conditions = conditions
}

// Set component conditions
controller := &Controller{}
MarkTrue(&controller.Status, "DatabaseReady", WithReason("Connected"))
MarkTrue(&controller.Status, "CacheReady", WithReason("Connected"))
MarkFalse(&controller.Status, "APIReady",
    WithReason("Unavailable"),
    WithMessage("API server is not responding"))

// Aggregate into overall Ready condition
Aggregate(&controller.Status, "Ready",
    []string{"DatabaseReady", "CacheReady", "APIReady"})

// Result: Ready=False with:
//   Reason="Unavailable" (from first False condition)
//   Message="API server is not responding" (from first False condition)

// Example with all conditions True
MarkTrue(&controller.Status, "APIReady", WithReason("Connected"))
Aggregate(&controller.Status, "Ready",
    []string{"DatabaseReady", "CacheReady", "APIReady"})

// Result: Ready=True with:
//   Reason="Ready" (defaults to target condition type)
//   Message="Ready" (defaults to target condition type)

// Example with custom defaults
Aggregate(&controller.Status, "Ready",
    []string{"DatabaseReady", "CacheReady", "APIReady"},
    WithDefaultReason("AllComponentsHealthy"),
    WithDefaultMessage("All systems operational"))

// Result: Ready=True with:
//   Reason="AllComponentsHealthy"
//   Message="All systems operational"
```

### Checking Conditions

```go
if IsTrue(&resource.Status, "Ready") {
    // Resource is ready
}

if condition := Get(&resource.Status, "Progressing"); condition != nil {
    log.Printf("Progressing: %s - %s", condition.Reason, condition.Message)
}

if !Has(&resource.Status, "Available") {
    MarkUnknown(&resource.Status, "Available", WithReason("NotYetChecked"))
}
```

## Design Decisions

### Why No Manager Type?

Earlier designs included a `Manager` type that wrapped an `Accessor`. However, this added an unnecessary layer of indirection. The functional approach is simpler:

```go
// Manager approach (rejected)
manager := NewManager(accessor)
manager.MarkTrue("Ready")

// Functional approach (chosen)
MarkTrue(accessor, "Ready")
```

The functional approach aligns with the "functions over interfaces" philosophy in the development guidelines.

### Why Accessor Instead of Direct Slice Manipulation?

The `Accessor` interface provides:
- Uniform API across different resource types
- Compatibility with resources that compute conditions dynamically
- Future flexibility for resources with custom condition storage

The `NewAccessor` helper ensures compatibility with simple slice-based storage.

### Why Delegate to meta Package?

Kubernetes provides battle-tested condition utilities in `k8s.io/apimachinery/pkg/api/meta`. Reusing them:
- Avoids duplicating well-tested logic
- Ensures compatibility with Kubernetes conventions
- Reduces maintenance burden
- Follows the guideline to "reuse Kubernetes libraries"

The convenience aliases (`IsTrue`, `Get`, etc.) provide consistent naming within this package while delegating to the standard library.

### Why Interface-Based Options?

Following controller-runtime's pattern:
- Provides consistency with the broader ecosystem
- Allows both concrete types and struct-based options
- Enables future extensibility without breaking changes
- Supports composition of options

## Future Enhancements

The package is designed for incremental enhancement:

1. **Custom Aggregation Strategies**
   - Add `WithMergeStrategy` option for custom aggregation logic
   - Support weighted conditions or priority-based aggregation

2. **Negative Polarity Support**
   - Add `WithNegativePolarity` option for conditions like `ErrorFound=False`
   - Automatically invert condition status in aggregation

3. **Condition Templates**
   - Predefined reason/message templates for common scenarios
   - Type-safe condition type constants

4. **Batch Operations**
   - Functions to set multiple conditions atomically
   - Bulk aggregation across multiple resources

5. **Observability**
   - Metrics for condition state changes
   - Event emission for condition transitions

## References

- [Kubernetes API Conventions - Typical Status Properties](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties)
- [Cluster API Conditions Utilities](https://pkg.go.dev/sigs.k8s.io/cluster-api/util/conditions)
- [controller-runtime Options Pattern](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/client#ListOption)
