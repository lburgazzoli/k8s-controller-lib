package flags_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/config/flags"

	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

func TestParseTag(t *testing.T) {
	tests := []struct {
		name     string
		tag      string
		expected flags.Metadata
	}{
		{
			name: "skip tag",
			tag:  "-",
			expected: flags.Metadata{
				Skip: true,
			},
		},
		{
			name: "name only",
			tag:  "custom-name",
			expected: flags.Metadata{
				Name: "custom-name",
			},
		},
		{
			name: "name and description",
			tag:  "custom-name,This is a description",
			expected: flags.Metadata{
				Name:        "custom-name",
				Description: "This is a description",
			},
		},
		{
			name: "name and description with spaces",
			tag:  " custom-name , This is a description ",
			expected: flags.Metadata{
				Name:        "custom-name",
				Description: "This is a description",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			result, err := flags.ParseTag(tt.tag)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(result).To(MatchAllFields(Fields{
				"Name":        Equal(tt.expected.Name),
				"Description": Equal(tt.expected.Description),
				"Skip":        Equal(tt.expected.Skip),
			}))
		})
	}
}

func TestExtractFlagMetadata(t *testing.T) {
	tests := []struct {
		name     string
		field    reflect.StructField
		prefix   string
		expected flags.FlagBindingMetadata
	}{
		{
			name: "full metadata with description",
			field: reflect.StructField{
				Name: "LogLevel",
				Tag:  `flag:"log-level,Set the log level" mapstructure:"log_level"`,
			},
			prefix: "",
			expected: flags.FlagBindingMetadata{
				FlagName:    "log-level",
				ConfigKey:   "log_level",
				Description: "Set the log level",
				ShouldBind:  true,
			},
		},
		{
			name: "with prefix",
			field: reflect.StructField{
				Name: "Port",
				Tag:  `flag:"port,Server port" mapstructure:"port"`,
			},
			prefix: "server",
			expected: flags.FlagBindingMetadata{
				FlagName:    "server-port",
				ConfigKey:   "server.port",
				Description: "Server port",
				ShouldBind:  true,
			},
		},
		{
			name: "skip flag",
			field: reflect.StructField{
				Name: "Internal",
				Tag:  `flag:"-"`,
			},
			prefix: "",
			expected: flags.FlagBindingMetadata{
				ShouldBind: false,
			},
		},
		{
			name: "no flag tag, auto-generate",
			field: reflect.StructField{
				Name: "MaxRetries",
				Tag:  `mapstructure:"max_retries"`,
			},
			prefix: "",
			expected: flags.FlagBindingMetadata{
				FlagName:    "max-retries",
				ConfigKey:   "max_retries",
				Description: "",
				ShouldBind:  true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			result, err := flags.ExtractFlagMetadata(tt.field, tt.prefix, "-")
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(result).To(MatchAllFields(Fields{
				"FlagName":    Equal(tt.expected.FlagName),
				"ConfigKey":   Equal(tt.expected.ConfigKey),
				"Description": Equal(tt.expected.Description),
				"ShouldBind":  Equal(tt.expected.ShouldBind),
			}))
		})
	}
}

func TestGetFlagName(t *testing.T) {
	tests := []struct {
		name         string
		field        reflect.StructField
		prefix       string
		expectedName string
		expectedBind bool
	}{
		{
			name: "no tag, no prefix",
			field: reflect.StructField{
				Name: "LogLevel",
				Tag:  "",
			},
			prefix:       "",
			expectedName: "log-level",
			expectedBind: true,
		},
		{
			name: "no tag, with prefix",
			field: reflect.StructField{
				Name: "Port",
				Tag:  "",
			},
			prefix:       "server",
			expectedName: "server-port",
			expectedBind: true,
		},
		{
			name: "custom flag name",
			field: reflect.StructField{
				Name: "LogLevel",
				Tag:  `flag:"custom-log"`,
			},
			prefix:       "",
			expectedName: "custom-log",
			expectedBind: true,
		},
		{
			name: "custom flag name with prefix",
			field: reflect.StructField{
				Name: "Port",
				Tag:  `flag:"custom-port"`,
			},
			prefix:       "server",
			expectedName: "server-custom-port",
			expectedBind: true,
		},
		{
			name: "skip flag",
			field: reflect.StructField{
				Name: "Internal",
				Tag:  `flag:"-"`,
			},
			prefix:       "",
			expectedName: "",
			expectedBind: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			name, shouldBind, err := flags.GetFlagName(tt.field, tt.prefix, "-")
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(name).To(Equal(tt.expectedName))
			g.Expect(shouldBind).To(Equal(tt.expectedBind))
		})
	}
}

func TestGetDescription(t *testing.T) {
	tests := []struct {
		name         string
		field        reflect.StructField
		expectedDesc string
	}{
		{
			name: "no tag",
			field: reflect.StructField{
				Name: "LogLevel",
				Tag:  "",
			},
			expectedDesc: "",
		},
		{
			name: "skip tag",
			field: reflect.StructField{
				Name: "Internal",
				Tag:  `flag:"-"`,
			},
			expectedDesc: "",
		},
		{
			name: "name only",
			field: reflect.StructField{
				Name: "LogLevel",
				Tag:  `flag:"log-level"`,
			},
			expectedDesc: "",
		},
		{
			name: "with description",
			field: reflect.StructField{
				Name: "LogLevel",
				Tag:  `flag:"log-level,Set the log level"`,
			},
			expectedDesc: "Set the log level",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			desc := flags.GetDescription(tt.field)
			g.Expect(desc).To(Equal(tt.expectedDesc))
		})
	}
}

func TestNestedSeparatorInFlags(t *testing.T) {
	t.Run("uses custom separator", func(t *testing.T) {
		g := NewWithT(t)

		type innerStruct struct {
			Value string `flag:"value" mapstructure:"value"`
		}

		type testStruct struct {
			Nested innerStruct `mapstructure:"nested"`
		}

		cfg := testStruct{}
		val := reflect.ValueOf(cfg)
		fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
		v := viper.New()

		err := flags.BindStruct(fs, v, val, val.Type(), "", "_")
		g.Expect(err).ToNot(HaveOccurred())

		// Should create flag with underscore
		flag := fs.Lookup("nested_value")
		g.Expect(flag).NotTo(BeNil())
	})

	t.Run("uses hyphen by default", func(t *testing.T) {
		g := NewWithT(t)

		type innerStruct struct {
			Port int `flag:"port" mapstructure:"port"`
		}

		type testStruct struct {
			Server innerStruct `mapstructure:"server"`
		}

		cfg := testStruct{}
		val := reflect.ValueOf(cfg)
		fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
		v := viper.New()

		err := flags.BindStruct(fs, v, val, val.Type(), "", "-")
		g.Expect(err).ToNot(HaveOccurred())

		// Should create flag with hyphen
		flag := fs.Lookup("server-port")
		g.Expect(flag).NotTo(BeNil())
	})
}

func TestBindStruct(t *testing.T) {
	type nestedStruct struct {
		Host string `flag:"host,Server host"`
		Port int    `flag:"port,Server port"`
	}

	type testConfig struct {
		LogLevel string       `flag:"log-level,Log level"`
		Debug    bool         `flag:"debug,Enable debug mode"`
		Server   nestedStruct `mapstructure:"server"`
		Internal string       `flag:"-"`
	}

	t.Run("bind flat struct", func(t *testing.T) {
		g := NewWithT(t)

		cfg := &testConfig{
			LogLevel: "info",
			Debug:    false,
		}

		fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
		v := viper.New()
		val := reflect.ValueOf(cfg).Elem()

		err := flags.BindStruct(fs, v, val, val.Type(), "", "-")
		g.Expect(err).ToNot(HaveOccurred())

		g.Expect(fs.Lookup("log-level")).ToNot(BeNil())
		g.Expect(fs.Lookup("debug")).ToNot(BeNil())
		g.Expect(fs.Lookup("internal")).To(BeNil())
	})

	t.Run("bind nested struct", func(t *testing.T) {
		g := NewWithT(t)

		cfg := &testConfig{
			Server: nestedStruct{
				Host: "localhost",
				Port: 8080,
			},
		}

		fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
		v := viper.New()
		val := reflect.ValueOf(cfg).Elem()

		err := flags.BindStruct(fs, v, val, val.Type(), "", "-")
		g.Expect(err).ToNot(HaveOccurred())

		// Nested fields should be flattened with prefix
		g.Expect(fs.Lookup("server-host")).ToNot(BeNil())
		g.Expect(fs.Lookup("server-port")).ToNot(BeNil())
	})

	t.Run("bind with prefix", func(t *testing.T) {
		g := NewWithT(t)

		cfg := &testConfig{
			LogLevel: "info",
		}

		fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
		v := viper.New()
		val := reflect.ValueOf(cfg).Elem()

		err := flags.BindStruct(fs, v, val, val.Type(), "app", "-")
		g.Expect(err).ToNot(HaveOccurred())

		g.Expect(fs.Lookup("app-log-level")).ToNot(BeNil())
		g.Expect(fs.Lookup("app-server-host")).ToNot(BeNil())
	})
}

func TestBindStructWithAllTypes(t *testing.T) {
	type allTypes struct {
		StringVal   string        `flag:"string,String value"`
		BoolVal     bool          `flag:"bool,Bool value"`
		IntVal      int           `flag:"int,Int value"`
		Int32Val    int32         `flag:"int32,Int32 value"`
		Int64Val    int64         `flag:"int64,Int64 value"`
		UintVal     uint          `flag:"uint,Uint value"`
		Uint32Val   uint32        `flag:"uint32,Uint32 value"`
		Uint64Val   uint64        `flag:"uint64,Uint64 value"`
		Float32Val  float32       `flag:"float32,Float32 value"`
		Float64Val  float64       `flag:"float64,Float64 value"`
		DurationVal time.Duration `flag:"duration,Duration value"`
		SliceVal    []string      `flag:"slice,Slice value"`
	}

	t.Run("all types bindable", func(t *testing.T) {
		g := NewWithT(t)

		cfg := &allTypes{}
		fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
		v := viper.New()
		val := reflect.ValueOf(cfg).Elem()

		err := flags.BindStruct(fs, v, val, val.Type(), "", "-")
		g.Expect(err).ToNot(HaveOccurred())

		// Verify all flags were created
		g.Expect(fs.Lookup("string")).ToNot(BeNil())
		g.Expect(fs.Lookup("bool")).ToNot(BeNil())
		g.Expect(fs.Lookup("int")).ToNot(BeNil())
		g.Expect(fs.Lookup("int32")).ToNot(BeNil())
		g.Expect(fs.Lookup("int64")).ToNot(BeNil())
		g.Expect(fs.Lookup("uint")).ToNot(BeNil())
		g.Expect(fs.Lookup("uint32")).ToNot(BeNil())
		g.Expect(fs.Lookup("uint64")).ToNot(BeNil())
		g.Expect(fs.Lookup("float32")).ToNot(BeNil())
		g.Expect(fs.Lookup("float64")).ToNot(BeNil())
		g.Expect(fs.Lookup("duration")).ToNot(BeNil())
		g.Expect(fs.Lookup("slice")).ToNot(BeNil())
	})
}

func TestParseTagErrors(t *testing.T) {
	tests := []struct {
		name        string
		tag         string
		expectedErr string
	}{
		{
			name:        "empty flag name",
			tag:         ",Description only",
			expectedErr: "flag name cannot be empty",
		},
		{
			name:        "invalid character at sign",
			tag:         "invalid@name",
			expectedErr: "invalid flag name",
		},
		{
			name:        "invalid character space",
			tag:         "invalid name",
			expectedErr: "invalid flag name",
		},
		{
			name:        "invalid character underscore",
			tag:         "invalid_name",
			expectedErr: "invalid flag name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			result, err := flags.ParseTag(tt.tag)
			g.Expect(err).To(HaveOccurred())
			g.Expect(err.Error()).To(ContainSubstring(tt.expectedErr))
			g.Expect(result).To(Equal(flags.Metadata{}))
		})
	}
}
