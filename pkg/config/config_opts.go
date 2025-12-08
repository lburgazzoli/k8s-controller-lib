package config

import (
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/spf13/pflag"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/util"
)

// LoaderOption is a type alias for configuration options following the functional options pattern.
type LoaderOption = util.Option[LoaderOptions]

// LoaderOptions configures the configuration loader behavior.
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

	// FlagSet is the pflag.FlagSet to bind configuration flags to
	// If set, flags will be automatically generated from struct fields
	// Default: nil (no flag binding)
	FlagSet *pflag.FlagSet

	// DecodeHook is the mapstructure DecodeHook for custom type conversions
	// Default: defaultDecodeHook() with support for time.Time, url.URL, net.IP, etc.
	DecodeHook mapstructure.DecodeHookFunc

	// NestedSeparator is the separator used for nested struct field names in flags
	// Example: with "-" separator, Server.Port becomes --server-port
	//          with "_" separator, Server.Port becomes --server_port
	// Default: "-" (kebab-case style)
	NestedSeparator string

	// TimeFormats is a list of time formats to try when parsing time.Time values
	// Formats are tried in order until one succeeds
	// Default: [time.RFC3339]
	TimeFormats []string
}

// ApplyOptions applies all provided options to this instance.
func (o *LoaderOptions) ApplyOptions(opts []LoaderOption) *LoaderOptions {
	for _, opt := range opts {
		opt.ApplyTo(o)
	}

	return o
}

// WithEnvPrefix sets the prefix for environment variables.
// Example: WithEnvPrefix("MYAPP") results in MYAPP_CONFIG_KEY.
func WithEnvPrefix(prefix string) LoaderOption {
	return util.FunctionalOption[LoaderOptions](func(opts *LoaderOptions) {
		opts.EnvPrefix = prefix
	})
}

// WithConfigPathEnvVar sets the environment variable name for configuration path.
// The path can point to either a single file or a directory.
// Example: WithConfigPathEnvVar("MYAPP_CONFIG").
func WithConfigPathEnvVar(envVar string) LoaderOption {
	return util.FunctionalOption[LoaderOptions](func(opts *LoaderOptions) {
		opts.ConfigPathEnvVar = envVar
	})
}

// WithAutomaticEnv enables or disables automatic environment variable binding.
// When enabled, all config keys automatically bind to environment variables with the configured prefix.
func WithAutomaticEnv(enabled bool) LoaderOption {
	return util.FunctionalOption[LoaderOptions](func(opts *LoaderOptions) {
		opts.AutomaticEnv = enabled
	})
}

// WithFlags binds configuration flags to the given FlagSet.
// Flags are automatically generated from struct fields based on field names and tags.
// Use flag:"-" tag to skip a field, or flag:"name,description" to customize.
//
// Example:
//
//	type Config struct {
//	    LogLevel string `flag:"log-level,Set log level"`
//	    Port     int    // Auto-generates --port flag
//	    Internal string `flag:"-"` // Skipped
//	}
//
//	cfg := &Config{}
//	loader := config.For(cfg, config.WithFlags(pflag.CommandLine))
//	pflag.Parse()
func WithFlags(fs *pflag.FlagSet) LoaderOption {
	return util.FunctionalOption[LoaderOptions](func(opts *LoaderOptions) {
		opts.FlagSet = fs
	})
}

// WithDecodeHook adds a custom DecodeHook for type conversions.
// Multiple hooks can be composed by calling this option multiple times.
// Custom hooks are composed with the default hook.
//
// Example:
//
//	customHook := func(from, to reflect.Type, data any) (any, error) {
//	    // Custom conversion logic
//	    return data, nil
//	}
//	loader := config.For(cfg, config.WithDecodeHook(customHook))
func WithDecodeHook(hook mapstructure.DecodeHookFunc) LoaderOption {
	return util.FunctionalOption[LoaderOptions](func(opts *LoaderOptions) {
		if opts.DecodeHook == nil {
			opts.DecodeHook = hook
		} else {
			opts.DecodeHook = mapstructure.ComposeDecodeHookFunc(
				opts.DecodeHook,
				hook,
			)
		}
	})
}

// WithNestedSeparator sets the separator used for nested struct field names in flags.
//
// Example:
//
//	type Config struct {
//	    Server struct {
//	        Port int
//	    }
//	}
//
//	// With default "-" separator:
//	config.For(cfg, config.WithFlags(fs))
//	// Creates flag: --server-port
//
//	// With "_" separator:
//	config.For(cfg, config.WithFlags(fs), config.WithNestedSeparator("_"))
//	// Creates flag: --server_port
func WithNestedSeparator(separator string) LoaderOption {
	return util.FunctionalOption[LoaderOptions](func(opts *LoaderOptions) {
		opts.NestedSeparator = separator
	})
}

// WithTimeFormats sets custom time formats to use when parsing time.Time values.
// Formats are tried in order until one succeeds.
//
// Example:
//
//	config.For(cfg, config.WithTimeFormats(
//	    time.RFC3339,
//	    "2006-01-02",           // Date only
//	    "2006-01-02 15:04:05",  // DateTime
//	))
func WithTimeFormats(formats ...string) LoaderOption {
	return util.FunctionalOption[LoaderOptions](func(opts *LoaderOptions) {
		opts.TimeFormats = formats
	})
}

func defaultLoaderOptions() *LoaderOptions {
	return &LoaderOptions{
		EnvPrefix:        "CONTROLLER",
		ConfigPathEnvVar: "CONTROLLER_CONFIGURATION_PATH",
		AutomaticEnv:     true,
		FlagSet:          nil,
		DecodeHook:       nil, // Will be set to defaultDecodeHook() in For() if nil
		NestedSeparator:  "-",
		TimeFormats:      []string{time.RFC3339},
	}
}
