package watch

import (
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/util"
)

// Option configures a Watcher during construction.
type Option = util.Option[Options]

// Options holds configuration for Watcher construction.
type Options struct {
	Configs []Config
}

// ApplyTo implements Option for Options.
func (o *Options) ApplyTo(opts *Options) {
	opts.Configs = append(opts.Configs, o.Configs...)
}

// ApplyOptions applies all given options to this Options.
func (o *Options) ApplyOptions(opts []Option) *Options {
	for _, opt := range opts {
		opt.ApplyTo(o)
	}

	return o
}

// WithConfigs creates an Option that adds watch configurations for specific GVKs.
// Use this to pre-configure watch behavior for specific GVKs.
func WithConfigs(configs ...Config) Option {
	return util.FunctionalOption[Options](func(opts *Options) {
		opts.Configs = append(opts.Configs, configs...)
	})
}

// ConfigOption configures a Config during construction.
type ConfigOption = util.Option[Config]

// ConfigOptions holds configuration for Config construction.
type ConfigOptions struct {
	Predicates []predicate.Predicate
	Handler    handler.EventHandler
	Disabled   bool
	Partial    bool
}

// ApplyTo implements ConfigOption for ConfigOptions.
func (o *ConfigOptions) ApplyTo(cfg *Config) {
	if len(o.Predicates) > 0 {
		cfg.Predicates = append(cfg.Predicates, o.Predicates...)
	}
	if o.Handler != nil {
		cfg.Handler = o.Handler
	}
	if o.Disabled {
		cfg.Disabled = o.Disabled
	}
	if o.Partial {
		cfg.Partial = o.Partial
	}
}

// ApplyOptions applies all given options to this ConfigOptions.
func (o *ConfigOptions) ApplyOptions(opts []ConfigOption) *ConfigOptions {
	for _, opt := range opts {
		opt.ApplyTo(&Config{})
	}

	return o
}

// Config configures how a specific GroupVersionKind should be watched.
// Empty Predicates or Handler fields use sensible defaults.
// If Disabled is true, the GVK will not be watched.
// If Partial is true, PartialObjectMetadata will be used for watching (metadata only).
type Config struct {
	GVK        schema.GroupVersionKind
	Predicates []predicate.Predicate
	Handler    handler.EventHandler
	Disabled   bool
	Partial    bool
}

// ApplyTo implements ConfigOption for Config.
func (c *Config) ApplyTo(target *Config) {
	if len(c.Predicates) > 0 {
		target.Predicates = append(target.Predicates, c.Predicates...)
	}
	if c.Handler != nil {
		target.Handler = c.Handler
	}
	if c.Disabled {
		target.Disabled = c.Disabled
	}
	if c.Partial {
		target.Partial = c.Partial
	}
}

// WithPredicates creates a ConfigOption that sets predicates for a watch.
// Multiple predicates will be passed to source.Kind which handles their combination.
func WithPredicates(predicates ...predicate.Predicate) ConfigOption {
	return util.FunctionalOption[Config](func(cfg *Config) {
		cfg.Predicates = append(cfg.Predicates, predicates...)
	})
}

// WithHandler creates a ConfigOption that sets the event handler for a watch.
func WithHandler(h handler.EventHandler) ConfigOption {
	return util.FunctionalOption[Config](func(cfg *Config) {
		cfg.Handler = h
	})
}

// Disabled creates a ConfigOption that prevents watching for a GVK.
func Disabled() ConfigOption {
	return util.FunctionalOption[Config](func(cfg *Config) {
		cfg.Disabled = true
	})
}

// Partial creates a ConfigOption that enables watching with PartialObjectMetadata.
//
// When enabled, only object metadata is watched (no spec/status), which is more efficient
// for use cases that only need metadata like labels, annotations, and ownership information.
//
// Important: When using Partial(), default predicates are NOT automatically applied.
// You must explicitly provide predicates using WithPredicates() if you need filtering.
//
// Example:
//
//	watch.For(gvk, watch.Partial(), watch.WithPredicates(predicates.LabelChanged()))
func Partial() ConfigOption {
	return util.FunctionalOption[Config](func(cfg *Config) {
		cfg.Partial = true
	})
}

// For creates a Config for watching the specified GroupVersionKind with optional customizations.
// At least one ConfigOption is required; additional options can be passed to compose behavior.
//
// Supports both functional and struct-based configuration styles:
//
//	// Functional options style - single predicate
//	watch.For(gvk, watch.WithPredicates(predicate.GenerationChangedPredicate{}))
//
//	// Functional options style - multiple predicates
//	watch.For(gvk, watch.WithPredicates(pred1, pred2, pred3))
//
//	// Combined functional options
//	watch.For(gvk, watch.WithHandler(myHandler), watch.WithPredicates(myPredicate))
//
//	// Struct-based style
//	watch.For(gvk, &watch.ConfigOptions{
//	    Predicates: []predicate.Predicate{pred1, pred2},
//	    Handler: myHandler,
//	})
//
//	// Disable watching for a specific GVK
//	watch.For(gvk, watch.Disabled())
//
//	// Watch using PartialObjectMetadata (metadata only, more efficient)
//	watch.For(gvk, watch.Partial(), watch.WithPredicates(predicates.LabelChanged()))
//
// Multiple predicates are passed directly to source.Kind which handles their combination.
// The Watcher will apply sensible defaults for any unspecified predicates or handler.
// Note: When using Partial(), default predicates are NOT applied automatically.
func For(gvk schema.GroupVersionKind, opt ConfigOption, opts ...ConfigOption) Config {
	cfg := Config{GVK: gvk}

	opt.ApplyTo(&cfg)

	for i := range opts {
		opts[i].ApplyTo(&cfg)
	}

	return cfg
}
