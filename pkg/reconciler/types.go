package reconciler

import (
	"context"
	"fmt"
	"time"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/status"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ManagedObject combines client.Object with status.Accessor.
// Objects reconciled by the pipeline must implement this interface to support
// automatic status management and condition updates.
type ManagedObject interface {
	client.Object
	status.Accessor
}

// Request provides resources for action execution.
// Logger should be retrieved from context using log.FromContext(ctx).
type Request struct {
	Client client.Client
	Object ManagedObject
}

// TypedRequest provides type-safe access to the reconciled object.
// The generic constraint ensures T is a ManagedObject at compile time.
type TypedRequest[T ManagedObject] struct {
	Client client.Client
	Object T
}

// Response collects objects to provision and controls reconciliation flow.
// Actions populate the response during execution, and the framework uses
// it to manage resources and determine requeue behavior.
type Response struct {
	objects      []client.Object
	requeue      bool
	requeueAfter time.Duration
}

// NewResponse creates a new Response instance.
func NewResponse() *Response {
	return &Response{
		objects: make([]client.Object, 0),
	}
}

// Objects adds objects to be provisioned by the framework.
// The framework will apply these objects using server-side apply
// and track them for garbage collection.
// Returns the response for method chaining.
func (r *Response) Objects(objs ...client.Object) *Response {
	r.objects = append(r.objects, objs...)
	return r
}

// Requeue marks the reconciliation for immediate requeue.
// Returns the response for method chaining.
func (r *Response) Requeue() *Response {
	r.requeue = true
	r.requeueAfter = 0
	return r
}

// RequeueAfter schedules the reconciliation to be requeued after the specified duration.
// Returns the response for method chaining.
func (r *Response) RequeueAfter(duration time.Duration) *Response {
	r.requeue = true
	r.requeueAfter = duration
	return r
}

// GetObjects returns the list of objects to be provisioned.
// Used internally by the framework.
func (r *Response) GetObjects() []client.Object {
	return r.objects
}

// ShouldRequeue returns whether reconciliation should be requeued and the duration.
// Used internally by the framework to determine reconcile.Result.
func (r *Response) ShouldRequeue() (bool, time.Duration) {
	return r.requeue, r.requeueAfter
}

// ActionFunc defines the signature for reconciliation actions.
type ActionFunc func(ctx context.Context, req *Request, resp *Response) error

// CleanupFunc defines the signature for cleanup actions.
// Cleanup actions have a simpler signature than regular actions since they don't
// need to accumulate objects or control requeue behavior.
type CleanupFunc func(ctx context.Context, req *Request) error

// TypedActionFunc is a type-safe action that works with a specific object type.
// Use ToActionFunc to convert it to an ActionFunc for use with Pipeline.
type TypedActionFunc[T ManagedObject] func(ctx context.Context, req *TypedRequest[T], resp *Response) error

// TypedCleanupFunc is a type-safe cleanup action that works with a specific object type.
// Use ToCleanupFunc to convert it to a CleanupFunc for use with Pipeline.
type TypedCleanupFunc[T ManagedObject] func(ctx context.Context, req *TypedRequest[T]) error

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
