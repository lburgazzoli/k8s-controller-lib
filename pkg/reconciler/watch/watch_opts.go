package watch

import (
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Option configures a Watcher during construction.
type Option interface {
	ApplyToWatcher(opts *Options)
}

// Options holds configuration for Watcher construction.
type Options struct {
	Configs []Config
}

// ApplyToWatcher implements Option for Options.
func (o *Options) ApplyToWatcher(opts *Options) {
	opts.Configs = append(opts.Configs, o.Configs...)
}

// ApplyOptions applies all given options to this Options.
func (o *Options) ApplyOptions(opts []Option) *Options {
	for _, opt := range opts {
		opt.ApplyToWatcher(o)
	}
	return o
}

// Configs is an Option that adds watch configurations for specific GVKs.
type Configs struct {
	configs []Config
}

// ApplyToWatcher implements Option.
func (c Configs) ApplyToWatcher(opts *Options) {
	opts.Configs = append(opts.Configs, c.configs...)
}

// WithConfigs creates a Configs option.
// Use this to pre-configure watch behavior for specific GVKs.
func WithConfigs(configs ...Config) Configs {
	return Configs{configs: configs}
}

// ConfigOption configures a Config during construction.
type ConfigOption interface {
	ApplyToConfig(cfg *Config)
}

// ConfigOptions holds configuration for Config construction.
type ConfigOptions struct {
	Predicates []predicate.Predicate
	Handler    handler.EventHandler
	Disabled   bool
}

// ApplyToConfig implements ConfigOption for ConfigOptions.
func (o *ConfigOptions) ApplyToConfig(cfg *Config) {
	if len(o.Predicates) > 0 {
		cfg.Predicates = append(cfg.Predicates, o.Predicates...)
	}
	if o.Handler != nil {
		cfg.Handler = o.Handler
	}
	if o.Disabled {
		cfg.Disabled = o.Disabled
	}
}

// ApplyOptions applies all given options to this ConfigOptions.
func (o *ConfigOptions) ApplyOptions(opts []ConfigOption) *ConfigOptions {
	for _, opt := range opts {
		opt.ApplyToConfig((*Config)(nil))
	}
	return o
}

// Config configures how a specific GroupVersionKind should be watched.
// Empty Predicates or Handler fields use sensible defaults.
// If Disabled is true, the GVK will not be watched.
type Config struct {
	GVK        schema.GroupVersionKind
	Predicates []predicate.Predicate
	Handler    handler.EventHandler
	Disabled   bool
}

// PredicatesOption is a ConfigOption that sets predicates for a watch.
type PredicatesOption struct {
	predicates []predicate.Predicate
}

// ApplyToConfig implements ConfigOption.
func (p PredicatesOption) ApplyToConfig(cfg *Config) {
	cfg.Predicates = append(cfg.Predicates, p.predicates...)
}

// WithPredicates creates a PredicatesOption.
// Multiple predicates will be passed to source.Kind which handles their combination.
func WithPredicates(predicates ...predicate.Predicate) PredicatesOption {
	return PredicatesOption{predicates: predicates}
}

// HandlerOption is a ConfigOption that sets the event handler for a watch.
type HandlerOption struct {
	handler handler.EventHandler
}

// ApplyToConfig implements ConfigOption.
func (h HandlerOption) ApplyToConfig(cfg *Config) {
	cfg.Handler = h.handler
}

// WithHandler creates a HandlerOption.
func WithHandler(h handler.EventHandler) HandlerOption {
	return HandlerOption{handler: h}
}

// DisabledOption is a ConfigOption that disables watching for a GVK.
type DisabledOption struct{}

// ApplyToConfig implements ConfigOption.
func (d DisabledOption) ApplyToConfig(cfg *Config) {
	cfg.Disabled = true
}

// Disabled creates a DisabledOption that prevents watching for a GVK.
func Disabled() DisabledOption {
	return DisabledOption{}
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
// Multiple predicates are passed directly to source.Kind which handles their combination.
// The Watcher will apply sensible defaults for any unspecified predicates or handler.
func For(gvk schema.GroupVersionKind, opt ConfigOption, opts ...ConfigOption) Config {
	cfg := Config{GVK: gvk}

	opt.ApplyToConfig(&cfg)

	for i := range opts {
		opts[i].ApplyToConfig(&cfg)
	}
	return cfg
}
