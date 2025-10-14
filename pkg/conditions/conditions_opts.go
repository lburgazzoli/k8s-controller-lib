package conditions

// ConditionOption is an interface for applying options when setting conditions.
type ConditionOption interface {
	ApplyToCondition(opts *ConditionOptions)
}

// ConditionOptions holds the configurable parameters for a condition.
type ConditionOptions struct {
	Reason             string
	Message            string
	ObservedGeneration int64
}

// ApplyOptions applies all provided options to this ConditionOptions instance.
func (o *ConditionOptions) ApplyOptions(opts []ConditionOption) *ConditionOptions {
	for _, opt := range opts {
		opt.ApplyToCondition(o)
	}
	return o
}

// ApplyToCondition implements ConditionOption interface for ConditionOptions.
// This allows ConditionOptions to be used as an option itself.
func (o *ConditionOptions) ApplyToCondition(target *ConditionOptions) {
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

// Reason is a ConditionOption that sets the reason field.
type Reason string

// ApplyToCondition applies the Reason option.
func (r Reason) ApplyToCondition(opts *ConditionOptions) {
	opts.Reason = string(r)
}

// WithReason creates a Reason option.
func WithReason(reason string) Reason {
	return Reason(reason)
}

// Message is a ConditionOption that sets the message field.
type Message string

// ApplyToCondition applies the Message option.
func (m Message) ApplyToCondition(opts *ConditionOptions) {
	opts.Message = string(m)
}

// WithMessage creates a Message option.
func WithMessage(message string) Message {
	return Message(message)
}

// ObservedGeneration is a ConditionOption that sets the observed generation field.
type ObservedGeneration int64

// ApplyToCondition applies the ObservedGeneration option.
func (g ObservedGeneration) ApplyToCondition(opts *ConditionOptions) {
	opts.ObservedGeneration = int64(g)
}

// WithObservedGeneration creates an ObservedGeneration option.
func WithObservedGeneration(generation int64) ObservedGeneration {
	return ObservedGeneration(generation)
}
