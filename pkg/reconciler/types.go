package reconciler

import (
	"context"
	"fmt"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/status"
)

// ManagedObject combines client.Object with status.Accessor.
// Objects reconciled by the pipeline must implement this interface to support
// automatic status management and condition updates.
type ManagedObject interface {
	client.Object
	status.Accessor
}

// TypedRequest provides type-safe access to the reconciled object.
// The generic constraint ensures T is a ManagedObject at compile time.
type TypedRequest[T ManagedObject] struct {
	Client client.Client
	Object T
}

// Request is an alias for TypedRequest with ManagedObject.
// Logger should be retrieved from context using log.FromContext(ctx).
// For type-safe access to specific object types, use TypedRequest[T] directly.
type Request = TypedRequest[ManagedObject]

// objectEntry pairs an object with its per-object options.
type objectEntry struct {
	obj  client.Object
	opts ObjectOptions
}

// ObjectEntry is the exported view of an object entry with its per-object options.
type ObjectEntry struct {
	Object  client.Object
	Options ObjectOptions
}

// Response collects objects to provision and controls reconciliation flow.
// Actions populate the response during execution, and the framework uses
// it to manage resources and determine requeue behavior.
type Response struct {
	entries      []objectEntry
	requeueAfter time.Duration
}

// NewResponse creates a new Response instance.
func NewResponse() *Response {
	return &Response{
		entries: make([]objectEntry, 0),
	}
}

// Objects adds multiple objects to be provisioned with default options.
// Returns the response for method chaining.
func (r *Response) Objects(objs ...client.Object) *Response {
	for _, obj := range objs {
		r.entries = append(r.entries, objectEntry{obj: obj})
	}

	return r
}

// Object adds a single object with per-object options.
// Use WithOwnership(false) to skip OwnerReferences for this specific object.
// Returns the response for method chaining.
func (r *Response) Object(obj client.Object, opts ...ObjectOption) *Response {
	entry := objectEntry{obj: obj}

	for _, opt := range opts {
		opt.ApplyTo(&entry.opts)
	}

	r.entries = append(r.entries, entry)

	return r
}

// Requeue schedules the reconciliation to be requeued after the specified duration.
// Returns the response for method chaining.
func (r *Response) Requeue(duration time.Duration) *Response {
	r.requeueAfter = duration

	return r
}

// GetObjects returns the list of all objects to be provisioned.
func (r *Response) GetObjects() []client.Object {
	result := make([]client.Object, len(r.entries))
	for i, e := range r.entries {
		result[i] = e.obj
	}

	return result
}

// GetEntries returns object entries with their per-object options.
func (r *Response) GetEntries() []ObjectEntry {
	result := make([]ObjectEntry, len(r.entries))
	for i, e := range r.entries {
		result[i] = ObjectEntry{Object: e.obj, Options: e.opts}
	}

	return result
}

// ShouldRequeue returns whether reconciliation should be requeued and the duration.
func (r *Response) ShouldRequeue() time.Duration {
	return r.requeueAfter
}

// TypedActionFunc is a type-safe action that works with a specific object type.
// Use ToActionFunc to convert it to an ActionFunc for use with Pipeline.
type TypedActionFunc[T ManagedObject] func(ctx context.Context, req *TypedRequest[T], resp *Response) error

// ActionFunc is an alias for TypedActionFunc with ManagedObject.
// For type-safe actions on specific object types, use TypedActionFunc[T] and convert
// with ToActionFunc or WithTypedActions.
type ActionFunc = TypedActionFunc[ManagedObject]

// TypedCleanupFunc is a type-safe cleanup action that works with a specific object type.
// Use ToCleanupFunc to convert it to a CleanupFunc for use with Pipeline.
type TypedCleanupFunc[T ManagedObject] func(ctx context.Context, req *TypedRequest[T]) error

// CleanupFunc is an alias for TypedCleanupFunc with ManagedObject.
// Cleanup actions have a simpler signature than regular actions since they don't
// need to accumulate objects or control requeue behavior.
// For type-safe cleanup actions on specific object types, use TypedCleanupFunc[T] and
// convert with ToCleanupFunc or WithTypedCleanup.
type CleanupFunc = TypedCleanupFunc[ManagedObject]

// TypedReconciler is a type-safe reconciler interface that works with a specific object type.
// It receives a context with the controller name injected and a typed request with the fetched object.
// Returns a Response controlling requeue behavior and objects to provision, or an error.
type TypedReconciler[T ManagedObject] interface {
	Reconcile(ctx context.Context, req *TypedRequest[T]) (*Response, error)
}

// TypedReconcilerFunc is a function adapter that implements TypedReconciler.
type TypedReconcilerFunc[T ManagedObject] func(ctx context.Context, req *TypedRequest[T]) (*Response, error)

// Reconcile implements TypedReconciler interface.
func (f TypedReconcilerFunc[T]) Reconcile(ctx context.Context, req *TypedRequest[T]) (*Response, error) {
	return f(ctx, req)
}

// Wrap converts a reconciler function to a TypedReconciler interface.
// This is a convenience function to avoid explicit type conversion in Complete() calls.
//
// Example:
//
//	fn := func(ctx context.Context, req *reconciler.TypedRequest[*v1.MyApp]) (*reconciler.Response, error) {
//	    return reconciler.NewResponse(), nil
//	}
//	b.Complete(reconciler.Wrap(fn))
func Wrap[T ManagedObject](fn func(context.Context, *TypedRequest[T]) (*Response, error)) TypedReconciler[T] {
	return TypedReconcilerFunc[T](fn)
}

// ApplyHookFunc processes provisioned objects at specific stages of the pipeline.
// Used with WithPreApply (before objects are applied) and WithPostApply (after).
type ApplyHookFunc func(ctx context.Context, owner client.Object, objects []client.Object) error

// ControllerAware is an optional interface that reconcilers can implement
// to receive the controller instance after it's created.
// The builder will automatically inject the controller via SetController()
// if the reconciler implements this interface.
type ControllerAware interface {
	SetController(ctrl controller.Controller)
}

// ClientAware is an optional interface that reconcilers can implement
// to receive the client instance.
// The builder will automatically inject the client via SetClient()
// if the reconciler implements this interface.
type ClientAware interface {
	SetClient(c client.Client)
}

// CacheAware is an optional interface that reconcilers can implement
// to receive the cache instance.
// The builder will automatically inject the cache via SetCache()
// if the reconciler implements this interface.
// This is primarily used by TypedPipeline for watch support.
type CacheAware interface {
	SetCache(c cache.Cache)
}

// ToActionFunc converts a TypedActionFunc to an ActionFunc with compile-time type safety.
// The generic constraint T ManagedObject ensures the typed action can only be created
// with types that implement ManagedObject.
//
// Runtime safety: The returned ActionFunc will return an error if req.Object cannot be cast to T.
// This should never happen if the Pipeline is used correctly (only reconciling objects of type T).
func ToActionFunc[T ManagedObject](typedAction TypedActionFunc[T]) ActionFunc {
	return func(ctx context.Context, req *Request, resp *Response) error {
		// Type assertion - return error if Object is not of type T
		obj, ok := req.Object.(T)
		if !ok {
			var zero T

			return fmt.Errorf("type assertion failed: expected %T, got %T", zero, req.Object)
		}

		typedReq := &TypedRequest[T]{
			Client: req.Client,
			Object: obj,
		}

		return typedAction(ctx, typedReq, resp)
	}
}

// ToCleanupFunc converts a TypedCleanupFunc to a CleanupFunc with compile-time type safety.
// The generic constraint T ManagedObject ensures the typed cleanup action can only be created
// with types that implement ManagedObject.
//
// Runtime safety: The returned CleanupFunc will return an error if req.Object cannot be cast to T.
func ToCleanupFunc[T ManagedObject](typedCleanup TypedCleanupFunc[T]) CleanupFunc {
	return func(ctx context.Context, req *Request) error {
		// Type assertion - return error if Object is not of type T
		obj, ok := req.Object.(T)
		if !ok {
			var zero T

			return fmt.Errorf("type assertion failed: expected %T, got %T", zero, req.Object)
		}

		typedReq := &TypedRequest[T]{
			Client: req.Client,
			Object: obj,
		}

		return typedCleanup(ctx, typedReq)
	}
}
