package reconciler

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// AsReconciler wraps a TypedReconciler into a controller-runtime reconcile.Reconciler.
// It handles:
//   - Fetching the object from the API server
//   - Injecting the controller name into the context
//   - Converting the Response to reconcile.Result
//
// The controllerName parameter is injected into the context for metrics and logging.
//
// Example:
//
//	func myReconciler(ctx context.Context, req *reconciler.TypedRequest[*v1.MyApp]) (*reconciler.Response, error) {
//	    // reconciliation logic
//	    return reconciler.NewResponse(), nil
//	}
//
//	r := reconciler.AsReconciler(client, "myapp-controller", reconciler.TypedReconcilerFunc[*v1.MyApp](myReconciler))
func AsReconciler[T ManagedObject](
	c client.Client,
	controllerName string,
	typedReconciler TypedReconciler[T],
) reconcile.Reconciler {
	return &typedReconcilerAdapter[T]{
		client:          c,
		controllerName:  controllerName,
		typedReconciler: typedReconciler,
	}
}

// typedReconcilerAdapter adapts a TypedReconciler to reconcile.Reconciler.
type typedReconcilerAdapter[T ManagedObject] struct {
	client          client.Client
	controllerName  string
	typedReconciler TypedReconciler[T]
}

// Reconcile implements reconcile.Reconciler.
func (a *typedReconcilerAdapter[T]) Reconcile(
	ctx context.Context,
	req reconcile.Request,
) (reconcile.Result, error) {
	// Inject controller name into context
	ctx = WithControllerName(ctx, a.controllerName)

	// Create new instance of T using reflection to handle pointer types correctly
	// If T is *SomeType, this creates a new SomeType and returns *SomeType
	obj, ok := reflect.New(reflect.TypeFor[T]().Elem()).Interface().(T)
	if !ok {
		return reconcile.Result{}, fmt.Errorf("failed to create object of type %T", obj)
	}
	if err := a.client.Get(ctx, req.NamespacedName, obj); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get object: %w", err)
	}

	// Create typed request
	typedReq := &TypedRequest[T]{
		Client: a.client,
		Object: obj,
	}

	// Call user's reconcile function
	resp, err := a.typedReconciler.Reconcile(ctx, typedReq)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("reconciler failed: %w", err)
	}

	// Convert Response to reconcile.Result
	result := reconcile.Result{}
	if requeueAfter := resp.ShouldRequeue(); requeueAfter > 0 {
		result.RequeueAfter = requeueAfter
	}

	return result, nil
}

// ResultFromResponse converts a Response to a reconcile.Result.
// This helper is useful when wrapping custom reconcilers that return Response.
func ResultFromResponse(resp *Response) reconcile.Result {
	result := reconcile.Result{}
	if requeueAfter := resp.ShouldRequeue(); requeueAfter > 0 {
		result.RequeueAfter = requeueAfter
	}

	return result
}

// ResponseFromResult converts a reconcile.Result to a Response.
// This helper is useful when adapting between different reconciler styles.
func ResponseFromResult(result reconcile.Result) *Response {
	resp := NewResponse()
	if result.RequeueAfter > 0 {
		resp.Requeue(result.RequeueAfter)
	} else if result.Requeue { //nolint:staticcheck // Requeue is deprecated but still valid for backward compatibility
		// If Requeue is true without duration, use a small default
		resp.Requeue(1 * time.Second)
	}

	return resp
}
