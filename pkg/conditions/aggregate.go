package conditions

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Aggregate computes the status of a target condition by aggregating the status
// of contributing conditions from the given Accessor.
//
// The aggregation logic examines all contributing conditions and determines the
// target condition's status based on their collective state. Options can be
// provided to customize the aggregation behavior.
//
// Aggregation Logic:
//   - If any contributing condition is False, the target is False
//     Uses the first False condition's reason and message
//   - If any contributing condition is Unknown (and none are False), the target is Unknown
//     Uses the first Unknown condition's reason and message if available
//   - If all contributing conditions are True, the target is True
//   - If no contributing conditions are provided (empty list), the target is True
//   - Missing conditions are treated as Unknown
//
// Default Reason and Message:
//   - If no reason/message is found from contributing conditions, defaults to the target condition type
//   - Can be customized using WithDefaultReason and WithDefaultMessage options
//
// Example:
//
//		accessor := NewAccessor(&resource.Status.Conditions)
//
//		// Basic aggregation - defaults to target name for reason/message
//		Aggregate(accessor, "Ready", []string{"DatabaseReady", "CacheReady", "APIReady"})
//
//		// Custom default reason/message
//		Aggregate(
//	  	accessor,
//	 	"Ready",
//	 	[]string{"DatabaseReady", "CacheReady"},
//		    WithDefaultReason("AllHealthy"),
//		    WithDefaultMessage("All components are operational"),
//		)
func Aggregate(
	accessor Accessor,
	target string,
	contributing []string,
	opts ...AggregateOption,
) {
	aggregateOpts := AggregateOptions{
		DefaultReason:  target,
		DefaultMessage: target,
	}

	for _, opt := range opts {
		opt.ApplyToAggregate(&aggregateOpts)
	}

	condition := metav1.Condition{}
	condition.Type = target
	condition.Status = metav1.ConditionTrue

	if f := FirstFalse(accessor, contributing); f != nil {
		condition.Status = metav1.ConditionFalse
		condition.Reason = f.Reason
		condition.Message = f.Message
	} else if u := FirstUnknown(accessor, contributing); u != nil {
		condition.Status = metav1.ConditionUnknown
		condition.Reason = u.Reason
		condition.Message = u.Message
	}

	if condition.Reason == "" {
		condition.Reason = aggregateOpts.DefaultReason
	}
	if condition.Message == "" {
		condition.Message = aggregateOpts.DefaultMessage
	}

	conditions := accessor.GetConditions()
	meta.SetStatusCondition(&conditions, condition)
	accessor.SetConditions(conditions)
}
