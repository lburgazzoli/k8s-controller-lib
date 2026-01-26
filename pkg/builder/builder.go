/*
Copyright 2025 The k8s-controller-lib Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package builder provides a type-safe, generic controller builder that internally uses
// unstructured or partial object metadata for watches while providing a clean typed API.
//
// Key features:
//   - Type-safe generic APIs with automatic type conversion for For() (primary resource)
//   - GVK-based Watches() and Owns() for zero conversion overhead
//   - Direct controller.Watch() calls for transparency
//   - Compatible with Pipeline auto-watch system
//   - Performance optimization via partial metadata for metadata-only watches
//
// Example usage:
//
//	b, err := builder.NewControllerBuilder[*v1.MyApp](mgr)
//	if err != nil {
//	    return err
//	}
//
//	return b.
//	    For(&v1.MyApp{}).
//	    Owns(gvks.ConfigMap).
//	    Owns(gvks.Secret, builder.WithPredicates(predicates.LabelChanged())).
//	    Complete(reconciler)
package builder

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	libreconciler "github.com/lburgazzoli/k8s-controller-lib/pkg/reconciler"
)

// Builder provides type-safe controller building with automatic type conversion.
// The generic type T constrains the primary resource type to ManagedObject.
type Builder[T libreconciler.ManagedObject] struct {
	ctrl    controller.Controller
	mgr     manager.Manager
	scheme  *runtime.Scheme
	cache   cache.Cache
	client  client.Client
	errors  []error
	forSet  bool // Track if For() was called
	name    string
	options *ControllerOptions
	watches []watchRegistration // Store watches to register after controller creation
}

// watchRegistration stores a pending watch to be registered.
type watchRegistration struct {
	obj        client.Object
	handler    handler.EventHandler
	predicates []predicate.Predicate
	asPartial  bool
}

// NewControllerBuilder creates a new Builder for constructing a controller.
//
// Parameters:
//   - mgr: Controller manager providing client, cache, and scheme
//   - opts: Optional controller configuration (Name, Cache, Client, MaxConcurrentReconciles, RateLimiter)
//
// The controller name is automatically derived from the For() object type if not explicitly
// provided via WithName(). For example, &v1.TestApp{} becomes "testapp".
//
// If a custom cache is provided via WithCache(), it will be used instead of the manager's cache.
// If a custom client is provided via WithClient(), it will be used instead of the manager's client.
//
// Returns a Builder instance ready for watch registration.
func NewControllerBuilder[T libreconciler.ManagedObject](
	mgr manager.Manager,
	opts ...ControllerOption,
) (*Builder[T], error) {
	if mgr == nil {
		return nil, errors.New("manager is required")
	}

	// Apply controller options
	ctrlOpts := &ControllerOptions{}
	ctrlOpts.ApplyOptions(opts)

	// Use custom cache if provided, otherwise use manager's cache
	cacheToUse := ctrlOpts.Cache
	if cacheToUse == nil {
		cacheToUse = mgr.GetCache()
	}

	// Use custom client if provided, otherwise use manager's client
	clientToUse := ctrlOpts.Client
	if clientToUse == nil {
		clientToUse = mgr.GetClient()
	}

	return &Builder[T]{
		ctrl:    nil, // Created in Complete() with reconciler
		mgr:     mgr,
		scheme:  mgr.GetScheme(),
		cache:   cacheToUse,
		client:  clientToUse,
		errors:  make([]error, 0),
		forSet:  false,
		name:    ctrlOpts.Name, // May be empty, will be set in For() or validated in Complete()
		options: ctrlOpts,
	}, nil
}

// For registers the primary resource type to reconcile.
// This method must be called exactly once before Complete().
//
// The object parameter must match the Builder's type parameter T.
// For example, Builder[*v1.MyApp] requires For(&v1.MyApp{}).
// This enforces type safety at compile time.
//
// If no controller name was provided via WithName(), the name is automatically
// derived from the object type (e.g., *v1.MyApp → "myapp").
//
// Options control predicates and whether to use partial metadata.
// No handler is specified for the primary resource - it uses the reconciler.
func (b *Builder[T]) For(
	obj T,
	opts ...WatchOption,
) *Builder[T] {
	if b.forSet {
		b.errors = append(b.errors, errors.New("For() can only be called once"))

		return b
	}
	b.forSet = true

	// Derive controller name from object type if not already set
	if b.name == "" {
		b.name = deriveControllerNameFromObject(obj)
	}

	// Process options
	watchOpts := &WatchOptions{}
	watchOpts.ApplyOptions(opts)

	// For() should not have handler or mapper
	if watchOpts.Handler != nil {
		b.errors = append(b.errors, errors.New("For() does not support WithHandler"))

		return b
	}

	if watchOpts.hasMapper() {
		b.errors = append(b.errors, errors.New("For() does not support WithMapper"))

		return b
	}

	// For() uses the default enqueue handler
	h := &handler.TypedEnqueueRequestForObject[client.Object]{}

	// Register watch
	b.registerWatch(obj, h, watchOpts.Predicates, watchOpts.isPartial())

	return b
}

// Owns registers an owned resource with default EnqueueRequestForOwner handler using a GVK.
// Events from owned resources trigger reconciliation of their owner.
//
// By default, watches use unstructured objects (zero conversion overhead).
// Use AsPartial() option for partial metadata watches (even lower memory usage).
// Custom handlers can be provided via WithHandler option.
// WithMapper is not allowed for Owns (only for Watches).
//
// Example:
//
//	builder.Owns(gvks.ConfigMap)
//	builder.Owns(gvks.Secret, builder.WithPredicates(predicates.LabelChanged()))
//	builder.Owns(gvks.ConfigMap, builder.WithHandler(customHandler))
func (b *Builder[T]) Owns(
	gvk schema.GroupVersionKind,
	opts ...WatchOption,
) *Builder[T] {
	// Process options
	watchOpts := &WatchOptions{}
	watchOpts.ApplyOptions(opts)

	// Owns() does not support mapper
	if watchOpts.hasMapper() {
		b.errors = append(b.errors, errors.New("Owns() does not support WithMapper; use Watches() instead"))

		return b
	}

	// Validate GVK
	if err := validateGVK(gvk, "Owns"); err != nil {
		b.errors = append(b.errors, err)

		return b
	}

	// Create watch object based on Strategy option - no conversion needed
	var watchObj client.Object
	if watchOpts.isPartial() {
		p := &metav1.PartialObjectMetadata{}
		p.SetGroupVersionKind(gvk)
		watchObj = p
	} else {
		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(gvk)
		watchObj = u
	}

	// Default handler if none provided
	h := watchOpts.Handler
	if h == nil {
		// Create a new instance of T using reflection
		// T is a pointer type (e.g., *TestApp), so we need to create the underlying value
		ownerType, ok := reflect.New(reflect.TypeFor[T]().Elem()).Interface().(client.Object)
		if !ok {
			b.errors = append(b.errors, errors.New("failed to create owner type instance"))

			return b
		}

		h = handler.EnqueueRequestForOwner(
			b.scheme,
			b.mgr.GetRESTMapper(),
			ownerType,
			handler.OnlyControllerOwner(),
		)
	}

	// Resolve typed predicate factories and combine with regular predicates
	predicates := b.resolvePredicates(watchOpts)

	b.registerWatchGVK(watchObj, h, predicates)

	return b
}

// Watches registers a custom watch with explicit handler or mapper using a GVK.
// Either WithHandler or WithMapper must be provided, but not both.
//
// By default, watches use unstructured objects (zero conversion overhead).
// Use AsPartial() option for partial metadata watches (even lower memory usage).
//
// WithMapper is a convenience for simple mapping functions:
//
//	builder.Watches(gvks.Secret,
//	    builder.WithMapper(func(ctx context.Context, u *unstructured.Unstructured) []reconcile.Request {
//	        // Return reconcile requests based on secret
//	        return []reconcile.Request{{NamespacedName: types.NamespacedName{...}}}
//	    }),
//	)
//
// WithHandler provides full control for complex event handling:
//
//	builder.Watches(gvks.Pod, builder.WithHandler(customHandler))
func (b *Builder[T]) Watches(
	gvk schema.GroupVersionKind,
	opts ...WatchOption,
) *Builder[T] {
	// Process options
	watchOpts := &WatchOptions{}
	watchOpts.ApplyOptions(opts)

	// Validate mapper XOR handler
	if watchOpts.hasMapper() && watchOpts.Handler != nil {
		b.errors = append(b.errors, errors.New("WithMapper and WithHandler are mutually exclusive"))

		return b
	}

	if !watchOpts.hasMapper() && watchOpts.Handler == nil {
		b.errors = append(b.errors, errors.New("Watches() requires either WithMapper or WithHandler"))

		return b
	}

	// Validate GVK
	if err := validateGVK(gvk, "Watches"); err != nil {
		b.errors = append(b.errors, err)

		return b
	}

	// Validate mapper expectation matches watch type
	if watchOpts.hasMapper() {
		if err := b.validateMapperExpectation(watchOpts); err != nil {
			b.errors = append(b.errors, err)

			return b
		}
	}

	// Create watch object based on Strategy option - no conversion needed
	var watchObj client.Object
	if watchOpts.isPartial() {
		p := &metav1.PartialObjectMetadata{}
		p.SetGroupVersionKind(gvk)
		watchObj = p
	} else {
		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(gvk)
		watchObj = u
	}

	// Resolve typed predicate factories and combine with regular predicates
	allPredicates := b.resolvePredicates(watchOpts)

	// Determine handler
	var h handler.EventHandler
	var predicatesToRegister []predicate.Predicate

	if watchOpts.hasMapper() {
		// GVK-based watches never need conversion - we're watching unstructured/partial directly
		// Pass controller name for metrics labeling and predicates for internal evaluation.
		h = watchOpts.mapperFactory.Create(b.scheme, false, b.name, allPredicates)
		// Predicates handled internally by typedMapperHandler
		predicatesToRegister = nil
	} else {
		h = watchOpts.Handler
		predicatesToRegister = allPredicates
	}

	b.registerWatchGVK(watchObj, h, predicatesToRegister)

	return b
}

// Complete finalizes the builder by creating the controller with a typed reconciler.
// The reconciler will receive:
//   - A context with the controller name injected via reconciler.WithControllerName
//   - A TypedRequest with the fetched object of type T
//
// The builder automatically handles:
//   - Fetching the object from the API server
//   - Injecting the controller name into the context for metrics and logging
//   - Converting the Response to controller-runtime's reconcile.Result
//   - Optionally injecting controller and/or client if reconciler implements
//     ControllerAware and/or ClientAware interfaces
//
// Returns any accumulated errors from watch registration or validation.
// For() must have been called before Complete(), and a controller name must be set
// either via WithName() or derived from the For() object type.
//
// Example with function using Wrap helper:
//
//	fn := func(ctx context.Context, req *reconciler.TypedRequest[*v1.MyApp]) (*reconciler.Response, error) {
//	    return reconciler.NewResponse(), nil
//	}
//	b.For(&v1.MyApp{}).Complete(reconciler.Wrap(fn))
//
// Example with function using TypedReconcilerFunc:
//
//	b.For(&v1.MyApp{}).Complete(
//	    reconciler.TypedReconcilerFunc[*v1.MyApp](
//	        func(ctx context.Context, req *reconciler.TypedRequest[*v1.MyApp]) (*reconciler.Response, error) {
//	            return reconciler.NewResponse(), nil
//	        },
//	    ),
//	)
//
// Example with custom reconciler:
//
//	type MyReconciler struct{}
//
//	func (r *MyReconciler) Reconcile(
//	    ctx context.Context,
//	    req *reconciler.TypedRequest[*v1.MyApp],
//	) (*reconciler.Response, error) {
//	    return reconciler.NewResponse(), nil
//	}
//
//	b.For(&v1.MyApp{}).Complete(&MyReconciler{})
//
// Example with dependency injection:
//
//	type MyReconciler struct {
//	    ctrl   controller.Controller
//	    client client.Client
//	}
//
//	func (r *MyReconciler) SetController(ctrl controller.Controller) {
//	    r.ctrl = ctrl
//	}
//
//	func (r *MyReconciler) SetClient(c client.Client) {
//	    r.client = c
//	}
//
//	func (r *MyReconciler) Reconcile(
//	    ctx context.Context,
//	    req *reconciler.TypedRequest[*v1.MyApp],
//	) (*reconciler.Response, error) {
//	    // Use r.ctrl and r.client
//	    return reconciler.NewResponse(), nil
//	}
//
//	b.For(&v1.MyApp{}).Complete(&MyReconciler{})
func (b *Builder[T]) Complete(r libreconciler.TypedReconciler[T]) error {
	if !b.forSet {
		return errors.New("For() must be called before Complete()")
	}

	if b.name == "" {
		return errors.New("controller name is required: call For() or provide WithName()")
	}

	if len(b.errors) > 0 {
		return fmt.Errorf("builder has %d error(s): %w", len(b.errors), errors.Join(b.errors...))
	}

	// Wrap the typed reconciler with object fetching and context injection
	wrappedReconciler := libreconciler.AsReconciler(b.client, b.name, r)

	// Create controller configuration with wrapped reconciler
	config := controller.Options{
		Reconciler: wrappedReconciler,
	}

	if b.options.MaxConcurrentReconciles > 0 {
		config.MaxConcurrentReconciles = b.options.MaxConcurrentReconciles
	}

	if b.options.RateLimiter != nil {
		config.RateLimiter = b.options.RateLimiter
	}

	// Create controller
	ctrl, err := controller.New(b.name, b.mgr, config)
	if err != nil {
		return fmt.Errorf("failed to create controller: %w", err)
	}

	b.ctrl = ctrl

	// Inject dependencies if reconciler implements the aware interfaces
	if ca, ok := r.(libreconciler.ControllerAware); ok {
		ca.SetController(ctrl)
	}

	if cla, ok := r.(libreconciler.ClientAware); ok {
		cla.SetClient(b.client)
	}

	// Register all watches now that controller exists
	for _, watch := range b.watches {
		b.doRegisterWatch(watch.obj, watch.handler, watch.predicates, watch.asPartial)
	}

	// Check for any watch registration errors
	if len(b.errors) > 0 {
		return fmt.Errorf("failed to register watches: %w", errors.Join(b.errors...))
	}

	return nil
}

// GetController returns the underlying controller for integration with other systems.
// This is useful for Pipeline auto-watch integration.
func (b *Builder[T]) GetController() controller.Controller {
	return b.ctrl
}

// validateMapperExpectation validates that the mapper's expected type matches the watch configuration.
// Returns an error if mapper expects *unstructured.Unstructured but AsPartial() is set,
// mapper expects *metav1.PartialObjectMetadata but AsPartial() is not set,
// or mapper expects typed objects but AsPartial() is set.
func (b *Builder[T]) validateMapperExpectation(watchOpts *WatchOptions) error {
	exp := watchOpts.mapperFactory.Expectation

	//nolint:exhaustive // Default case handles MapperExpectsClientObject and unknown values
	switch exp {
	case MapperExpectsUnstructured:
		if watchOpts.isPartial() {
			return errors.New("mapper expects *unstructured.Unstructured but WatchPartial is set; " +
				"use WatchFull or a mapper that accepts *metav1.PartialObjectMetadata or client.Object")
		}
	case MapperExpectsPartial:
		if !watchOpts.isPartial() {
			return errors.New("mapper expects *metav1.PartialObjectMetadata but WatchFull is set; " +
				"use AsPartial() or a mapper that accepts *unstructured.Unstructured or client.Object")
		}
	case MapperExpectsTyped:
		if watchOpts.isPartial() {
			return errors.New("mapper expects typed object but WatchPartial is set; " +
				"partial metadata lacks data for conversion to typed objects; use WatchFull")
		}
	default:
		// MapperExpectsClientObject and unknown values - no validation needed
	}

	return nil
}

// validateGVK validates that a GVK is complete and usable for watches.
func validateGVK(gvk schema.GroupVersionKind, method string) error {
	if gvk.Kind == "" {
		return fmt.Errorf("%s(): GVK must have a Kind", method)
	}

	if gvk.Version == "" {
		return fmt.Errorf("%s(): GVK must have a Version", method)
	}

	// Group can be empty for core resources (e.g., v1/Pod)
	return nil
}

// resolvePredicates resolves typed predicate factories and combines them with regular predicates.
func (b *Builder[T]) resolvePredicates(watchOpts *WatchOptions) []predicate.Predicate {
	// Start with regular predicates
	result := make([]predicate.Predicate, 0, len(watchOpts.Predicates)+len(watchOpts.typedPredicateFactories))
	result = append(result, watchOpts.Predicates...)

	// Resolve typed predicate factories
	for _, factory := range watchOpts.typedPredicateFactories {
		result = append(result, factory(b.scheme, b.name))
	}

	return result
}

// registerWatch stores a watch registration to be completed during Complete().
// This is an internal method called by For().
// Validates AsPartial usage immediately for fail-fast behavior.
//
//nolint:revive // asPartial is a configuration flag, not control coupling
func (b *Builder[T]) registerWatch(
	obj client.Object,
	h handler.EventHandler,
	predicates []predicate.Predicate,
	asPartial bool,
) {
	// Validate AsPartial usage with typed objects immediately (fail-fast)
	if asPartial {
		switch any(obj).(type) {
		case *unstructured.Unstructured, *metav1.PartialObjectMetadata:
			// OK - these can use partial metadata
		default:
			b.errors = append(b.errors, fmt.Errorf(
				"AsPartial() cannot be used with typed object %T: "+
					"partial metadata conversion to typed objects results in incomplete data. "+
					"Use *unstructured.Unstructured or *metav1.PartialObjectMetadata instead",
				obj,
			))

			return
		}
	}

	// Store watch for later registration
	b.watches = append(b.watches, watchRegistration{
		obj:        obj,
		handler:    h,
		predicates: predicates,
		asPartial:  asPartial,
	})
}

// registerWatchGVK stores a watch registration for GVK-based watches (Owns/Watches).
// The watch object is already unstructured or partial metadata, so no conversion is needed.
func (b *Builder[T]) registerWatchGVK(
	watchObj client.Object,
	h handler.EventHandler,
	predicates []predicate.Predicate,
) {
	// Store watch for later registration - no conversion needed
	b.watches = append(b.watches, watchRegistration{
		obj:        watchObj,
		handler:    h,
		predicates: predicates,
		asPartial:  false, // not used for GVK watches since obj is already unstructured/partial
	})
}

// doRegisterWatch processes the object and registers a watch with the controller.
// This is called from Complete() after the controller is created.
// AsPartial validation is done earlier in registerWatch() for fail-fast behavior.
func (b *Builder[T]) doRegisterWatch(
	obj client.Object,
	h handler.EventHandler,
	predicates []predicate.Predicate,
	asPartial bool,
) {
	// Determine watch object and conversion needs
	watchObj, needsConversion, err := processObject(obj, b.scheme, asPartial)
	if err != nil {
		b.errors = append(b.errors, err)

		return
	}

	var wrappedHandler handler.EventHandler
	var wrappedPreds []predicate.Predicate

	// PERFORMANCE CRITICAL: Use combined handler when conversion needed and both handler
	// and predicates are present. This prevents double conversion where the same object
	// would be converted once in the predicate wrapper and again in the handler wrapper.
	//
	// Benchmarks show this optimization reduces allocations by 50% (~16 allocs vs ~32)
	// and improves latency by ~46% per event. See builder_conversion_bench_test.go.
	//
	// DO NOT refactor to separate predicate/handler wrapping without re-running benchmarks.
	if needsConversion && h != nil && len(predicates) > 0 {
		// Combined handler: converts once, evaluates predicates, then calls handler
		wrappedHandler = newConvertingHandler(b.scheme, h, predicates, b.name)
		// Predicates are handled internally by convertingHandler
		wrappedPreds = nil
	} else {
		// Separate wrapping for edge cases:
		// - No conversion needed (pass-through)
		// - Handler is nil (predicates only)
		// - No predicates (handler only)
		wrappedPreds = wrapPredicates(b.scheme, predicates, needsConversion, b.name)
		if h != nil {
			wrappedHandler = wrapHandler(b.scheme, h, needsConversion, b.name)
		}
	}

	// Create source using the builder's cache
	src := source.Kind(b.cache, watchObj, wrappedHandler, wrappedPreds...)

	// Register watch
	if err := b.ctrl.Watch(src); err != nil {
		b.errors = append(b.errors, fmt.Errorf("failed to watch %T: %w", obj, err))
	}
}

// deriveControllerNameFromObject derives a controller name from a client.Object.
// It extracts the type name, handles pointers, and converts to lowercase.
// Example: &v1.TestApp{} -> "testapp".
func deriveControllerNameFromObject(obj client.Object) string {
	t := reflect.TypeOf(obj)

	// Handle pointer types
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	// Get the type name (without package)
	name := t.Name()

	// Convert to lowercase
	return strings.ToLower(name)
}
