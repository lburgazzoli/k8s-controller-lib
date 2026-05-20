package watch

import (
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/util"
)

// ForOption configures watch behavior.
// This follows the util.Option[T] pattern for consistency with the rest of the codebase.
type ForOption = util.Option[ForOptions]

// ForOptions holds the accumulated configuration for watch setup.
type ForOptions struct {
	Configs []Config
}

// ApplyTo implements ForOption for ForOptions.
func (o *ForOptions) ApplyTo(opts *ForOptions) {
	opts.Configs = append(opts.Configs, o.Configs...)
}

// ApplyOptions applies all given options to this ForOptions.
func (o *ForOptions) ApplyOptions(opts []ForOption) *ForOptions {
	for _, opt := range opts {
		opt.ApplyTo(o)
	}

	return o
}

// Option configures a Watcher during construction.
type Option = util.Option[Options]

// Options holds configuration for Watcher construction.
type Options struct {
	Configs    []Config
	Client     client.Client
	Cache      cache.Cache
	Controller controller.Controller
}

// ApplyTo implements Option for Options.
func (o *Options) ApplyTo(opts *Options) {
	opts.Configs = append(opts.Configs, o.Configs...)

	if o.Client != nil {
		opts.Client = o.Client
	}
	if o.Cache != nil {
		opts.Cache = o.Cache
	}
	if o.Controller != nil {
		opts.Controller = o.Controller
	}
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
//
// To mark a GVK as already watched externally (skip watch registration),
// use a config with Disabled: true:
//
//	watch.WithConfigs(watch.NewConfig(gvk, watch.Disabled()))
func WithConfigs(configs ...Config) Option {
	return util.FunctionalOption[Options](func(opts *Options) {
		opts.Configs = append(opts.Configs, configs...)
	})
}

// WithClient creates an Option that sets the Kubernetes client on the Watcher.
func WithClient(c client.Client) Option {
	return util.FunctionalOption[Options](func(opts *Options) {
		opts.Client = c
	})
}

// WithController creates an Option that sets the controller on the Watcher.
func WithController(ctrl controller.Controller) Option {
	return util.FunctionalOption[Options](func(opts *Options) {
		opts.Controller = ctrl
	})
}

// WithCache creates an Option that sets the cache on the Watcher.
func WithCache(c cache.Cache) Option {
	return util.FunctionalOption[Options](func(opts *Options) {
		opts.Cache = c
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

// Config configures how a specific GroupVersionKind should be watched.
// Empty Predicates or Handler fields use sensible defaults.
// If Disabled is true, the GVK will not be watched (use this for externally watched GVKs).
// If Partial is true, PartialObjectMetadata will be used for watching (metadata only).
type Config struct {
	GVK        schema.GroupVersionKind
	Predicates []predicate.Predicate
	Handler    handler.EventHandler
	Disabled   bool
	Partial    bool
}

// Clone returns a deep copy of the Config.
// The returned copy is safe to modify without affecting the original.
func (c Config) Clone() Config {
	clone := Config{
		GVK:      c.GVK,
		Handler:  c.Handler,
		Disabled: c.Disabled,
		Partial:  c.Partial,
	}

	// Deep copy predicates slice
	if len(c.Predicates) > 0 {
		clone.Predicates = make([]predicate.Predicate, len(c.Predicates))
		copy(clone.Predicates, c.Predicates)
	}

	return clone
}

// ApplyTo implements ConfigOption for Config, allowing configs to be composed.
func (c Config) ApplyTo(target *Config) {
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
// Use this to mark GVKs as already watched externally (e.g., by Builder)
// to prevent redundant watch registration during watch.
//
// Example:
//
//	// Mark a GVK as externally watched
//	watch.For(gvk, watch.Disabled())
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

// For creates a ForOption for watching the specified GroupVersionKind with optional customizations.
// At least one ConfigOption is required; additional options can be passed to compose behavior.
//
// By default, watches use EnqueueRequestForOwnerOrLabel handler which:
// - First attempts to use OwnerReferences (standard Kubernetes approach)
// - Falls back to controller-lib.k8s.io labels if no owner reference is found
//
// This makes watches work seamlessly for both owned and non-owned objects.
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
//	// Mark GVK as externally watched (skip watch registration)
//	watch.For(gvk, watch.Disabled())
//
//	// Watch using PartialObjectMetadata (metadata only, more efficient)
//	watch.For(gvk, watch.Partial(), watch.WithPredicates(predicates.LabelChanged()))
//
// Multiple predicates are passed directly to source.Kind which handles their combination.
// The Watcher will apply sensible defaults for any unspecified predicates or handler.
// Note: When using Partial(), default predicates are NOT applied automatically.
func For(gvk schema.GroupVersionKind, opt ConfigOption, opts ...ConfigOption) ForOption {
	cfg := Config{GVK: gvk}

	opt.ApplyTo(&cfg)

	for i := range opts {
		opts[i].ApplyTo(&cfg)
	}

	return util.FunctionalOption[ForOptions](func(awOpts *ForOptions) {
		awOpts.Configs = append(awOpts.Configs, cfg)
	})
}

// NewConfig creates a Config directly for use with Watcher or other contexts requiring Config.
// For pipeline integration, use watch.For() which returns ForOption.
func NewConfig(gvk schema.GroupVersionKind, opt ConfigOption, opts ...ConfigOption) Config {
	cfg := Config{GVK: gvk}

	opt.ApplyTo(&cfg)

	for i := range opts {
		opts[i].ApplyTo(&cfg)
	}

	return cfg
}
