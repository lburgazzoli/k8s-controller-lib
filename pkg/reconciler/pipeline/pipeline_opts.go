package pipeline

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/reconciler"
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
	Client client.Client

	// Pre/post apply hooks
	PreApply  []reconciler.ApplyHookFunc
	PostApply []reconciler.ApplyHookFunc
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
	opts.PreApply = append(opts.PreApply, o.PreApply...)
	opts.PostApply = append(opts.PostApply, o.PostApply...)
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

// WithPreApply adds hooks that run before objects are applied.
// Hooks receive the context, the owner object, and the objects about to be applied.
func WithPreApply(hooks ...reconciler.ApplyHookFunc) Option {
	return util.FunctionalOption[Options](func(o *Options) {
		o.PreApply = append(o.PreApply, hooks...)
	})
}

// WithPostApply adds hooks that run after objects are applied.
// Hooks receive the context, the owner object, and the applied objects.
//
// Example with watcher:
//
//	w := watch.New(watch.WithClient(client), watch.WithController(ctrl), watch.WithCache(cache))
//	p := pipeline.NewPipeline(
//	    pipeline.WithClient(client),
//	    pipeline.WithPostApply(watch.All(w)),
//	    pipeline.WithActions(myAction),
//	)
func WithPostApply(hooks ...reconciler.ApplyHookFunc) Option {
	return util.FunctionalOption[Options](func(o *Options) {
		o.PostApply = append(o.PostApply, hooks...)
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
