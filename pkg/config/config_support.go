package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// setDefaults extracts default values from the struct and sets them in Viper.
func setDefaults(v *viper.Viper, cfg any) {
	cfgValue := reflect.ValueOf(cfg).Elem()
	cfgType := cfgValue.Type()

	setDefaultsRecursive(v, cfgValue, cfgType, "")
}

// setDefaultsRecursive recursively sets defaults for all fields in a struct.
func setDefaultsRecursive(v *viper.Viper, value reflect.Value, typ reflect.Type, prefix string) {
	for i := range typ.NumField() {
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
			// Check if it's a value type that shouldn't be recursed
			if isValueType(field.Type) {
				if !fieldValue.IsZero() {
					v.SetDefault(key, fieldValue.Interface())
				}

				continue
			}

			setDefaultsRecursive(v, fieldValue, field.Type, key)

			continue
		}

		// Set default value if field is not zero
		if !fieldValue.IsZero() {
			v.SetDefault(key, fieldValue.Interface())
		}
	}
}

// loadFromPath loads configuration from the given path.
// Automatically detects if path is a file or directory.
func loadFromPath(v *viper.Viper, configPath string) error {
	info, err := os.Stat(configPath)
	if err != nil {
		return fmt.Errorf("failed to stat config path %s: %w", configPath, err)
	}

	if info.IsDir() {
		return loadFromDirectory(v, configPath)
	}

	return loadFromFile(v, configPath)
}

// loadFromFile loads configuration from a single file.
func loadFromFile(v *viper.Viper, filePath string) error {
	v.SetConfigFile(filePath)

	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read config file %s: %w", filePath, err)
	}

	return nil
}

// loadFromDirectory loads all configuration files from a directory.
// Each file is merged into the configuration, with later files taking precedence.
func loadFromDirectory(v *viper.Viper, dirPath string) error {
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
			if err := loadSimpleFile(v, filePath); err != nil {
				// Skip files we can't parse rather than failing

				continue
			}
			loaded = true

			continue
		}

		// Merge the loaded config into our main Viper instance
		if err := v.MergeConfigMap(fileViper.AllSettings()); err != nil {
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
// and the file content is the value.
func loadSimpleFile(v *viper.Viper, filePath string) error {
	content, err := os.ReadFile(filePath) // #nosec G304 -- filePath is controlled by configPathEnvVar
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
	v.Set(key, value)

	return nil
}

// isValueType checks if a type should be treated as a value (not recursed into).
// These are struct types that have special handling and shouldn't be recursed.
func isValueType(typ reflect.Type) bool {
	// List of types that should be treated as values
	valueTypes := []reflect.Type{
		reflect.TypeFor[time.Time](),
		reflect.TypeFor[time.Duration](),
		reflect.TypeFor[url.URL](),
		reflect.TypeFor[net.IP](),
	}

	//nolint:modernize // Simple loop is more readable than slices.Contains
	for i := range valueTypes {
		if typ == valueTypes[i] {
			return true
		}
	}

	return false
}

// bindEnvVars binds all struct fields to environment variables.
//
// This is a workaround for a Viper limitation:
//
// Viper's AutomaticEnv() makes environment variables available through Get() but NOT
// through Unmarshal(). This is because:
//   - AutomaticEnv() only affects the Get() method
//   - Unmarshal() uses AllSettings() internally
//   - AllSettings() only returns explicitly set or bound values
//   - Environment variables accessed via AutomaticEnv() don't appear in AllSettings()
//
// Example of the problem:
//
//	v := viper.New()
//	v.SetEnvPrefix("APP")
//	v.AutomaticEnv()
//	os.Setenv("APP_DATABASE_HOST", "localhost")
//
//	v.Get("database_host")     // ✅ Returns "localhost"
//	v.AllSettings()            // ❌ Returns empty map
//	v.Unmarshal(&cfg)          // ❌ cfg.DatabaseHost is empty
//
// Solution:
//
// We must explicitly call v.BindEnv(key) for every config key. This makes the
// environment variable appear in AllSettings() and thus work with Unmarshal().
//
//	v.BindEnv("database_host")
//	v.AllSettings()            // ✅ Returns map[database_host:localhost]
//	v.Unmarshal(&cfg)          // ✅ cfg.DatabaseHost is "localhost"
//
// This function walks the entire config struct and calls BindEnv() for each field.
// While not elegant, this is the only way to make environment variables work with
// struct unmarshaling in Viper.
//
// Performance note: This uses reflection to traverse the struct. For large structs
// with many fields, this could be noticeable. However, this only happens once during
// initialization, so the cost is acceptable.
func bindEnvVars(v *viper.Viper, cfg any) {
	cfgValue := reflect.ValueOf(cfg).Elem()
	cfgType := cfgValue.Type()

	bindEnvVarsRecursive(v, cfgValue, cfgType, "")
}

// bindEnvVarsRecursive recursively binds all fields to environment variables.
// It walks the struct tree and calls v.BindEnv() for each field, including nested structs.
func bindEnvVarsRecursive(v *viper.Viper, value reflect.Value, typ reflect.Type, prefix string) {
	for i := range typ.NumField() {
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
			// Check if it's a value type that shouldn't be recursed
			if isValueType(field.Type) {
				_ = v.BindEnv(key)

				continue
			}

			bindEnvVarsRecursive(v, fieldValue, field.Type, key)

			continue
		}

		// Bind this field to its environment variable
		_ = v.BindEnv(key)
	}
}
