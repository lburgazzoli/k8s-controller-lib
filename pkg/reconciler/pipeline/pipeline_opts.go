package pipeline

import (
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/reconciler"
	"github.com/lburgazzoli/k8s-controller-lib/pkg/reconciler/watch"
	"github.com/lburgazzoli/k8s-controller-lib/pkg/util"
)

// Option configures a Pipeline during construction.
type Option = util.Option[Options]

// AutoWatchOptions holds configuration for automatic watch setup.
type AutoWatchOptions struct {
	Controller   controller.Controller
	Cache        cache.Cache
	WatchConfigs []watch.Config
}

// Options holds configuration for Pipeline construction.
type Options struct {
	Actions        []reconciler.ActionFunc
	CleanupActions []reconciler.CleanupFunc
	Finalizer      string
	FieldOwner     string
	AutoWatch      *AutoWatchOptions
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

// AutoWatchOption configures AutoWatchOptions.
type AutoWatchOption func(*AutoWatchOptions)

// WithAutoWatch enables automatic watch setup for provisioned objects.
// The controller and cache parameters are required to register watches.
// Optional Config parameters customize watch behavior for specific GVKs.
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
//	    watch.For(deploymentGVK, watch.WithPredicate(myPredicate)),
//	    watch.For(serviceGVK),  // uses defaults
//	)
func WithAutoWatch(
	ctrl controller.Controller,
	c cache.Cache,
	options ...any,
) Option {
	opts := &AutoWatchOptions{
		Controller: ctrl,
		Cache:      c,
	}

	for _, opt := range options {
		switch v := opt.(type) {
		case AutoWatchOption:
			v(opts)
		case watch.Config:
			opts.WatchConfigs = append(opts.WatchConfigs, v)
		}
	}

	return opts
}

// ApplyTo implements Option interface for AutoWatchOptions.
func (a *AutoWatchOptions) ApplyTo(opts *Options) {
	opts.AutoWatch = a
}
