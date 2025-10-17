package conditions

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MarkTrue sets a condition to status True on the given Accessor.
// It uses the current time as LastTransitionTime and applies any provided options.
func MarkTrue(
	accessor Accessor,
	conditionType string,
	opts ...ConditionOption,
) {
	Set(accessor, conditionType, metav1.ConditionTrue, opts...)
}

// MarkFalse sets a condition to status False on the given Accessor.
// It uses the current time as LastTransitionTime and applies any provided options.
func MarkFalse(
	accessor Accessor,
	conditionType string,
	opts ...ConditionOption,
) {
	Set(accessor, conditionType, metav1.ConditionFalse, opts...)
}

// MarkUnknown sets a condition to status Unknown on the given Accessor.
// It uses the current time as LastTransitionTime and applies any provided options.
func MarkUnknown(
	accessor Accessor,
	conditionType string,
	opts ...ConditionOption,
) {
	Set(accessor, conditionType, metav1.ConditionUnknown, opts...)
}

// Set is the internal function that creates and sets a condition.
func Set(
	accessor Accessor,
	conditionType string,
	status metav1.ConditionStatus,
	opts ...ConditionOption,
) {
	condition := metav1.Condition{
		Type:   conditionType,
		Status: status,
	}

	// Apply options
	condOpts := &ConditionOptions{}
	condOpts.ApplyOptions(opts)

	if condOpts.Reason != "" {
		condition.Reason = condOpts.Reason
	}
	if condOpts.Message != "" {
		condition.Message = condOpts.Message
	}
	if condOpts.ObservedGeneration != 0 {
		condition.ObservedGeneration = condOpts.ObservedGeneration
	}

	conditions := accessor.GetConditions()
	meta.SetStatusCondition(&conditions, condition)
	accessor.SetConditions(conditions)
}

// Convenience aliases for meta package functions to provide consistent naming.

// Get retrieves a condition by type from the Accessor.
// Returns nil if the condition is not found.
func Get(
	accessor Accessor,
	conditionType string,
) *metav1.Condition {
	return meta.FindStatusCondition(accessor.GetConditions(), conditionType)
}

// Has checks if a condition exists in the Accessor.
func Has(
	accessor Accessor,
	conditionType string,
) bool {
	return meta.FindStatusCondition(accessor.GetConditions(), conditionType) != nil
}

// IsTrue checks if a condition exists and has status True.
func IsTrue(
	accessor Accessor,
	conditionType string,
) bool {
	return meta.IsStatusConditionTrue(accessor.GetConditions(), conditionType)
}

// IsFalse checks if a condition exists and has status False.
func IsFalse(
	accessor Accessor,
	conditionType string,
) bool {
	return meta.IsStatusConditionFalse(accessor.GetConditions(), conditionType)
}

// Remove removes a condition by type from the Accessor.
func Remove(
	accessor Accessor,
	conditionType string,
) {
	conditions := accessor.GetConditions()
	meta.RemoveStatusCondition(&conditions, conditionType)
	accessor.SetConditions(conditions)
}

// FirstFalse returns the first condition with status False from the given condition types.
// Returns nil if no False condition is found.
func FirstFalse(
	accessor Accessor,
	conditionTypes []string,
) *metav1.Condition {
	for _, conditionType := range conditionTypes {
		condition := Get(accessor, conditionType)
		if condition != nil && condition.Status == metav1.ConditionFalse {
			return condition
		}
	}
	return nil
}

// FirstUnknown returns the first condition with status Unknown from the given condition types.
// Returns nil if no Unknown condition is found or if conditions are missing.
func FirstUnknown(
	accessor Accessor,
	conditionTypes []string,
) *metav1.Condition {
	for _, conditionType := range conditionTypes {
		condition := Get(accessor, conditionType)
		if condition != nil && condition.Status == metav1.ConditionUnknown {
			return condition
		}
	}
	return nil
}
