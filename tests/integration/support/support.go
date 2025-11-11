package support

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// NoOpReconciler is a no-op reconciler for testing.
type NoOpReconciler struct{}

// Reconcile implements the reconcile.Reconciler interface with a no-op implementation.
// Always returns an empty result and no error.
func (r *NoOpReconciler) Reconcile(
	_ context.Context,
	_ reconcile.Request,
) (reconcile.Result, error) {
	return reconcile.Result{}, nil
}
