package conditions

// AggregateOption is an interface for applying options to condition aggregation.
// This interface is designed to be extended with concrete option types as aggregation
// customization needs arise (e.g., polarity handling, custom merge strategies, etc.).
type AggregateOption interface {
	ApplyToAggregate(opts *AggregateOptions)
}

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
		opt.ApplyToAggregate(o)
	}
	return o
}

// ApplyToAggregate implements AggregateOption interface for AggregateOptions.
// This allows AggregateOptions to be used as an option itself.
func (o *AggregateOptions) ApplyToAggregate(target *AggregateOptions) {
	if o.DefaultReason != "" {
		target.DefaultReason = o.DefaultReason
	}
	if o.DefaultMessage != "" {
		target.DefaultMessage = o.DefaultMessage
	}
}

// DefaultReason is an AggregateOption that sets the default reason.
type DefaultReason string

// ApplyToAggregate applies the DefaultReason option.
func (r DefaultReason) ApplyToAggregate(opts *AggregateOptions) {
	opts.DefaultReason = string(r)
}

// WithDefaultReason creates a DefaultReason option.
func WithDefaultReason(reason string) DefaultReason {
	return DefaultReason(reason)
}

// DefaultMessage is an AggregateOption that sets the default message.
type DefaultMessage string

// ApplyToAggregate applies the DefaultMessage option.
func (m DefaultMessage) ApplyToAggregate(opts *AggregateOptions) {
	opts.DefaultMessage = string(m)
}

// WithDefaultMessage creates a DefaultMessage option.
func WithDefaultMessage(message string) DefaultMessage {
	return DefaultMessage(message)
}
