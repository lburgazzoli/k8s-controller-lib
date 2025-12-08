package flags

import (
	"encoding"
	"fmt"
	"net"
	"net/url"
	"reflect"
	"time"

	"github.com/spf13/pflag"
)

// FlagBinder is a function that binds a field value to a flag.
type FlagBinder func(
	fs *pflag.FlagSet,
	flagName string,
	defaultValue any,
	description string,
) error

// Registry maps Go types to flag binding functions.
type Registry struct {
	binders map[reflect.Type]FlagBinder
}

// NewRegistry creates a new empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		binders: make(map[reflect.Type]FlagBinder),
	}
}

// Register registers a FlagBinder for the given type.
func (r *Registry) Register(typ reflect.Type, binder FlagBinder) {
	r.binders[typ] = binder
}

// Get returns the FlagBinder for the given type.
// It handles direct type matches, pointer types, and TextUnmarshaler types.
func (r *Registry) Get(typ reflect.Type) (FlagBinder, bool) {
	// Direct type match
	if binder, ok := r.binders[typ]; ok {
		return binder, true
	}

	// Check if it's a pointer to a registered type
	if typ.Kind() == reflect.Pointer {
		if binder, ok := r.binders[typ.Elem()]; ok {
			return makePointerBinder(binder), true
		}
	}

	// Check if type implements encoding.TextUnmarshaler
	if implementsTextUnmarshaler(typ) {
		return textUnmarshalerBinder, true
	}

	// Check if pointer to type implements encoding.TextUnmarshaler
	if typ.Kind() != reflect.Pointer {
		ptrType := reflect.PointerTo(typ)
		if implementsTextUnmarshaler(ptrType) {
			return textUnmarshalerBinder, true
		}
	}

	return nil, false
}

// CanBind returns true if the registry can bind the given type.
func (r *Registry) CanBind(typ reflect.Type) bool {
	_, ok := r.Get(typ)

	return ok
}

// DefaultRegistry is the default registry with all basic types registered.
//
//nolint:gochecknoglobals // Global registry is intentional for convenience.
var DefaultRegistry = newDefaultRegistry()

func newDefaultRegistry() *Registry {
	r := NewRegistry()
	registerBasicTypes(r)
	registerTimeTypes(r)
	registerNetworkTypes(r)

	return r
}

// registerBasicTypes registers all basic Go types.
func registerBasicTypes(r *Registry) {
	r.Register(reflect.TypeFor[string](), stringBinder)
	r.Register(reflect.TypeFor[bool](), boolBinder)
	r.Register(reflect.TypeFor[int](), intBinder)
	r.Register(reflect.TypeFor[int32](), int32Binder)
	r.Register(reflect.TypeFor[int64](), int64Binder)
	r.Register(reflect.TypeFor[uint](), uintBinder)
	r.Register(reflect.TypeFor[uint32](), uint32Binder)
	r.Register(reflect.TypeFor[uint64](), uint64Binder)
	r.Register(reflect.TypeFor[float32](), float32Binder)
	r.Register(reflect.TypeFor[float64](), float64Binder)
	r.Register(reflect.TypeFor[time.Duration](), durationBinder)
	r.Register(reflect.TypeFor[[]string](), stringSliceBinder)
}

// registerTimeTypes registers time-related types.
func registerTimeTypes(r *Registry) {
	r.Register(reflect.TypeFor[time.Time](), timeBinder)
}

// registerNetworkTypes registers network-related types.
func registerNetworkTypes(r *Registry) {
	r.Register(reflect.TypeFor[url.URL](), urlBinder)
	r.Register(reflect.TypeFor[net.IP](), ipBinder)
}

// Binder functions for each type

func stringBinder(fs *pflag.FlagSet, flagName string, defaultValue any, description string) error {
	val, ok := defaultValue.(string)
	if !ok {
		return fmt.Errorf("expected string, got %T", defaultValue)
	}
	fs.String(flagName, val, description)

	return nil
}

func boolBinder(fs *pflag.FlagSet, flagName string, defaultValue any, description string) error {
	val, ok := defaultValue.(bool)
	if !ok {
		return fmt.Errorf("expected bool, got %T", defaultValue)
	}
	fs.Bool(flagName, val, description)

	return nil
}

func intBinder(fs *pflag.FlagSet, flagName string, defaultValue any, description string) error {
	val, ok := defaultValue.(int)
	if !ok {
		return fmt.Errorf("expected int, got %T", defaultValue)
	}
	fs.Int(flagName, val, description)

	return nil
}

func int32Binder(fs *pflag.FlagSet, flagName string, defaultValue any, description string) error {
	val, ok := defaultValue.(int32)
	if !ok {
		return fmt.Errorf("expected int32, got %T", defaultValue)
	}
	fs.Int32(flagName, val, description)

	return nil
}

func int64Binder(fs *pflag.FlagSet, flagName string, defaultValue any, description string) error {
	val, ok := defaultValue.(int64)
	if !ok {
		return fmt.Errorf("expected int64, got %T", defaultValue)
	}
	fs.Int64(flagName, val, description)

	return nil
}

func uintBinder(fs *pflag.FlagSet, flagName string, defaultValue any, description string) error {
	val, ok := defaultValue.(uint)
	if !ok {
		return fmt.Errorf("expected uint, got %T", defaultValue)
	}
	fs.Uint(flagName, val, description)

	return nil
}

func uint32Binder(fs *pflag.FlagSet, flagName string, defaultValue any, description string) error {
	val, ok := defaultValue.(uint32)
	if !ok {
		return fmt.Errorf("expected uint32, got %T", defaultValue)
	}
	fs.Uint32(flagName, val, description)

	return nil
}

func uint64Binder(fs *pflag.FlagSet, flagName string, defaultValue any, description string) error {
	val, ok := defaultValue.(uint64)
	if !ok {
		return fmt.Errorf("expected uint64, got %T", defaultValue)
	}
	fs.Uint64(flagName, val, description)

	return nil
}

func float32Binder(fs *pflag.FlagSet, flagName string, defaultValue any, description string) error {
	val, ok := defaultValue.(float32)
	if !ok {
		return fmt.Errorf("expected float32, got %T", defaultValue)
	}
	fs.Float32(flagName, val, description)

	return nil
}

func float64Binder(fs *pflag.FlagSet, flagName string, defaultValue any, description string) error {
	val, ok := defaultValue.(float64)
	if !ok {
		return fmt.Errorf("expected float64, got %T", defaultValue)
	}
	fs.Float64(flagName, val, description)

	return nil
}

func durationBinder(fs *pflag.FlagSet, flagName string, defaultValue any, description string) error {
	val, ok := defaultValue.(time.Duration)
	if !ok {
		return fmt.Errorf("expected time.Duration, got %T", defaultValue)
	}
	fs.Duration(flagName, val, description)

	return nil
}

func stringSliceBinder(fs *pflag.FlagSet, flagName string, defaultValue any, description string) error {
	val, ok := defaultValue.([]string)
	if !ok {
		return fmt.Errorf("expected []string, got %T", defaultValue)
	}
	fs.StringSlice(flagName, val, description)

	return nil
}

func timeBinder(fs *pflag.FlagSet, flagName string, defaultValue any, description string) error {
	val, ok := defaultValue.(time.Time)
	if !ok {
		return fmt.Errorf("expected time.Time, got %T", defaultValue)
	}
	// Bind as string in RFC3339 format
	fs.String(flagName, val.Format(time.RFC3339), description)

	return nil
}

func urlBinder(fs *pflag.FlagSet, flagName string, defaultValue any, description string) error {
	val, ok := defaultValue.(url.URL)
	if !ok {
		return fmt.Errorf("expected url.URL, got %T", defaultValue)
	}
	// Bind as string
	fs.String(flagName, val.String(), description)

	return nil
}

func ipBinder(fs *pflag.FlagSet, flagName string, defaultValue any, description string) error {
	val, ok := defaultValue.(net.IP)
	if !ok {
		return fmt.Errorf("expected net.IP, got %T", defaultValue)
	}
	// Bind as string
	fs.String(flagName, val.String(), description)

	return nil
}

// textUnmarshalerBinder handles types implementing encoding.TextUnmarshaler.
func textUnmarshalerBinder(fs *pflag.FlagSet, flagName string, defaultValue any, description string) error {
	// All TextUnmarshaler types are bound as strings
	// The actual unmarshaling happens in the DecodeHook
	fs.String(flagName, fmt.Sprintf("%v", defaultValue), description)

	return nil
}

// makePointerBinder wraps a binder to handle pointer types.
func makePointerBinder(innerBinder FlagBinder) FlagBinder {
	return func(fs *pflag.FlagSet, flagName string, defaultValue any, description string) error {
		// Extract value from pointer
		v := reflect.ValueOf(defaultValue)
		if v.Kind() == reflect.Pointer {
			if v.IsNil() {
				// Use zero value of the underlying type
				zero := reflect.Zero(v.Type().Elem()).Interface()

				return innerBinder(fs, flagName, zero, description)
			}
			defaultValue = v.Elem().Interface()
		}

		return innerBinder(fs, flagName, defaultValue, description)
	}
}

// implementsTextUnmarshaler checks if a type implements encoding.TextUnmarshaler.
func implementsTextUnmarshaler(typ reflect.Type) bool {
	textUnmarshalerType := reflect.TypeFor[encoding.TextUnmarshaler]()

	return typ.Implements(textUnmarshalerType)
}
