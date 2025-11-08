package conditions

import "github.com/lburgazzoli/k8s-controller-lib/pkg/util"

// ConditionOption is an interface for applying options when setting conditions.
type ConditionOption = util.Option[ConditionOptions]

// ConditionOptions holds the configurable parameters for a condition.
type ConditionOptions struct {
	Reason             string
	Message            string
	ObservedGeneration int64
}

// ApplyOptions applies all provided options to this ConditionOptions instance.
func (o *ConditionOptions) ApplyOptions(opts []ConditionOption) *ConditionOptions {
	for _, opt := range opts {
		opt.ApplyTo(o)
	}

	return o
}

// ApplyTo implements ConditionOption interface for ConditionOptions.
// This allows ConditionOptions to be used as an option itself.
func (o *ConditionOptions) ApplyTo(target *ConditionOptions) {
	if o.Reason != "" {
		target.Reason = o.Reason
	}
	if o.Message != "" {
		target.Message = o.Message
	}
	if o.ObservedGeneration != 0 {
		target.ObservedGeneration = o.ObservedGeneration
	}
}

// WithReason creates a ConditionOption that sets the reason field.
func WithReason(reason string) ConditionOption {
	return util.FunctionalOption[ConditionOptions](func(opts *ConditionOptions) {
		opts.Reason = reason
	})
}

// WithMessage creates a ConditionOption that sets the message field.
func WithMessage(message string) ConditionOption {
	return util.FunctionalOption[ConditionOptions](func(opts *ConditionOptions) {
		opts.Message = message
	})
}

// WithObservedGeneration creates a ConditionOption that sets the observed generation field.
func WithObservedGeneration(generation int64) ConditionOption {
	return util.FunctionalOption[ConditionOptions](func(opts *ConditionOptions) {
		opts.ObservedGeneration = generation
	})
}
