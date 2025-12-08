package config

import "github.com/lburgazzoli/k8s-controller-lib/pkg/util"

// LoaderOption is a type alias for configuration options following the functional options pattern
type LoaderOption = util.Option[LoaderOptions]

// LoaderOptions configures the configuration loader behavior
type LoaderOptions struct {
	// EnvPrefix is the prefix for environment variables
	// Default: "CONTROLLER"
	EnvPrefix string

	// ConfigPathEnvVar is the environment variable name that contains
	// the path to configuration file or directory
	// Default: "CONTROLLER_CONFIGURATION_PATH"
	ConfigPathEnvVar string

	// AutomaticEnv enables automatic environment variable binding
	// When true, all configuration keys are automatically bound to environment variables
	// Default: true
	AutomaticEnv bool
}

// ApplyOptions applies all provided options to this instance
func (o *LoaderOptions) ApplyOptions(opts []LoaderOption) *LoaderOptions {
	for _, opt := range opts {
		opt.ApplyTo(o)
	}
	return o
}

// WithEnvPrefix sets the prefix for environment variables
// Example: WithEnvPrefix("MYAPP") results in MYAPP_CONFIG_KEY
func WithEnvPrefix(prefix string) LoaderOption {
	return util.FunctionalOption[LoaderOptions](func(opts *LoaderOptions) {
		opts.EnvPrefix = prefix
	})
}

// WithConfigPathEnvVar sets the environment variable name for configuration path
// The path can point to either a single file or a directory
// Example: WithConfigPathEnvVar("MYAPP_CONFIG")
func WithConfigPathEnvVar(envVar string) LoaderOption {
	return util.FunctionalOption[LoaderOptions](func(opts *LoaderOptions) {
		opts.ConfigPathEnvVar = envVar
	})
}

// WithAutomaticEnv enables or disables automatic environment variable binding
// When enabled, all config keys automatically bind to environment variables with the configured prefix
func WithAutomaticEnv(enabled bool) LoaderOption {
	return util.FunctionalOption[LoaderOptions](func(opts *LoaderOptions) {
		opts.AutomaticEnv = enabled
	})
}

func defaultLoaderOptions() *LoaderOptions {
	return &LoaderOptions{
		EnvPrefix:        "CONTROLLER",
		ConfigPathEnvVar: "CONTROLLER_CONFIGURATION_PATH",
		AutomaticEnv:     true,
	}
}
