package pipeline

import (
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/reconciler"
	"github.com/lburgazzoli/k8s-controller-lib/pkg/reconciler/watch"
	"github.com/lburgazzoli/k8s-controller-lib/pkg/util"
)

// Option configures a Pipeline during construction.
type Option = util.Option[Options]

// AutoWatchOptions holds configuration for automatic watch setup.
type AutoWatchOptions struct {
	Controller      controller.Controller
	Cache           cache.Cache
	WatchConfigs    []watch.Config
	ExternalWatches []schema.GroupVersionKind
}

// Options holds configuration for Pipeline construction.
type Options struct {
	Actions        []reconciler.ActionFunc
	CleanupActions []reconciler.CleanupFunc
	Finalizer      string
	FieldOwner     string
	AutoWatch      *AutoWatchOptions

	// Ownership controls whether OwnerReferences are set on provisioned objects
	Ownership bool

	// AnnotateNonOwnedObjects adds owner tracking annotations to objects without OwnerReferences
	AnnotateNonOwnedObjects bool

	// LabelNonOwnedObjects adds owner tracking labels to objects without OwnerReferences
	LabelNonOwnedObjects bool
}

// ApplyTo implements Option for Options.
func (o *Options) ApplyTo(opts *Options) {
	opts.Actions = append(opts.Actions, o.Actions...)
	opts.CleanupActions = append(opts.CleanupActions, o.CleanupActions...)

	if o.Finalizer != "" {
		opts.Finalizer = o.Finalizer
	}
	if o.FieldOwner != "" {
		opts.FieldOwner = o.FieldOwner
	}
	if o.AutoWatch != nil {
		opts.AutoWatch = o.AutoWatch
	}

	// Apply ownership and tracking options
	opts.Ownership = o.Ownership
	opts.AnnotateNonOwnedObjects = o.AnnotateNonOwnedObjects
	opts.LabelNonOwnedObjects = o.LabelNonOwnedObjects
}

// ApplyOptions applies all given options to this Options.
func (o *Options) ApplyOptions(opts []Option) *Options {
	for _, opt := range opts {
		opt.ApplyTo(o)
	}

	return o
}

// WithActions creates an Option that adds regular actions that execute sequentially.
func WithActions(actions ...reconciler.ActionFunc) Option {
	return util.FunctionalOption[Options](func(opts *Options) {
		opts.Actions = append(opts.Actions, actions...)
	})
}

// WithCleanupActions creates an Option that adds cleanup actions that execute in reverse order.
// Cleanup actions run even if regular actions fail.
func WithCleanupActions(actions ...reconciler.CleanupFunc) Option {
	return util.FunctionalOption[Options](func(opts *Options) {
		opts.CleanupActions = append(opts.CleanupActions, actions...)
	})
}

// WithFinalizer creates an Option that sets a custom finalizer name for the pipeline.
// If not specified and cleanup actions are present, a default finalizer is used.
func WithFinalizer(name string) Option {
	return util.FunctionalOption[Options](func(opts *Options) {
		opts.Finalizer = name
	})
}

// WithFieldOwner creates an Option that sets the field manager name for server-side apply operations.
// This is used for both spec updates (via resources.Apply) and status updates (via resources.ApplyStatus).
// If not specified, status updates will be skipped.
func WithFieldOwner(name string) Option {
	return util.FunctionalOption[Options](func(opts *Options) {
		opts.FieldOwner = name
	})
}

// WithTypedActions converts TypedActionFunc to ActionFunc and adds them.
// The generic constraint ensures actions can only be created with ManagedObject types.
func WithTypedActions[T reconciler.ManagedObject](actions ...reconciler.TypedActionFunc[T]) Option {
	converted := make([]reconciler.ActionFunc, len(actions))
	for i, action := range actions {
		converted[i] = reconciler.ToActionFunc(action)
	}

	return util.FunctionalOption[Options](func(opts *Options) {
		opts.Actions = append(opts.Actions, converted...)
	})
}

// WithTypedCleanup converts TypedCleanupFunc to CleanupFunc and adds them.
// The generic constraint ensures cleanup actions can only be created with ManagedObject types.
func WithTypedCleanup[T reconciler.ManagedObject](actions ...reconciler.TypedCleanupFunc[T]) Option {
	converted := make([]reconciler.CleanupFunc, len(actions))
	for i, action := range actions {
		converted[i] = reconciler.ToCleanupFunc(action)
	}

	return util.FunctionalOption[Options](func(opts *Options) {
		opts.CleanupActions = append(opts.CleanupActions, converted...)
	})
}

// WithAutoWatch enables automatic watch setup for provisioned objects.
// The controller and cache parameters are required to register watches.
// Optional AutoWatchOption arguments customize watch behavior for specific GVKs or mark external watches.
//
// Valid options are:
//   - watch.For(): configures watch behavior for a specific GVK
//   - watch.WithExternallyWatched(): marks GVKs as already watched
//
// Controller name for metrics is retrieved from the context via reconciler.WithControllerName().
// If not present in context, "unknown" will be used as fallback.
//
// Without configurations, all watched objects use:
// - Predicate: predicates.Default (generation || labels || annotations changed || deleted)
// - Handler: EnqueueRequestForOwner (reconciles owner via OwnerReference)
//
// Example:
//
//	pipeline.WithAutoWatch(ctrl, cache,
//	    watch.For(deploymentGVK, watch.WithPredicates(myPredicate)),
//	    watch.For(serviceGVK, watch.WithPredicates(predicates.Default())),
//	    watch.WithExternallyWatched(alreadyWatchedGVKs...),
//	)
func WithAutoWatch(
	ctrl controller.Controller,
	c cache.Cache,
	opts ...watch.AutoWatchOption,
) Option {
	awOpts := &watch.AutoWatchOptions{}
	awOpts.ApplyOptions(opts)

	return &AutoWatchOptions{
		Controller:      ctrl,
		Cache:           c,
		WatchConfigs:    awOpts.Configs,
		ExternalWatches: awOpts.ExternalWatches,
	}
}

// ApplyTo implements Option interface for AutoWatchOptions.
func (a *AutoWatchOptions) ApplyTo(opts *Options) {
	opts.AutoWatch = a
}

// deferredAutoWatch is a marker option that indicates auto-watch should be enabled
// with controller and cache injected later via ControllerAware and CacheAware interfaces.
// This is processed by TypedPipeline.initPipelineLocked().
type deferredAutoWatch struct {
	opts []watch.AutoWatchOption
}

// ApplyTo implements Option interface for deferredAutoWatch.
// This is a marker option - actual processing happens in TypedPipeline.initPipelineLocked().
func (d *deferredAutoWatch) ApplyTo(_ *Options) {
	// No-op: This is a marker option processed by TypedPipeline
}

// DeferredAutoWatch enables automatic watch setup with controller and cache injected later.
// Use this when creating a TypedPipeline for Builder.Complete().
//
// The controller and cache are automatically injected by Builder via the ControllerAware
// and CacheAware interfaces after the controller is created.
//
// Optional watch.AutoWatchOption parameters customize watch behavior for specific GVKs,
// same as WithAutoWatch.
//
// Example:
//
//	p := pipeline.NewTyped[*v1alpha1.MyApp](
//	    mgr.GetClient(),
//	    pipeline.WithActions(myAction),
//	    pipeline.DeferredAutoWatch(
//	        watch.For(deploymentGVK, watch.WithPredicates(myPredicate)),
//	    ),
//	)
//
//	b.For(&v1alpha1.MyApp{}).Complete(p)
func DeferredAutoWatch(opts ...watch.AutoWatchOption) Option {
	return &deferredAutoWatch{opts: opts}
}

// WithOwnership creates an Option that controls whether OwnerReferences are set on provisioned objects.
// When enabled (default), all objects get OwnerReference set to the reconciled object.
// When disabled, objects do not get OwnerReferences but can still have owner tracking via annotations/labels.
func WithOwnership(enabled bool) Option {
	return util.FunctionalOption[Options](func(opts *Options) {
		opts.Ownership = enabled
	})
}

// WithOwnerTracking creates an Option that enables both annotations and labels for non-owned objects.
// This is a convenience option that sets both AnnotateNonOwnedObjects and LabelNonOwnedObjects to true.
func WithOwnerTracking(enabled bool) Option {
	return util.FunctionalOption[Options](func(opts *Options) {
		opts.AnnotateNonOwnedObjects = enabled
		opts.LabelNonOwnedObjects = enabled
	})
}

// WithOwnerAnnotations creates an Option that controls whether owner tracking annotations are added.
// When enabled, objects without OwnerReferences get annotations with owner GVK and identity.
func WithOwnerAnnotations(enabled bool) Option {
	return util.FunctionalOption[Options](func(opts *Options) {
		opts.AnnotateNonOwnedObjects = enabled
	})
}

// WithOwnerLabels creates an Option that controls whether owner tracking labels are added.
// When enabled, objects without OwnerReferences get labels with owner name and namespace.
// These labels enable label-based watch mapping as an alternative to owner references.
func WithOwnerLabels(enabled bool) Option {
	return util.FunctionalOption[Options](func(opts *Options) {
		opts.LabelNonOwnedObjects = enabled
	})
}
