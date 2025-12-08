package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Loader manages configuration loading from multiple sources with defined precedence
type Loader struct {
	v    *viper.Viper
	opts *LoaderOptions
}

// NewLoader creates a new configuration loader with the given options
func NewLoader(opts ...LoaderOption) *Loader {
	options := defaultLoaderOptions()
	options.ApplyOptions(opts)

	v := viper.New()

	// Configure environment variable handling
	v.SetEnvPrefix(options.EnvPrefix)
	if options.AutomaticEnv {
		v.AutomaticEnv()
	}
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	return &Loader{
		v:    v,
		opts: options,
	}
}

// BindFlags binds the given flag set to the loader
// This allows command-line flags to override configuration from other sources
// Call this before flag.Parse() to ensure flags take precedence
func (l *Loader) BindFlags(flags *pflag.FlagSet) {
	if err := l.v.BindPFlags(flags); err != nil {
		// This should rarely fail, but we handle it gracefully
		// by continuing without flag binding
		return
	}
}

// Load reads configuration from all sources and unmarshals into cfg
// cfg must be a pointer to a struct with mapstructure tags
//
// Configuration is loaded in the following order (later sources override earlier ones):
// 1. Struct defaults (from field initialization)
// 2. Configuration files (from CONTROLLER_CONFIGURATION_PATH or custom env var)
// 3. Environment variables (with configured prefix)
// 4. Command-line flags (if BindFlags was called)
func (l *Loader) Load(cfg interface{}) error {
	if cfg == nil {
		return fmt.Errorf("configuration target cannot be nil")
	}

	// Ensure cfg is a pointer
	cfgValue := reflect.ValueOf(cfg)
	if cfgValue.Kind() != reflect.Ptr {
		return fmt.Errorf("configuration target must be a pointer, got %T", cfg)
	}

	// Set defaults from struct
	if err := l.setDefaults(cfg); err != nil {
		return fmt.Errorf("failed to set defaults: %w", err)
	}

	// Load configuration files if path is set
	configPath := os.Getenv(l.opts.ConfigPathEnvVar)
	if configPath != "" {
		if err := l.loadFromPath(configPath); err != nil {
			return fmt.Errorf("failed to load configuration from path: %w", err)
		}
	}

	// Unmarshal into struct
	if err := l.v.Unmarshal(cfg); err != nil {
		return fmt.Errorf("failed to unmarshal configuration: %w", err)
	}

	return nil
}

// setDefaults extracts default values from the struct and sets them in Viper
func (l *Loader) setDefaults(cfg interface{}) error {
	cfgValue := reflect.ValueOf(cfg).Elem()
	cfgType := cfgValue.Type()

	l.setDefaultsRecursive(cfgValue, cfgType, "")
	return nil
}

// setDefaultsRecursive recursively sets defaults for all fields in a struct
func (l *Loader) setDefaultsRecursive(value reflect.Value, typ reflect.Type, prefix string) {
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		fieldValue := value.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Get mapstructure tag or use field name
		tag := field.Tag.Get("mapstructure")
		if tag == "" || tag == "-" {
			continue
		}

		// Build config key
		key := tag
		if prefix != "" {
			key = prefix + "." + tag
		}

		// Handle nested structs
		if fieldValue.Kind() == reflect.Struct {
			l.setDefaultsRecursive(fieldValue, field.Type, key)
			continue
		}

		// Set default value if field is not zero
		if !fieldValue.IsZero() {
			l.v.SetDefault(key, fieldValue.Interface())
		}
	}
}

// loadFromPath loads configuration from the given path
// Automatically detects if path is a file or directory
func (l *Loader) loadFromPath(configPath string) error {
	info, err := os.Stat(configPath)
	if err != nil {
		return fmt.Errorf("failed to stat config path %s: %w", configPath, err)
	}

	if info.IsDir() {
		return l.loadFromDirectory(configPath)
	}

	return l.loadFromFile(configPath)
}

// loadFromFile loads configuration from a single file
func (l *Loader) loadFromFile(filePath string) error {
	l.v.SetConfigFile(filePath)

	if err := l.v.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read config file %s: %w", filePath, err)
	}

	return nil
}

// loadFromDirectory loads all configuration files from a directory
// Each file is merged into the configuration, with later files taking precedence
func (l *Loader) loadFromDirectory(dirPath string) error {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %w", dirPath, err)
	}

	// Track if we've loaded at least one file
	loaded := false

	for _, entry := range entries {
		// Skip directories and hidden files
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		filePath := filepath.Join(dirPath, entry.Name())

		// Try to load the file
		// We use a new Viper instance for each file to handle both structured and simple files
		fileViper := viper.New()
		fileViper.SetConfigFile(filePath)

		if err := fileViper.ReadInConfig(); err != nil {
			// If the file can't be parsed as a config file, try to read it as a simple key-value
			// where the filename (without extension) is the key and content is the value
			if err := l.loadSimpleFile(filePath); err != nil {
				// Skip files we can't parse rather than failing
				continue
			}
			loaded = true
			continue
		}

		// Merge the loaded config into our main Viper instance
		if err := l.v.MergeConfigMap(fileViper.AllSettings()); err != nil {
			return fmt.Errorf("failed to merge config from %s: %w", filePath, err)
		}

		loaded = true
	}

	if !loaded {
		return fmt.Errorf("no configuration files found in directory %s", dirPath)
	}

	return nil
}

// loadSimpleFile handles simple key-value files where the filename is the key
// and the file content is the value
func (l *Loader) loadSimpleFile(filePath string) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	// Use filename (without extension) as the key
	filename := filepath.Base(filePath)
	key := strings.TrimSuffix(filename, filepath.Ext(filename))
	
	// If there's no extension, use the full filename
	if key == "" {
		key = filename
	}

	// Set the value in Viper
	// Trim whitespace from content
	value := strings.TrimSpace(string(content))
	l.v.Set(key, value)

	return nil
}

