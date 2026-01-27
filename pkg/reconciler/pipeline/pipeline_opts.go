package pipeline

import (
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/reconciler"
	"github.com/lburgazzoli/k8s-controller-lib/pkg/reconciler/watch"
	"github.com/lburgazzoli/k8s-controller-lib/pkg/util"
)

// Option configures a Pipeline during construction.
type Option = util.Option[Options]

// Options holds configuration for Pipeline construction.
type Options struct {
	// Actions to execute during reconciliation
	Actions        []reconciler.ActionFunc
	CleanupActions []reconciler.CleanupFunc
	Finalizer      string
	FieldOwner     string

	// Ownership controls whether OwnerReferences are set on provisioned objects
	Ownership bool

	// AnnotateNonOwnedObjects adds owner tracking annotations to objects without OwnerReferences
	AnnotateNonOwnedObjects bool

	// LabelNonOwnedObjects adds owner tracking labels to objects without OwnerReferences
	LabelNonOwnedObjects bool

	// Dependencies (injectable via options or aware interfaces)
	Client     client.Client
	Cache      cache.Cache
	Controller controller.Controller

	// Auto-watch configuration
	AutoWatch    bool           // enabled flag
	WatchConfigs []watch.Config // includes disabled configs for external watches
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

	// Apply ownership and tracking options
	opts.Ownership = o.Ownership
	opts.AnnotateNonOwnedObjects = o.AnnotateNonOwnedObjects
	opts.LabelNonOwnedObjects = o.LabelNonOwnedObjects

	// Apply dependencies if set
	if o.Client != nil {
		opts.Client = o.Client
	}
	if o.Cache != nil {
		opts.Cache = o.Cache
	}
	if o.Controller != nil {
		opts.Controller = o.Controller
	}

	// Apply auto-watch config
	if o.AutoWatch {
		opts.AutoWatch = true
	}
	opts.WatchConfigs = append(opts.WatchConfigs, o.WatchConfigs...)
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

// WithClient creates an Option that sets the Kubernetes client.
// The client can also be injected via the ClientAware interface.
func WithClient(c client.Client) Option {
	return util.FunctionalOption[Options](func(opts *Options) {
		opts.Client = c
	})
}

// WithCache creates an Option that sets the cache for auto-watch.
// The cache can also be injected via the CacheAware interface.
func WithCache(c cache.Cache) Option {
	return util.FunctionalOption[Options](func(opts *Options) {
		opts.Cache = c
	})
}

// WithController creates an Option that sets the controller for auto-watch.
// The controller can also be injected via the ControllerAware interface.
func WithController(ctrl controller.Controller) Option {
	return util.FunctionalOption[Options](func(opts *Options) {
		opts.Controller = ctrl
	})
}

// WithAutoWatch enables automatic watch setup for provisioned objects.
// Optional watch.AutoWatchOption arguments customize watch behavior for specific GVKs.
//
// The controller and cache must be provided either via WithController/WithCache options
// or injected via the ControllerAware/CacheAware interfaces (when used with Builder).
//
// Valid options are:
//   - watch.For(): configures watch behavior for a specific GVK
//   - watch.For(gvk, watch.Disabled()): marks a GVK as already watched (skip registration)
//
// Controller name for metrics is retrieved from the context via reconciler.WithControllerName().
// If not present in context, "unknown" will be used as fallback.
//
// Without configurations, all watched objects use:
// - Predicate: predicates.Default (generation || labels || annotations changed || deleted)
// - Handler: EnqueueRequestForOwner (reconciles owner via OwnerReference)
//
// Example with Builder (dependencies injected automatically):
//
//	p := pipeline.NewPipeline(
//	    pipeline.WithFieldOwner("my-controller"),
//	    pipeline.WithAutoWatch(
//	        watch.For(deploymentGVK, watch.WithPredicates(myPredicate)),
//	    ),
//	    pipeline.WithActions(myAction),
//	)
//	b.For(&v1alpha1.MyApp{}).Complete(p)
//
// Example standalone (dependencies provided explicitly):
//
//	p := pipeline.NewPipeline(
//	    pipeline.WithClient(client),
//	    pipeline.WithController(ctrl),
//	    pipeline.WithCache(cache),
//	    pipeline.WithAutoWatch(),
//	    pipeline.WithActions(myAction),
//	)
func WithAutoWatch(opts ...watch.AutoWatchOption) Option {
	return util.FunctionalOption[Options](func(o *Options) {
		o.AutoWatch = true

		awOpts := &watch.AutoWatchOptions{}
		awOpts.ApplyOptions(opts)
		o.WatchConfigs = append(o.WatchConfigs, awOpts.Configs...)
	})
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
