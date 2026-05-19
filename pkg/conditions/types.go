package conditions

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Getter interface defines the method to retrieve conditions from a Kubernetes resource.
type Getter interface {
	GetConditions() []metav1.Condition
}

// Setter interface defines the method to set conditions on a Kubernetes resource.
type Setter interface {
	SetConditions(conditions []metav1.Condition)
}

// Accessor interface combines Getter and Setter for full condition management.
type Accessor interface {
	Getter
	Setter
}

// conditionsAccessor provides an Accessor implementation for a conditions slice pointer.
// This allows working with structs that don't natively implement the Accessor interface.
type conditionsAccessor struct {
	conditions *[]metav1.Condition
}

// GetConditions returns the current conditions.
func (a *conditionsAccessor) GetConditions() []metav1.Condition {
	if a.conditions == nil {
		return nil
	}

	return *a.conditions
}

// SetConditions updates the conditions.
func (a *conditionsAccessor) SetConditions(conditions []metav1.Condition) {
	if a.conditions != nil {
		*a.conditions = conditions
	}
}

// NewAccessor creates an Accessor from a pointer to a conditions slice.
// This is useful for working with structs that have a []metav1.Condition field
// but don't implement the Accessor interface.
//
// Example:
//
//	type MyResource struct {
//	    Status MyResourceStatus
//	}
//
//	type MyResourceStatus struct {
//	    Conditions []metav1.Condition
//	}
//
//	resource := &MyResource{}
//	accessor := NewAccessor(&resource.Status.Conditions)
//	MarkTrue(accessor, "Ready", WithReason("Initialized"))
func NewAccessor(conditions *[]metav1.Condition) Accessor {
	return &conditionsAccessor{conditions: conditions}
}

// Common condition types following Kubernetes API conventions.
// These constants provide standardized condition type names for common scenarios.
const (
	// ConditionTypeReady indicates the resource is ready for use.
	// This is the most commonly used condition type across Kubernetes resources.
	// Status=True means the resource is fully operational.
	ConditionTypeReady = "Ready"

	// ConditionTypeAvailable indicates the resource is available for use.
	// Status=True means the resource can accept requests/traffic.
	// Similar to Ready but often used for services and deployments.
	ConditionTypeAvailable = "Available"

	// ConditionTypeProgressing indicates the resource is actively being reconciled.
	// Status=True means work is in progress, False when reconciliation is complete.
	// Commonly used to show deployment or update progress.
	ConditionTypeProgressing = "Progressing"

	// ConditionTypeDegraded indicates the resource is operational but with reduced functionality.
	// Status=True means the resource is degraded.
	// This condition often coexists with Ready=True but signals reduced capacity.
	ConditionTypeDegraded = "Degraded"

	// ConditionTypeDependenciesReady indicates all dependencies are ready.
	// Status=True means all required dependencies are available and ready.
	ConditionTypeDependenciesReady = "DependenciesReady"
)

// Common condition reasons that can be used across different condition types.
const (
	// ReasonReconciling indicates reconciliation is actively in progress.
	ReasonReconciling = "Reconciling"

	// ReasonReconcileSuccess indicates reconciliation completed successfully.
	ReasonReconcileSuccess = "ReconcileSuccess"

	// ReasonReconcileError indicates reconciliation failed with an error.
	ReasonReconcileError = "ReconcileError"

	// ReasonInitializing indicates initial setup is in progress.
	ReasonInitializing = "Initializing"

	// ReasonResourcesProvisioned indicates all managed resources have been created/updated.
	ReasonResourcesProvisioned = "ResourcesProvisioned"

	// ReasonConditionMissing indicates a contributing condition has not been reported yet.
	ReasonConditionMissing = "ConditionMissing"
)
