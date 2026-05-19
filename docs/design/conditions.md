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
- Provides polarity-aware aggregation via the `Manager` type

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
- `ReasonConditionMissing` - Contributing condition not yet reported

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

### Manager and Condition Computation

The `Manager` type provides polarity-aware condition aggregation. It computes a target
condition from contributing conditions, each of which has a defined polarity.

**Polarity:**
- **Positive polarity** (`PositivePolarity`): `Status=True` means healthy (e.g., Available, Ready)
- **Negative polarity** (`NegativePolarity`): `Status=True` means unhealthy (e.g., Degraded)

**Creating a Manager:**

```go
m := NewManager(
    ConditionTypeReady,
    PositivePolarity(ConditionTypeAvailable),
    PositivePolarity("ProvisioningSucceeded"),
    NegativePolarity(ConditionTypeDegraded),
)
```

**Setting conditions via Manager:**

```go
m.MarkTrue(accessor, ConditionTypeAvailable,
    WithReason("Reconciled"),
    WithObservedGeneration(obj.Generation))
m.MarkFalse(accessor, ConditionTypeDegraded,
    WithReason("NotDegraded"))
```

**Computing the target condition:**

```go
m.Compute(accessor, obj.Generation)
```

**Compute logic (short-circuit evaluation):**
1. If any contributor is missing → Target `Unknown` with `ReasonConditionMissing`
2. If any contributor is unhealthy (polarity-aware) → Target `False` with that condition's reason/message
3. If all contributors are healthy → Target `True` with reason `"Ready"` and message `"All conditions met"`

Contributor order matters: the first problem found determines the target's reason and message.

**Health determination by polarity:**
- Positive polarity: healthy when `Status == True`
- Negative polarity: healthy when `Status == False`

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

### Manager with Polarity

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

// Create manager at controller setup
m := NewManager(
    ConditionTypeReady,
    PositivePolarity(ConditionTypeAvailable),
    PositivePolarity("ProvisioningSucceeded"),
    NegativePolarity(ConditionTypeDegraded),
)

// During reconciliation - set component conditions
controller := &Controller{}
m.MarkTrue(&controller.Status, ConditionTypeAvailable,
    WithReason("Connected"))
m.MarkTrue(&controller.Status, "ProvisioningSucceeded",
    WithReason("Reconciled"))
m.MarkFalse(&controller.Status, ConditionTypeDegraded,
    WithReason("NotDegraded"))

// Compute target condition
m.Compute(&controller.Status, obj.Generation)

// Result: Ready=True (all healthy)
//   Available=True (positive, healthy)
//   ProvisioningSucceeded=True (positive, healthy)
//   Degraded=False (negative, healthy - False means not degraded)

// If Degraded becomes True:
m.MarkTrue(&controller.Status, ConditionTypeDegraded,
    WithReason("PartialFailure"),
    WithMessage("Cache subsystem degraded"))
m.Compute(&controller.Status, obj.Generation)

// Result: Ready=False with:
//   Reason="PartialFailure" (from first unhealthy contributor)
//   Message="Cache subsystem degraded"
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

### Why a Manager Type?

The `Manager` type serves a specific purpose: it stores contributor configuration (condition types and their polarities) so that `Compute()` can perform polarity-aware aggregation without requiring the caller to re-specify contributors on every call.

This is different from a simple wrapper around `Accessor` (which was previously rejected). The Manager adds real value:
- Stores polarity metadata for each contributor
- Provides `Compute()` for polarity-aware aggregation with short-circuit evaluation
- Created once at controller setup, reused throughout reconciliation

Package-level functions (`MarkTrue`, `MarkFalse`, `Get`, etc.) remain available for direct condition manipulation without a Manager.

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

2. **Condition Templates**
   - Predefined reason/message templates for common scenarios
   - Type-safe condition type constants

3. **Batch Operations**
   - Functions to set multiple conditions atomically
   - Bulk aggregation across multiple resources

4. **Observability**
   - Metrics for condition state changes
   - Event emission for condition transitions

## References

- [Kubernetes API Conventions - Typical Status Properties](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties)
- [Cluster API Conditions Utilities](https://pkg.go.dev/sigs.k8s.io/cluster-api/util/conditions)
- [controller-runtime Options Pattern](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/client#ListOption)
