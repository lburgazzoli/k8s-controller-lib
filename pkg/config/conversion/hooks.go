package conversion

import (
	"encoding"
	"fmt"
	"net"
	"net/url"
	"reflect"
	"time"

	"github.com/mitchellh/mapstructure"
)

// DefaultDecodeHook returns the default DecodeHook with support for:
// - time.Duration (via mapstructure built-in)
// - []string from comma-separated strings (via mapstructure built-in)
// - time.Time from RFC3339 strings
// - url.URL from strings
// - net.IP from strings
// - encoding.TextUnmarshaler types
// - Pointer types.
func DefaultDecodeHook() mapstructure.DecodeHookFunc {
	return DefaultDecodeHookWithTimeFormats(time.RFC3339)
}

// DefaultDecodeHookWithTimeFormats returns a DecodeHook with custom time formats.
// Formats are tried in order until one succeeds.
func DefaultDecodeHookWithTimeFormats(formats ...string) mapstructure.DecodeHookFunc {
	return mapstructure.ComposeDecodeHookFunc(
		// Put specific type hooks first, before general hooks
		StringToTimeHookWithFormats(formats...),
		StringToURLHook(),
		StringToIPHook(),
		StringToTextUnmarshalerHook(),
		AnyToPointerHook(),
		// General hooks last
		mapstructure.StringToTimeDurationHookFunc(),
		mapstructure.StringToSliceHookFunc(","),
	)
}

// StringToTimeHook converts string to time.Time using RFC3339 format.
func StringToTimeHook() mapstructure.DecodeHookFunc {
	return StringToTimeHookWithFormats(time.RFC3339)
}

// StringToTimeHookWithFormats converts string to time.Time using multiple formats.
// Formats are tried in order until one succeeds.
func StringToTimeHookWithFormats(formats ...string) mapstructure.DecodeHookFunc {
	return func(from, to reflect.Type, data any) (any, error) {
		if to != reflect.TypeFor[time.Time]() {
			return data, nil
		}
		if from.Kind() != reflect.String {
			return data, nil
		}

		str, ok := data.(string)
		if !ok {
			return data, nil
		}

		// Try each format in order
		var lastErr error
		for _, format := range formats {
			t, err := time.Parse(format, str)
			if err == nil {
				return t, nil
			}
			lastErr = err
		}

		// All formats failed
		if len(formats) == 1 {
			return nil, fmt.Errorf("failed to parse time %q using format %q: %w", str, formats[0], lastErr)
		}

		return nil, fmt.Errorf("failed to parse time %q using any of %d formats", str, len(formats))
	}
}

// StringToURLHook converts string to url.URL.
func StringToURLHook() mapstructure.DecodeHookFunc {
	return func(from, to reflect.Type, data any) (any, error) {
		if to != reflect.TypeFor[url.URL]() {
			return data, nil
		}
		if from.Kind() != reflect.String {
			return data, nil
		}

		str, ok := data.(string)
		if !ok {
			return data, nil
		}

		u, err := url.Parse(str)
		if err != nil {
			return nil, fmt.Errorf("failed to parse URL %q: %w", str, err)
		}

		return *u, nil
	}
}

// StringToIPHook converts string to net.IP.
func StringToIPHook() mapstructure.DecodeHookFunc {
	return func(from, to reflect.Type, data any) (any, error) {
		if to != reflect.TypeFor[net.IP]() {
			return data, nil
		}
		if from.Kind() != reflect.String {
			return data, nil
		}

		str, ok := data.(string)
		if !ok {
			return data, nil
		}

		ip := net.ParseIP(str)
		if ip == nil {
			return nil, fmt.Errorf("failed to parse IP address %q", str)
		}

		return ip, nil
	}
}

// StringToTextUnmarshalerHook converts string to any type implementing encoding.TextUnmarshaler.
func StringToTextUnmarshalerHook() mapstructure.DecodeHookFunc {
	textUnmarshalerType := reflect.TypeFor[encoding.TextUnmarshaler]()

	return func(from, to reflect.Type, data any) (any, error) {
		// Check if target type implements TextUnmarshaler
		if !to.Implements(textUnmarshalerType) {
			// Check if pointer to target type implements TextUnmarshaler
			ptrType := reflect.PointerTo(to)
			if !ptrType.Implements(textUnmarshalerType) {
				return data, nil
			}
		}

		if from.Kind() != reflect.String {
			return data, nil
		}

		str, ok := data.(string)
		if !ok {
			return data, nil
		}

		// Create a new instance of the target type
		newVal := reflect.New(to)
		unmarshaler, ok := newVal.Interface().(encoding.TextUnmarshaler)
		if !ok {
			return data, nil
		}

		if err := unmarshaler.UnmarshalText([]byte(str)); err != nil {
			return nil, fmt.Errorf("failed to unmarshal text %q: %w", str, err)
		}

		return newVal.Elem().Interface(), nil
	}
}

// AnyToPointerHook handles conversion to pointer types.
func AnyToPointerHook() mapstructure.DecodeHookFunc {
	return func(from, to reflect.Type, data any) (any, error) {
		if to.Kind() != reflect.Pointer {
			return data, nil
		}

		// If data is already a pointer of the correct type, return it
		if from == to {
			return data, nil
		}

		// If data is nil, return nil pointer
		if data == nil {
			return reflect.Zero(to).Interface(), nil
		}

		// Create a new pointer and set the value
		elemType := to.Elem()
		dataType := reflect.TypeOf(data)

		// If data type matches the element type, wrap it in a pointer
		if dataType == elemType {
			ptr := reflect.New(elemType)
			ptr.Elem().Set(reflect.ValueOf(data))

			return ptr.Interface(), nil
		}

		// Otherwise, let other hooks handle the conversion
		return data, nil
	}
}
