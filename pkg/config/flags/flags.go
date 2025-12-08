package flags

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/util/strcase"
)

// Metadata holds parsed flag information from struct tags.
type Metadata struct {
	Name        string // Flag name (e.g., "log-level")
	Description string // Flag description
	Skip        bool   // If true, don't create a flag for this field
}

const maxTagParts = 2

// ParseTag parses the flag struct tag.
// Format: flag:"name" or flag:"name,description" or flag:"-" to skip.
// Returns an error if the flag name is empty or contains invalid characters.
func ParseTag(tag string) (Metadata, error) {
	if tag == "-" {
		return Metadata{Skip: true}, nil
	}

	parts := strings.SplitN(tag, ",", maxTagParts)
	name := strings.TrimSpace(parts[0])

	if name == "" {
		return Metadata{}, errors.New("flag name cannot be empty")
	}

	// Validate flag name: only alphanumeric characters and hyphens allowed
	for _, ch := range name {
		isLower := ch >= 'a' && ch <= 'z'
		isUpper := ch >= 'A' && ch <= 'Z'
		isDigit := ch >= '0' && ch <= '9'
		isHyphen := ch == '-'

		//nolint:staticcheck // Current form is more readable
		if !(isLower || isUpper || isDigit || isHyphen) {
			return Metadata{}, fmt.Errorf("invalid flag name %q: must contain only alphanumeric characters and hyphens", name)
		}
	}

	meta := Metadata{
		Name: name,
	}

	if len(parts) > 1 {
		meta.Description = strings.TrimSpace(parts[1])
	}

	return meta, nil
}

// GetFlagName determines the flag name for a field.
// Priority: flag tag > field name converted to kebab-case.
// The separator is used to join prefix with the field name.
// Returns the flag name, a bool indicating if it should be bound, and an error if tag validation fails.
func GetFlagName(field reflect.StructField, prefix string, separator string) (string, bool, error) {
	flagTag := field.Tag.Get("flag")
	if flagTag == "" {
		// Fall back to field name converted to kebab-case
		name := strcase.ToKebabCase(field.Name)
		if prefix != "" {
			name = prefix + separator + name
		}

		return name, true, nil
	}

	meta, err := ParseTag(flagTag)
	if err != nil {
		return "", false, fmt.Errorf("invalid flag tag for field %q: %w", field.Name, err)
	}

	if meta.Skip {
		return "", false, nil
	}

	if meta.Name == "" {
		return "", false, nil
	}

	if prefix != "" {
		return prefix + separator + meta.Name, true, nil
	}

	return meta.Name, true, nil
}

// GetConfigKey gets the Viper config key for a field (from mapstructure tag or field name).
func GetConfigKey(field reflect.StructField, prefix string) string {
	tag := field.Tag.Get("mapstructure")
	if tag == "" || tag == "-" {
		// Fall back to snake_case field name
		tag = strcase.ToSnakeCase(field.Name)
	}

	if prefix != "" {
		return prefix + "." + tag
	}

	return tag
}

// GetDescription extracts the description from the flag tag.
// Returns empty string if tag is invalid (errors are handled elsewhere).
func GetDescription(field reflect.StructField) string {
	flagTag := field.Tag.Get("flag")
	if flagTag == "" || flagTag == "-" {
		return ""
	}

	meta, err := ParseTag(flagTag)
	if err != nil {
		return ""
	}

	return meta.Description
}

// FlagBindingMetadata holds all metadata needed to bind a field to a flag.
type FlagBindingMetadata struct {
	FlagName    string
	ConfigKey   string
	Description string
	ShouldBind  bool
}

// ExtractFlagMetadata extracts all flag binding metadata from a struct field.
// The separator is used to join nested struct names.
// Returns an error if flag tag validation fails.
func ExtractFlagMetadata(field reflect.StructField, prefix string, separator string) (FlagBindingMetadata, error) {
	flagName, shouldBind, err := GetFlagName(field, prefix, separator)
	if err != nil {
		return FlagBindingMetadata{}, err
	}
	if !shouldBind {
		return FlagBindingMetadata{ShouldBind: false}, nil
	}

	return FlagBindingMetadata{
		FlagName:    flagName,
		ConfigKey:   GetConfigKey(field, prefix),
		Description: GetDescription(field),
		ShouldBind:  true,
	}, nil
}

// BindStruct recursively binds all fields in a struct to flags and Viper.
// The separator parameter specifies what character to use between nested struct names.
// Returns an error if:
//   - Flag tag validation fails (invalid flag name format)
//   - Duplicate flag names are detected
//   - Flag binding to Viper fails
//   - A registered flag cannot be looked up (internal error)
func BindStruct(
	fs *pflag.FlagSet,
	v *viper.Viper,
	value reflect.Value,
	typ reflect.Type,
	prefix string,
	separator string,
) error {
	seenFlags := make(map[string]string) // flag name -> field name

	return bindStructWithTracking(fs, v, value, typ, prefix, separator, seenFlags)
}

func bindStructWithTracking(
	fs *pflag.FlagSet,
	v *viper.Viper,
	value reflect.Value,
	typ reflect.Type,
	prefix string,
	separator string,
	seenFlags map[string]string,
) error {
	for i := range typ.NumField() {
		field := typ.Field(i)
		fieldValue := value.Field(i)

		if !field.IsExported() {
			continue
		}

		if flagTag := field.Tag.Get("flag"); flagTag == "-" {
			continue
		}

		// Handle nested structs (except time.Duration which is a struct but should be treated as a value type)
		if fieldValue.Kind() == reflect.Struct && field.Type != reflect.TypeFor[time.Duration]() {
			nestedPrefix := prefix
			if nestedPrefix != "" {
				nestedPrefix += separator
			}
			nestedPrefix += strcase.ToKebabCase(field.Name)

			if err := bindStructWithTracking(fs, v, fieldValue, field.Type, nestedPrefix, separator, seenFlags); err != nil {
				return err
			}

			continue
		}

		meta, err := ExtractFlagMetadata(field, prefix, separator)
		if err != nil {
			return err
		}
		if !meta.ShouldBind {
			continue
		}

		// Check for duplicate flag names
		if existingField, exists := seenFlags[meta.FlagName]; exists {
			return fmt.Errorf("duplicate flag name %q (fields %q and %q)", meta.FlagName, existingField, field.Name)
		}
		seenFlags[meta.FlagName] = field.Name

		if bindField(fs, field, fieldValue, meta) {
			flag := fs.Lookup(meta.FlagName)
			if flag == nil {
				return fmt.Errorf("flag %q was not registered (internal error)", meta.FlagName)
			}
			if err := v.BindPFlag(meta.ConfigKey, flag); err != nil {
				return fmt.Errorf("failed to bind flag %q to config key %q: %w", meta.FlagName, meta.ConfigKey, err)
			}
		}
	}

	return nil
}

// bindField binds a single field to a flag using the type registry.
// Returns true if the field was successfully bound, false otherwise.
func bindField(
	fs *pflag.FlagSet,
	field reflect.StructField,
	fieldValue reflect.Value,
	meta FlagBindingMetadata,
) bool {
	// Try to get a binder from the registry
	binder, ok := DefaultRegistry.Get(field.Type)
	if !ok {
		// Unsupported type, skip binding
		return false
	}

	// Call the binder
	err := binder(fs, meta.FlagName, fieldValue.Interface(), meta.Description)

	return err == nil
}
