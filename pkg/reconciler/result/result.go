package result

import (
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	// DefaultRequeueDelay is the default delay for RequeueIf when condition is true.
	DefaultRequeueDelay = 1 * time.Second
)

// Result wraps ctrl.Result to provide a fluent API.
type Result struct {
	ctrl.Result
}

// After sets a custom requeue delay, overriding any default.
// This enables a fluent API for customizing requeue behavior.
//
// Example:
//
//	return result.RequeueIf(func() bool {
//	    return !ready
//	}).After(5 * time.Second), nil
func (r Result) After(duration time.Duration) Result {
	r.RequeueAfter = duration

	return r
}

// RequeueAfter returns a Result that requeues the reconciliation after the given duration.
//
// Example:
//
//	if !dependencyReady {
//	    return result.RequeueAfter(30*time.Second), nil
//	}
func RequeueAfter(duration time.Duration) Result {
	return Result{ctrl.Result{
		RequeueAfter: duration,
	}}
}

// RequeueIf conditionally requeues the reconciliation based on the result of the condition function.
// If the condition function returns true, it requeues after DefaultRequeueDelay (1 second).
// If the condition function returns false, it returns a successful result with no requeue.
// Use .After() to customize the requeue delay.
//
// Example:
//
//	return result.RequeueIf(func() bool {
//	    return !podsReady
//	}), nil
//
//	// With custom delay:
//	return result.RequeueIf(func() bool {
//	    return !podsReady
//	}).After(5 * time.Second), nil
func RequeueIf(condition func() bool) Result {
	if condition() {
		return RequeueAfter(DefaultRequeueDelay)
	}

	return Success()
}

// Success returns a successful reconciliation result with no requeue.
// This indicates that reconciliation completed successfully and no further action is needed
// until the next watch event triggers reconciliation.
//
// Example:
//
//	// All work completed successfully
//	return result.Success(), nil
func Success() Result {
	return Result{ctrl.Result{}}
}
