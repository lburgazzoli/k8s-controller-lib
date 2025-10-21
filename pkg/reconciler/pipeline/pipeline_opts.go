package pipeline

import (
	"github.com/lburgazzoli/k8s-controller-lib/pkg/reconciler"
	"github.com/lburgazzoli/k8s-controller-lib/pkg/reconciler/watch"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/controller"
)

// Option configures a Pipeline during construction.
type Option interface {
	ApplyToPipeline(opts *Options)
}

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

// ApplyToPipeline implements Option for Options.
func (o *Options) ApplyToPipeline(opts *Options) {
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
		opt.ApplyToPipeline(o)
	}
	return o
}

// Actions is a Option that adds regular actions that execute sequentially.
type Actions struct {
	actions []reconciler.ActionFunc
}

// ApplyToPipeline implements Option.
func (a Actions) ApplyToPipeline(opts *Options) {
	opts.Actions = append(opts.Actions, a.actions...)
}

// WithActions creates an Actions option.
func WithActions(actions ...reconciler.ActionFunc) Actions {
	return Actions{actions: actions}
}

// CleanupActions is a Option that adds cleanup actions that execute in reverse order.
// Cleanup actions run even if regular actions fail.
type CleanupActions struct {
	actions []reconciler.CleanupFunc
}

// ApplyToPipeline implements Option.
func (c CleanupActions) ApplyToPipeline(opts *Options) {
	opts.CleanupActions = append(opts.CleanupActions, c.actions...)
}

// WithCleanupActions creates a CleanupActions option.
func WithCleanupActions(actions ...reconciler.CleanupFunc) CleanupActions {
	return CleanupActions{actions: actions}
}

// Finalizer is a Option that sets a custom finalizer name for the pipeline.
// If not specified and cleanup actions are present, a default finalizer is used.
type Finalizer string

// ApplyToPipeline implements Option.
func (f Finalizer) ApplyToPipeline(opts *Options) {
	opts.Finalizer = string(f)
}

// WithFinalizer creates a Finalizer option.
func WithFinalizer(name string) Finalizer {
	return Finalizer(name)
}

// FieldOwner is a Option that sets the field manager name for server-side apply operations.
// This is used for both spec updates (via resources.Apply) and status updates (via resources.ApplyStatus).
// If not specified, status updates will be skipped.
type FieldOwner string

// ApplyToPipeline implements Option.
func (f FieldOwner) ApplyToPipeline(opts *Options) {
	opts.FieldOwner = string(f)
}

// WithFieldOwner creates a FieldOwner option.
func WithFieldOwner(name string) FieldOwner {
	return FieldOwner(name)
}

// WithTypedActions converts TypedActionFunc to ActionFunc and adds them.
// The generic constraint ensures actions can only be created with ManagedObject types.
func WithTypedActions[T reconciler.ManagedObject](actions ...reconciler.TypedActionFunc[T]) Actions {
	converted := make([]reconciler.ActionFunc, len(actions))
	for i, action := range actions {
		converted[i] = reconciler.ToActionFunc(action)
	}
	return Actions{actions: converted}
}

// WithTypedCleanup converts TypedCleanupFunc to CleanupFunc and adds them.
// The generic constraint ensures cleanup actions can only be created with ManagedObject types.
func WithTypedCleanup[T reconciler.ManagedObject](actions ...reconciler.TypedCleanupFunc[T]) CleanupActions {
	converted := make([]reconciler.CleanupFunc, len(actions))
	for i, action := range actions {
		converted[i] = reconciler.ToCleanupFunc(action)
	}
	return CleanupActions{actions: converted}
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
// - Predicate: predicates.Default (generation || labels || annotations changed)
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
	cache cache.Cache,
	options ...interface{},
) Option {
	opts := &AutoWatchOptions{
		Controller: ctrl,
		Cache:      cache,
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

// ApplyToPipeline implements Option interface for AutoWatchOptions.
func (a *AutoWatchOptions) ApplyToPipeline(opts *Options) {
	opts.AutoWatch = a
}
