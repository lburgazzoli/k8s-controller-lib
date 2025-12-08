package flags_test

import (
	"encoding"
	"net"
	"net/url"
	"reflect"
	"testing"
	"time"

	"github.com/spf13/pflag"

	. "github.com/onsi/gomega"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/config/flags"
)

func TestRegistry(t *testing.T) {
	g := NewWithT(t)

	t.Run("creates new registry", func(t *testing.T) {
		r := flags.NewRegistry()
		g.Expect(r).NotTo(BeNil())
	})

	t.Run("registers and retrieves basic types", func(t *testing.T) {
		r := flags.NewRegistry()

		binder := func(fs *pflag.FlagSet, flagName string, defaultValue any, description string) error {
			fs.String(flagName, "test", description)

			return nil
		}

		typ := reflect.TypeFor[string]()
		r.Register(typ, binder)

		retrieved, ok := r.Get(typ)
		g.Expect(ok).To(BeTrue())
		g.Expect(retrieved).NotTo(BeNil())
	})

	t.Run("returns false for unregistered type", func(t *testing.T) {
		r := flags.NewRegistry()

		typ := reflect.TypeFor[complex64]()
		_, ok := r.Get(typ)
		g.Expect(ok).To(BeFalse())
	})

	t.Run("CanBind returns true for registered type", func(t *testing.T) {
		r := flags.NewRegistry()

		binder := func(fs *pflag.FlagSet, flagName string, defaultValue any, description string) error {
			return nil
		}

		typ := reflect.TypeFor[int]()
		r.Register(typ, binder)

		g.Expect(r.CanBind(typ)).To(BeTrue())
	})

	t.Run("CanBind returns false for unregistered type", func(t *testing.T) {
		r := flags.NewRegistry()

		typ := reflect.TypeFor[complex128]()
		g.Expect(r.CanBind(typ)).To(BeFalse())
	})
}

func TestDefaultRegistry(t *testing.T) {
	testCases := []struct {
		name  string
		typ   reflect.Type
		value any
	}{
		{"string", reflect.TypeFor[string](), "test"},
		{"bool", reflect.TypeFor[bool](), true},
		{"int", reflect.TypeFor[int](), 42},
		{"int32", reflect.TypeFor[int32](), int32(42)},
		{"int64", reflect.TypeFor[int64](), int64(42)},
		{"uint", reflect.TypeFor[uint](), uint(42)},
		{"uint32", reflect.TypeFor[uint32](), uint32(42)},
		{"uint64", reflect.TypeFor[uint64](), uint64(42)},
		{"float32", reflect.TypeFor[float32](), float32(3.14)},
		{"float64", reflect.TypeFor[float64](), 3.14},
		{"time.Duration", reflect.TypeFor[time.Duration](), 5 * time.Second},
		{"[]string", reflect.TypeFor[[]string](), []string{"a", "b"}},
		{"time.Time", reflect.TypeFor[time.Time](), time.Now()},
		{"url.URL", reflect.TypeFor[url.URL](), url.URL{Scheme: "https", Host: "example.com"}},
		{"net.IP", reflect.TypeFor[net.IP](), net.ParseIP("192.168.1.1")},
	}

	for _, tc := range testCases {
		t.Run("supports "+tc.name, func(t *testing.T) {
			g := NewWithT(t)

			binder, ok := flags.DefaultRegistry.Get(tc.typ)
			g.Expect(ok).To(BeTrue(), "type %s should be registered", tc.name)
			g.Expect(binder).NotTo(BeNil())

			// Test that binder can be called
			fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
			err := binder(fs, "test-flag", tc.value, "test description")
			g.Expect(err).ToNot(HaveOccurred())

			// Verify flag was created
			flag := fs.Lookup("test-flag")
			g.Expect(flag).NotTo(BeNil())
		})
	}
}

func TestRegistryPointerSupport(t *testing.T) {
	t.Run("handles pointer to registered type", func(t *testing.T) {
		g := NewWithT(t)

		typ := reflect.TypeFor[*string]()
		binder, ok := flags.DefaultRegistry.Get(typ)
		g.Expect(ok).To(BeTrue())
		g.Expect(binder).NotTo(BeNil())

		// Test with non-nil pointer
		val := "test"
		fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
		err := binder(fs, "test-flag", &val, "test description")
		g.Expect(err).ToNot(HaveOccurred())

		flag := fs.Lookup("test-flag")
		g.Expect(flag).NotTo(BeNil())
	})

	t.Run("handles nil pointer", func(t *testing.T) {
		g := NewWithT(t)

		typ := reflect.TypeFor[*int]()
		binder, ok := flags.DefaultRegistry.Get(typ)
		g.Expect(ok).To(BeTrue())

		fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
		var nilPtr *int
		err := binder(fs, "test-flag", nilPtr, "test description")
		g.Expect(err).ToNot(HaveOccurred())

		flag := fs.Lookup("test-flag")
		g.Expect(flag).NotTo(BeNil())
	})

	t.Run("handles pointer to unregistered type", func(t *testing.T) {
		g := NewWithT(t)

		typ := reflect.TypeFor[*complex64]()
		_, ok := flags.DefaultRegistry.Get(typ)
		g.Expect(ok).To(BeFalse())
	})
}

// CustomType implements encoding.TextUnmarshaler for testing.
type CustomType struct {
	Value string
}

func (c *CustomType) UnmarshalText(text []byte) error {
	c.Value = string(text)

	return nil
}

var _ encoding.TextUnmarshaler = (*CustomType)(nil)

func TestRegistryTextUnmarshaler(t *testing.T) {
	t.Run("handles type implementing TextUnmarshaler", func(t *testing.T) {
		g := NewWithT(t)

		typ := reflect.TypeFor[CustomType]()
		binder, ok := flags.DefaultRegistry.Get(typ)
		g.Expect(ok).To(BeTrue())
		g.Expect(binder).NotTo(BeNil())

		fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
		custom := CustomType{Value: "test"}
		err := binder(fs, "custom-flag", custom, "custom description")
		g.Expect(err).ToNot(HaveOccurred())

		flag := fs.Lookup("custom-flag")
		g.Expect(flag).NotTo(BeNil())
	})

	t.Run("handles pointer to type implementing TextUnmarshaler", func(t *testing.T) {
		g := NewWithT(t)

		typ := reflect.TypeFor[*CustomType]()
		binder, ok := flags.DefaultRegistry.Get(typ)
		g.Expect(ok).To(BeTrue())

		fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
		custom := &CustomType{Value: "test"}
		err := binder(fs, "custom-flag", custom, "custom description")
		g.Expect(err).ToNot(HaveOccurred())

		flag := fs.Lookup("custom-flag")
		g.Expect(flag).NotTo(BeNil())
	})
}

func TestRegistryCustomRegistration(t *testing.T) {
	t.Run("allows custom type registration", func(t *testing.T) {
		g := NewWithT(t)

		r := flags.NewRegistry()

		// Custom binder for a custom type
		type MyType struct {
			X int
		}

		customBinder := func(fs *pflag.FlagSet, flagName string, defaultValue any, description string) error {
			fs.String(flagName, "custom", description)

			return nil
		}

		typ := reflect.TypeFor[MyType]()
		r.Register(typ, customBinder)

		binder, ok := r.Get(typ)
		g.Expect(ok).To(BeTrue())
		g.Expect(binder).NotTo(BeNil())

		fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
		err := binder(fs, "my-flag", MyType{X: 42}, "my description")
		g.Expect(err).ToNot(HaveOccurred())

		flag := fs.Lookup("my-flag")
		g.Expect(flag).NotTo(BeNil())
	})
}
