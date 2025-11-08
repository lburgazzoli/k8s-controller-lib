package conditions

import "github.com/lburgazzoli/k8s-controller-lib/pkg/util"

// AggregateOption is an interface for applying options to condition aggregation.
// This interface is designed to be extended with concrete option types as aggregation
// customization needs arise (e.g., polarity handling, custom merge strategies, etc.).
type AggregateOption = util.Option[AggregateOptions]

// AggregateOptions holds the configurable parameters for aggregating conditions.
type AggregateOptions struct {
	// DefaultReason is used when no False or Unknown condition provides a reason
	DefaultReason string
	// DefaultMessage is used when no False or Unknown condition provides a message
	DefaultMessage string

	// Future fields can be added here, such as:
	// - NegativePolarity: Handle conditions with negative polarity
	// - MergeStrategy: Custom merge strategy function
	// - FallbackStatus: Default status when contributing conditions are missing
}

// ApplyOptions applies all provided options to this AggregateOptions instance.
func (o *AggregateOptions) ApplyOptions(opts []AggregateOption) *AggregateOptions {
	for _, opt := range opts {
		opt.ApplyTo(o)
	}

	return o
}

// ApplyTo implements AggregateOption interface for AggregateOptions.
// This allows AggregateOptions to be used as an option itself.
func (o *AggregateOptions) ApplyTo(target *AggregateOptions) {
	if o.DefaultReason != "" {
		target.DefaultReason = o.DefaultReason
	}
	if o.DefaultMessage != "" {
		target.DefaultMessage = o.DefaultMessage
	}
}

// WithDefaultReason creates an AggregateOption that sets the default reason.
func WithDefaultReason(reason string) AggregateOption {
	return util.FunctionalOption[AggregateOptions](func(opts *AggregateOptions) {
		opts.DefaultReason = reason
	})
}

// WithDefaultMessage creates an AggregateOption that sets the default message.
func WithDefaultMessage(message string) AggregateOption {
	return util.FunctionalOption[AggregateOptions](func(opts *AggregateOptions) {
		opts.DefaultMessage = message
	})
}
