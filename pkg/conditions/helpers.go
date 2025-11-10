package conditions

// MarkAvailable sets the Available condition to True with the given reason.
// This is a convenience wrapper around MarkTrue for the Available condition type.
// For additional options (e.g., WithObservedGeneration), use MarkTrue directly.
func MarkAvailable(accessor Accessor, reason string) {
	MarkTrue(accessor, ConditionTypeAvailable, WithReason(reason))
}

// MarkProgressing sets the Progressing condition to True with the given reason and message.
// Use this to indicate that reconciliation is actively in progress.
// The message should describe what operation is currently being performed.
// For additional options (e.g., WithObservedGeneration), use MarkTrue directly.
func MarkProgressing(accessor Accessor, reason string, message string) {
	MarkTrue(accessor, ConditionTypeProgressing, WithReason(reason), WithMessage(message))
}

// MarkDegraded sets the Degraded condition to True with the given reason and message.
// Use this to indicate the resource is operational but with reduced functionality.
// The message should describe what functionality is degraded or impaired.
// For additional options (e.g., WithObservedGeneration), use MarkTrue directly.
func MarkDegraded(accessor Accessor, reason string, message string) {
	MarkTrue(accessor, ConditionTypeDegraded, WithReason(reason), WithMessage(message))
}
