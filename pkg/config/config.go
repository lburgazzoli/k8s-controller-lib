// Package config provides a flexible, layered configuration system for Kubernetes controllers.
//
// # Features
//
// The config package supports:
//   - Multiple configuration sources with defined precedence
//   - Automatic flag generation from struct fields
//   - Environment variable binding with custom prefixes
//   - File-based configuration (YAML, JSON, TOML, etc.)
//   - Type-safe configuration with validation
//   - Extensible type system via DecodeHooks
//
// # Supported Types
//
// The package natively supports the following types:
//
// Basic Types:
//   - string, bool
//   - int, int32, int64
//   - uint, uint32, uint64
//   - float32, float64
//   - []string
//   - time.Duration
//
// Advanced Types:
//   - time.Time (RFC3339 format)
//   - url.URL (parsed from string)
//   - net.IP (IPv4 and IPv6)
//   - encoding.TextUnmarshaler (any type implementing this interface)
//
// Pointer Types:
//   - Pointers to any supported type (*string, *int, etc.)
//   - Useful for optional configuration values
//
// # Configuration Precedence
//
// Configuration values are loaded in the following order (later sources override earlier ones):
//  1. Struct field defaults (from field initialization)
//  2. Configuration files (from CONTROLLER_CONFIGURATION_PATH or custom env var)
//  3. Environment variables (with configured prefix)
//  4. Command-line flags (if WithFlags was used)
//
// # Usage Example
//
//	type Config struct {
//	    Name      string        `flag:"name,Service name" mapstructure:"name"`
//	    Port      int           `flag:"port,Port number" mapstructure:"port"`
//	    Timeout   time.Duration `flag:"timeout,Request timeout" mapstructure:"timeout"`
//	    StartTime time.Time     `flag:"start-time,Start time" mapstructure:"start_time"`
//	    Endpoint  url.URL       `flag:"endpoint,API endpoint" mapstructure:"endpoint"`
//	    Optional  *string       `flag:"optional,Optional value" mapstructure:"optional"`
//	}
//
//	func main() {
//	    cfg := &Config{
//	        Name:    "my-service",
//	        Port:    8080,
//	        Timeout: 30 * time.Second,
//	    }
//
//	    loader, err := config.For(
//	        cfg,
//	        config.WithEnvPrefix("MYAPP"),
//	        config.WithConfigPathEnvVar("MYAPP_CONFIG_PATH"),
//	        config.WithFlags(pflag.CommandLine),
//	    )
//	    if err != nil {
//	        log.Fatal(err)
//	    }
//
//	    pflag.Parse()
//
//	    if err := loader.Load(); err != nil {
//	        log.Fatal(err)
//	    }
//
//	    // Use cfg...
//	}
//
// # Custom Types
//
// To add support for custom types, implement encoding.TextUnmarshaler:
//
//	type Priority int
//
//	func (p *Priority) UnmarshalText(text []byte) error {
//	    switch string(text) {
//	    case "low": *p = 1
//	    case "high": *p = 10
//	    default: return fmt.Errorf("invalid priority")
//	    }
//	    return nil
//	}
//
//	type Config struct {
//	    Priority string `flag:"priority" mapstructure:"priority"`
//	}
//
// Alternatively, register a custom DecodeHook:
//
//	customHook := func(from, to reflect.Type, data any) (any, error) {
//	    // Custom conversion logic
//	    return data, nil
//	}
//
//	loader, err := config.For(cfg, config.WithDecodeHook(customHook))
//
// # Limitations
//
// Thread Safety:
//   - Loader instances are not thread-safe
//   - Create separate Loader instances for concurrent use
//   - Configuration should be loaded once at startup
//
// Key Mapping:
//   - Flag names use kebab-case (--server-port)
//   - Config keys use snake_case (server_port)
//   - Environment variables use UPPER_SNAKE_CASE (MYAPP_SERVER_PORT)
//   - The library handles these conversions automatically
//
// Environment Variables (Internal):
//   - Uses a workaround for Viper's AutomaticEnv limitation
//   - Explicitly binds each struct field to enable env var support with Unmarshal
//   - This traversal happens once during For() initialization
//   - Users don't need to worry about this; it's handled automatically
//   - See bindEnvVars() in config_support.go for technical details
//
// Hot Reload:
//   - Configuration is loaded once during startup
//   - No automatic reload on file changes
//   - To support hot reload, create a new Loader and call Load() again
//
// Nested Structs:
//   - Nested structs are supported and recursed automatically
//   - Nested flags are prefixed with parent field name in kebab-case
//   - Example: Server.Port becomes --server-port flag
//
// # File Configuration
//
// The CONTROLLER_CONFIGURATION_PATH environment variable can point to:
//   - A single configuration file (YAML, JSON, TOML, etc.)
//   - A directory containing multiple configuration files
//
// When pointing to a directory:
//   - All files in the directory are loaded and merged
//   - Files are processed in lexical order
//   - Later files override values from earlier files
//   - Hidden files (starting with .) are skipped
//
// ConfigMap Integration:
//   - Each ConfigMap key becomes a file in the mounted volume
//   - Structured data (YAML/JSON) is parsed as configuration
//   - Simple key-value pairs are loaded as config keys
package config

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/spf13/viper"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/config/conversion"
	"github.com/lburgazzoli/k8s-controller-lib/pkg/config/flags"
)

// Validator is an optional interface that config structs can implement
// to provide custom validation logic. If implemented, Validate() will be
// called automatically after Load() completes successfully.
//
// Example:
//
//	type Config struct {
//	    Port int `mapstructure:"port"`
//	    Host string `mapstructure:"host"`
//	}
//
//	func (c *Config) Validate() error {
//	    if c.Port < 1024 || c.Port > 65535 {
//	        return fmt.Errorf("port must be between 1024 and 65535, got %d", c.Port)
//	    }
//	    if c.Host == "" {
//	        return errors.New("host is required")
//	    }
//	    return nil
//	}
type Validator interface {
	Validate() error
}

// Loader manages configuration loading from multiple sources with defined precedence.
type Loader struct {
	v    *viper.Viper
	opts *LoaderOptions
	cfg  any // Reference to the config struct
}

// For creates a loader for the given config struct with the specified options.
// This is the main entry point for using the config package.
//
// Returns an error if:
//   - cfg is nil
//   - cfg is not a pointer
//   - cfg is not a pointer to a struct
//   - flag binding fails
//
// Example:
//
//	cfg := &MyConfig{LogLevel: "info"}
//
//	loader, err := config.For(
//		cfg,
//	    config.WithEnvPrefix("MYAPP"),
//	    config.WithFlags(pflag.CommandLine),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	pflag.Parse()
//
//	if err := loader.Load(); err != nil {
//	    log.Fatal(err)
//	}
func For(cfg any, opts ...LoaderOption) (*Loader, error) {
	// Validate config parameter
	if cfg == nil {
		return nil, errors.New("config cannot be nil")
	}

	cfgValue := reflect.ValueOf(cfg)
	if cfgValue.Kind() != reflect.Ptr {
		return nil, fmt.Errorf("config must be a pointer, got %T", cfg)
	}

	if cfgValue.Elem().Kind() != reflect.Struct {
		return nil, fmt.Errorf("config must be a pointer to struct, got pointer to %v", cfgValue.Elem().Kind())
	}

	options := defaultLoaderOptions()
	options.ApplyOptions(opts)

	// Set default DecodeHook if not provided
	if options.DecodeHook == nil {
		if len(options.TimeFormats) > 0 {
			options.DecodeHook = conversion.DefaultDecodeHookWithTimeFormats(options.TimeFormats...)
		} else {
			options.DecodeHook = conversion.DefaultDecodeHook()
		}
	}

	v := viper.New()

	// Configure key replacers for both env vars and flags
	// This allows kebab-case flags and snake_case config keys to match
	v.SetEnvPrefix(options.EnvPrefix)
	if options.AutomaticEnv {
		v.AutomaticEnv()
	}
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))

	loader := &Loader{
		v:    v,
		opts: options,
		cfg:  cfg,
	}

	// Set defaults from struct
	setDefaults(v, cfg)

	// Bind all struct fields to environment variables.
	//
	// WORKAROUND: Viper's AutomaticEnv() makes env vars available via Get() but not
	// via Unmarshal(). We must explicitly bind each field to make env vars appear in
	// AllSettings(), which Unmarshal() uses internally.
	//
	// Without this, environment variables would not work at all with Unmarshal().
	// See bindEnvVars() documentation in config_support.go for full explanation.
	if options.AutomaticEnv {
		bindEnvVars(v, cfg)
	}

	// Bind flags if FlagSet was provided
	if options.FlagSet != nil {
		if err := flags.BindStruct(
			options.FlagSet,
			v,
			cfgValue.Elem(),
			cfgValue.Elem().Type(),
			"",
			options.NestedSeparator,
		); err != nil {
			return nil, fmt.Errorf("failed to bind flags: %w", err)
		}
	}

	return loader, nil
}

// Load reads configuration from all sources and unmarshals into the config struct.
//
// Configuration is loaded in the following order (later sources override earlier ones):
// 1. Struct defaults (from field initialization)
// 2. Configuration files (from CONTROLLER_CONFIGURATION_PATH or custom env var)
// 3. Environment variables (with configured prefix)
// 4. Command-line flags (if WithFlags was used)
//
// After loading, if the config struct implements the Validator interface,
// its Validate() method will be called automatically. This allows for
// custom validation logic to ensure configuration values are valid.
func (l *Loader) Load() error {
	// Load configuration files if path is set
	configPath := os.Getenv(l.opts.ConfigPathEnvVar)
	if configPath != "" {
		if err := loadFromPath(l.v, configPath); err != nil {
			return fmt.Errorf("failed to load configuration from path: %w", err)
		}
	}

	// Unmarshal directly using Viper with mapstructure options
	// Viper internally uses mapstructure and we can pass DecodeHook through UnmarshalKey options
	if err := l.v.Unmarshal(l.cfg, viper.DecodeHook(l.opts.DecodeHook)); err != nil {
		return fmt.Errorf("unmarshal error: %w", err)
	}

	// Validate configuration if the struct implements Validator interface
	if validator, ok := l.cfg.(Validator); ok {
		if err := validator.Validate(); err != nil {
			return fmt.Errorf("configuration validation failed: %w", err)
		}
	}

	return nil
}
