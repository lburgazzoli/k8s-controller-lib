package pipeline

import (
	"github.com/lburgazzoli/k8s-controller-lib/pkg/reconciler"
)

// Option configures a Pipeline during construction.
type Option interface {
	ApplyToPipeline(opts *Options)
}

// Options holds configuration for Pipeline construction.
type Options struct {
	Actions        []reconciler.ActionFunc
	CleanupActions []reconciler.CleanupFunc
	Finalizer      string
	FieldOwner     string
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
