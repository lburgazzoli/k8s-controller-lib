package config_test

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/pflag"

	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/config"
)

// Test configuration structures

type testConfig struct {
	StringValue   string        `mapstructure:"string_value"`
	IntValue      int           `mapstructure:"int_value"`
	BoolValue     bool          `mapstructure:"bool_value"`
	DurationValue time.Duration `mapstructure:"duration_value"`
	NestedConfig  nestedConfig  `mapstructure:"nested_config"`
}

type nestedConfig struct {
	NestedString string `mapstructure:"nested_string"`
	NestedInt    int    `mapstructure:"nested_int"`
}

func defaultTestConfig() *testConfig {
	return &testConfig{
		StringValue:   "default-string",
		IntValue:      42,
		BoolValue:     true,
		DurationValue: 5 * time.Second,
		NestedConfig: nestedConfig{
			NestedString: "default-nested",
			NestedInt:    100,
		},
	}
}

// Test data constants

const testYAMLConfig = `
string_value: file-string
int_value: 99
bool_value: false
duration_value: 10s
nested_config:
  nested_string: file-nested
  nested_int: 200
`

const testJSONConfig = `{
  "string_value": "json-string",
  "int_value": 77,
  "bool_value": false,
  "nested_config": {
    "nested_string": "json-nested",
    "nested_int": 300
  }
}`

const testConfig1YAML = `
string_value: dir-string
int_value: 55
`

const testConfig2YAML = `
bool_value: false
nested_config:
  nested_string: dir-nested
`

const testMixedYAML = `
nested_config:
  nested_string: yaml-nested
  nested_int: 500
`

const testEnvOverrideYAML = `
string_value: file-string
int_value: 50
`

const testCustomPrefixYAML = `
string_value: custom-path-string
`

const testPrecedenceYAML = `
int_value: 10
`

const testFlagOverrideYAML = `
log_level: info
port: 8080
`

func TestForWithDefaults(t *testing.T) {
	g := NewWithT(t)

	cfg := defaultTestConfig()
	loader, err := config.For(cfg)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(err).ToNot(HaveOccurred())

	err = loader.Load()
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(cfg).To(PointTo(MatchAllFields(Fields{
		"StringValue":   Equal("default-string"),
		"IntValue":      Equal(42),
		"BoolValue":     BeTrue(),
		"DurationValue": Equal(5 * time.Second),
		"NestedConfig": MatchAllFields(Fields{
			"NestedString": Equal("default-nested"),
			"NestedInt":    Equal(100),
		}),
	})))
}

func TestForWithEnvPrefix(t *testing.T) {
	g := NewWithT(t)

	t.Setenv("MYAPP_STRING_VALUE", "myapp-string")
	t.Setenv("MYAPP_INT_VALUE", "123")

	cfg := defaultTestConfig()
	loader, err := config.For(cfg, config.WithEnvPrefix("MYAPP"))
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(err).ToNot(HaveOccurred())

	err = loader.Load()
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(cfg).To(PointTo(MatchFields(IgnoreExtras, Fields{
		"StringValue": Equal("myapp-string"),
		"IntValue":    Equal(123),
	})))
}

func TestForWithConfigPathEnvVar(t *testing.T) {
	g := NewWithT(t)

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	err := os.WriteFile(configFile, []byte(testCustomPrefixYAML), 0600)
	g.Expect(err).ToNot(HaveOccurred())

	t.Setenv("CUSTOM_CONFIG_PATH", configFile)

	cfg := defaultTestConfig()
	loader, err := config.For(cfg, config.WithConfigPathEnvVar("CUSTOM_CONFIG_PATH"))
	g.Expect(err).ToNot(HaveOccurred())

	err = loader.Load()
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(cfg).To(PointTo(MatchFields(IgnoreExtras, Fields{
		"StringValue": Equal("custom-path-string"),
	})))
}

func TestForWithAutomaticEnv(t *testing.T) {
	g := NewWithT(t)

	t.Setenv("CONTROLLER_STRING_VALUE", "env-value")

	cfg := defaultTestConfig()
	loader, err := config.For(cfg, config.WithAutomaticEnv(false))
	g.Expect(err).ToNot(HaveOccurred())

	err = loader.Load()
	g.Expect(err).ToNot(HaveOccurred())

	// With AutomaticEnv disabled, env var should not override
	g.Expect(cfg.StringValue).To(Equal("default-string"))
}

func TestLoadFromYAMLFile(t *testing.T) {
	g := NewWithT(t)

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	err := os.WriteFile(configFile, []byte(testYAMLConfig), 0600)
	g.Expect(err).ToNot(HaveOccurred())

	t.Setenv("CONTROLLER_CONFIGURATION_PATH", configFile)

	cfg := defaultTestConfig()
	loader, err := config.For(cfg)
	g.Expect(err).ToNot(HaveOccurred())

	err = loader.Load()
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(cfg).To(PointTo(MatchAllFields(Fields{
		"StringValue":   Equal("file-string"),
		"IntValue":      Equal(99),
		"BoolValue":     BeFalse(),
		"DurationValue": Equal(10 * time.Second),
		"NestedConfig": MatchAllFields(Fields{
			"NestedString": Equal("file-nested"),
			"NestedInt":    Equal(200),
		}),
	})))
}

func TestLoadFromJSONFile(t *testing.T) {
	g := NewWithT(t)

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.json")

	err := os.WriteFile(configFile, []byte(testJSONConfig), 0600)
	g.Expect(err).ToNot(HaveOccurred())

	t.Setenv("CONTROLLER_CONFIGURATION_PATH", configFile)

	cfg := defaultTestConfig()
	loader, err := config.For(cfg)
	g.Expect(err).ToNot(HaveOccurred())

	err = loader.Load()
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(cfg).To(PointTo(MatchFields(IgnoreExtras, Fields{
		"StringValue": Equal("json-string"),
		"IntValue":    Equal(77),
		"BoolValue":   BeFalse(),
		"NestedConfig": MatchAllFields(Fields{
			"NestedString": Equal("json-nested"),
			"NestedInt":    Equal(300),
		}),
	})))
}

func TestLoadFromDirectory(t *testing.T) {
	g := NewWithT(t)

	tmpDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tmpDir, "config1.yaml"), []byte(testConfig1YAML), 0600)
	g.Expect(err).ToNot(HaveOccurred())

	err = os.WriteFile(filepath.Join(tmpDir, "config2.yaml"), []byte(testConfig2YAML), 0600)
	g.Expect(err).ToNot(HaveOccurred())

	t.Setenv("CONTROLLER_CONFIGURATION_PATH", tmpDir)

	cfg := defaultTestConfig()
	loader, err := config.For(cfg)
	g.Expect(err).ToNot(HaveOccurred())

	err = loader.Load()
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(cfg).To(PointTo(MatchFields(IgnoreExtras, Fields{
		"StringValue": Equal("dir-string"),
		"IntValue":    Equal(55),
		"BoolValue":   BeFalse(),
		"NestedConfig": MatchFields(IgnoreExtras, Fields{
			"NestedString": Equal("dir-nested"),
			"NestedInt":    Equal(100),
		}),
	})))
}

func TestLoadSimpleKeyValueFiles(t *testing.T) {
	g := NewWithT(t)

	tmpDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tmpDir, "string_value"), []byte("simple-string"), 0600)
	g.Expect(err).ToNot(HaveOccurred())

	err = os.WriteFile(filepath.Join(tmpDir, "int_value"), []byte("88"), 0600)
	g.Expect(err).ToNot(HaveOccurred())

	err = os.WriteFile(filepath.Join(tmpDir, "bool_value"), []byte("false"), 0600)
	g.Expect(err).ToNot(HaveOccurred())

	t.Setenv("CONTROLLER_CONFIGURATION_PATH", tmpDir)

	cfg := defaultTestConfig()
	loader, err := config.For(cfg)
	g.Expect(err).ToNot(HaveOccurred())

	err = loader.Load()
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(cfg).To(PointTo(MatchFields(IgnoreExtras, Fields{
		"StringValue": Equal("simple-string"),
		"IntValue":    Equal(88),
		"BoolValue":   BeFalse(),
	})))
}

func TestLoadMixedStructuredAndSimpleFiles(t *testing.T) {
	g := NewWithT(t)

	tmpDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tmpDir, "config.yaml"), []byte(testMixedYAML), 0600)
	g.Expect(err).ToNot(HaveOccurred())

	err = os.WriteFile(filepath.Join(tmpDir, "string_value"), []byte("simple-string"), 0600)
	g.Expect(err).ToNot(HaveOccurred())

	err = os.WriteFile(filepath.Join(tmpDir, "int_value"), []byte("66"), 0600)
	g.Expect(err).ToNot(HaveOccurred())

	t.Setenv("CONTROLLER_CONFIGURATION_PATH", tmpDir)

	cfg := defaultTestConfig()
	loader, err := config.For(cfg)
	g.Expect(err).ToNot(HaveOccurred())

	err = loader.Load()
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(cfg).To(PointTo(MatchFields(IgnoreExtras, Fields{
		"StringValue": Equal("simple-string"),
		"IntValue":    Equal(66),
		"NestedConfig": MatchAllFields(Fields{
			"NestedString": Equal("yaml-nested"),
			"NestedInt":    Equal(500),
		}),
	})))
}

func TestLoadWithEnvironmentVariables(t *testing.T) {
	g := NewWithT(t)

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	err := os.WriteFile(configFile, []byte(testEnvOverrideYAML), 0600)
	g.Expect(err).ToNot(HaveOccurred())

	t.Setenv("CONTROLLER_CONFIGURATION_PATH", configFile)
	t.Setenv("CONTROLLER_STRING_VALUE", "env-string")
	t.Setenv("CONTROLLER_INT_VALUE", "999")

	cfg := defaultTestConfig()
	loader, err := config.For(cfg)
	g.Expect(err).ToNot(HaveOccurred())

	err = loader.Load()
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(cfg).To(PointTo(MatchFields(IgnoreExtras, Fields{
		"StringValue": Equal("env-string"),
		"IntValue":    Equal(999),
	})))
}

func TestLoadWithNestedEnvironmentVariables(t *testing.T) {
	g := NewWithT(t)

	t.Setenv("CONTROLLER_NESTED_CONFIG_NESTED_STRING", "env-nested-string")
	t.Setenv("CONTROLLER_NESTED_CONFIG_NESTED_INT", "777")

	cfg := defaultTestConfig()
	loader, err := config.For(cfg)
	g.Expect(err).ToNot(HaveOccurred())

	err = loader.Load()
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(cfg).To(PointTo(MatchFields(IgnoreExtras, Fields{
		"NestedConfig": MatchAllFields(Fields{
			"NestedString": Equal("env-nested-string"),
			"NestedInt":    Equal(777),
		}),
	})))
}

func TestLoadPrecedenceOrder(t *testing.T) {
	g := NewWithT(t)

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	err := os.WriteFile(configFile, []byte(testPrecedenceYAML), 0600)
	g.Expect(err).ToNot(HaveOccurred())

	t.Setenv("CONTROLLER_CONFIGURATION_PATH", configFile)
	t.Setenv("CONTROLLER_INT_VALUE", "20")

	cfg := defaultTestConfig()
	loader, err := config.For(cfg)
	g.Expect(err).ToNot(HaveOccurred())

	err = loader.Load()
	g.Expect(err).ToNot(HaveOccurred())

	// Env var (20) should win over file (10) and default (42)
	g.Expect(cfg.IntValue).To(Equal(20))
}

func TestLoadFromNonExistentPath(t *testing.T) {
	g := NewWithT(t)

	t.Setenv("CONTROLLER_CONFIGURATION_PATH", "/nonexistent/path/config.yaml")

	cfg := defaultTestConfig()
	loader, err := config.For(cfg)
	g.Expect(err).ToNot(HaveOccurred())

	err = loader.Load()
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("failed to stat config path"))
}

func TestLoadFromEmptyDirectory(t *testing.T) {
	g := NewWithT(t)

	tmpDir := t.TempDir()
	t.Setenv("CONTROLLER_CONFIGURATION_PATH", tmpDir)

	cfg := defaultTestConfig()
	loader, err := config.For(cfg)
	g.Expect(err).ToNot(HaveOccurred())

	err = loader.Load()
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("no configuration files found"))
}

func TestLoadSkipsHiddenFiles(t *testing.T) {
	g := NewWithT(t)

	tmpDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tmpDir, ".hidden"), []byte("should-not-load"), 0600)
	g.Expect(err).ToNot(HaveOccurred())

	err = os.WriteFile(filepath.Join(tmpDir, "string_value"), []byte("visible-file"), 0600)
	g.Expect(err).ToNot(HaveOccurred())

	t.Setenv("CONTROLLER_CONFIGURATION_PATH", tmpDir)

	cfg := defaultTestConfig()
	loader, err := config.For(cfg)
	g.Expect(err).ToNot(HaveOccurred())

	err = loader.Load()
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(cfg.StringValue).To(Equal("visible-file"))
}

func TestLoadSkipsDirectoriesInDirectory(t *testing.T) {
	g := NewWithT(t)

	tmpDir := t.TempDir()

	subDir := filepath.Join(tmpDir, "subdir")
	err := os.Mkdir(subDir, 0700)
	g.Expect(err).ToNot(HaveOccurred())

	err = os.WriteFile(filepath.Join(subDir, "config.yaml"), []byte("string_value: sub"), 0600)
	g.Expect(err).ToNot(HaveOccurred())

	err = os.WriteFile(filepath.Join(tmpDir, "string_value"), []byte("main"), 0600)
	g.Expect(err).ToNot(HaveOccurred())

	t.Setenv("CONTROLLER_CONFIGURATION_PATH", tmpDir)

	cfg := defaultTestConfig()
	loader, err := config.For(cfg)
	g.Expect(err).ToNot(HaveOccurred())

	err = loader.Load()
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(cfg.StringValue).To(Equal("main"))
}

func TestForWithFlags(t *testing.T) {
	type flagConfig struct {
		LogLevel string `flag:"log-level,Set log level" mapstructure:"log_level"`
		Port     int    `mapstructure:"port"`
		Debug    bool   `flag:"-"                       mapstructure:"debug"`
	}

	t.Run("auto-generate flags from struct", func(t *testing.T) {
		g := NewWithT(t)

		cfg := &flagConfig{
			LogLevel: "info",
			Port:     8080,
			Debug:    false,
		}

		fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
		loader, err := config.For(cfg, config.WithFlags(fs))
		g.Expect(err).ToNot(HaveOccurred())

		// Flags should be bound automatically
		g.Expect(fs.Lookup("log-level")).ToNot(BeNil())
		g.Expect(fs.Lookup("port")).ToNot(BeNil())
		g.Expect(fs.Lookup("debug")).To(BeNil()) // Skipped with flag:"-"

		_ = loader
	})

	t.Run("flags override config values", func(t *testing.T) {
		g := NewWithT(t)

		tmpDir := t.TempDir()
		configFile := filepath.Join(tmpDir, "config.yaml")

		err := os.WriteFile(configFile, []byte(testFlagOverrideYAML), 0600)
		g.Expect(err).ToNot(HaveOccurred())

		t.Setenv("CONTROLLER_CONFIGURATION_PATH", configFile)

		cfg := &flagConfig{}
		fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
		loader, err := config.For(cfg, config.WithFlags(fs))
		g.Expect(err).ToNot(HaveOccurred())

		err = fs.Parse([]string{"--log-level=debug", "--port=9090"})
		g.Expect(err).ToNot(HaveOccurred())

		err = loader.Load()
		g.Expect(err).ToNot(HaveOccurred())

		g.Expect(cfg).To(PointTo(MatchAllFields(Fields{
			"LogLevel": Equal("debug"),
			"Port":     Equal(9090),
			"Debug":    BeFalse(),
		})))
	})
}

func TestForWithFlagsAndNestedStruct(t *testing.T) {
	type serverConfig struct {
		Host string `flag:"host,Server host address" mapstructure:"host"`
		Port int    `flag:"port,Server port"         mapstructure:"port"`
	}

	type appConfig struct {
		LogLevel string       `flag:"log-level,Log level" mapstructure:"log_level"`
		Server   serverConfig `mapstructure:"server"`
	}

	t.Run("nested struct creates prefixed flags", func(t *testing.T) {
		g := NewWithT(t)

		cfg := &appConfig{
			LogLevel: "info",
			Server: serverConfig{
				Host: "localhost",
				Port: 8080,
			},
		}

		fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
		loader, err := config.For(cfg, config.WithFlags(fs))
		g.Expect(err).ToNot(HaveOccurred())

		// Top-level flag
		g.Expect(fs.Lookup("log-level")).ToNot(BeNil())

		// Nested flags with prefix
		g.Expect(fs.Lookup("server-host")).ToNot(BeNil())
		g.Expect(fs.Lookup("server-port")).ToNot(BeNil())

		_ = loader
	})

	t.Run("parse nested flags", func(t *testing.T) {
		g := NewWithT(t)

		cfg := &appConfig{}
		fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
		loader, err := config.For(cfg, config.WithFlags(fs))
		g.Expect(err).ToNot(HaveOccurred())

		err = fs.Parse([]string{
			"--log-level=debug",
			"--server-host=example.com",
			"--server-port=9090",
		})
		g.Expect(err).ToNot(HaveOccurred())

		err = loader.Load()
		g.Expect(err).ToNot(HaveOccurred())

		g.Expect(cfg).To(PointTo(MatchAllFields(Fields{
			"LogLevel": Equal("debug"),
			"Server": MatchAllFields(Fields{
				"Host": Equal("example.com"),
				"Port": Equal(9090),
			}),
		})))
	})
}

func TestFullPrecedenceWithFlags(t *testing.T) {
	type testCfg struct {
		Value int `mapstructure:"value"`
	}

	t.Run("precedence: flags > env > file > default", func(t *testing.T) {
		g := NewWithT(t)

		tmpDir := t.TempDir()
		configFile := filepath.Join(tmpDir, "config.yaml")

		err := os.WriteFile(configFile, []byte("value: 10"), 0600)
		g.Expect(err).ToNot(HaveOccurred())

		t.Setenv("CONTROLLER_CONFIGURATION_PATH", configFile)
		t.Setenv("CONTROLLER_VALUE", "20")

		cfg := &testCfg{Value: 5} // default
		fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
		loader, err := config.For(cfg, config.WithFlags(fs))
		g.Expect(err).ToNot(HaveOccurred())

		err = fs.Parse([]string{"--value=30"})
		g.Expect(err).ToNot(HaveOccurred())

		err = loader.Load()
		g.Expect(err).ToNot(HaveOccurred())

		// Flag (30) wins over env (20), file (10), and default (5)
		g.Expect(cfg.Value).To(Equal(30))
	})
}

func TestForErrorConditions(t *testing.T) {
	t.Run("nil config returns error", func(t *testing.T) {
		g := NewWithT(t)

		loader, err := config.For(nil)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("config cannot be nil"))
		g.Expect(loader).To(BeNil())
	})

	t.Run("non-pointer config returns error", func(t *testing.T) {
		g := NewWithT(t)

		cfg := testConfig{}
		loader, err := config.For(cfg)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("config must be a pointer"))
		g.Expect(loader).To(BeNil())
	})

	t.Run("pointer to non-struct returns error", func(t *testing.T) {
		g := NewWithT(t)

		var str string
		loader, err := config.For(&str)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("config must be a pointer to struct"))
		g.Expect(loader).To(BeNil())
	})

	t.Run("duplicate flag names return error", func(t *testing.T) {
		g := NewWithT(t)

		type duplicateConfig struct {
			Field1 string `flag:"my-flag,First field"`
			Field2 string `flag:"my-flag,Second field"`
		}

		cfg := &duplicateConfig{}
		fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
		loader, err := config.For(cfg, config.WithFlags(fs))
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("duplicate flag name"))
		g.Expect(err.Error()).To(ContainSubstring("my-flag"))
		g.Expect(loader).To(BeNil())
	})

	t.Run("invalid flag tag format returns error", func(t *testing.T) {
		g := NewWithT(t)

		type invalidTagConfig struct {
			Field1 string `flag:"invalid@name,Description"`
		}

		cfg := &invalidTagConfig{}
		fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
		loader, err := config.For(cfg, config.WithFlags(fs))
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("invalid flag"))
		g.Expect(loader).To(BeNil())
	})

	t.Run("empty flag name returns error", func(t *testing.T) {
		g := NewWithT(t)

		type emptyFlagConfig struct {
			Field1 string `flag:",Description only"`
		}

		cfg := &emptyFlagConfig{}
		fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
		loader, err := config.For(cfg, config.WithFlags(fs))
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("flag name cannot be empty"))
		g.Expect(loader).To(BeNil())
	})
}

// Test types for validation.
type validatedPortConfig struct {
	Port int `mapstructure:"port"`
}

func (c *validatedPortConfig) Validate() error {
	if c.Port < 1024 || c.Port > 65535 {
		return fmt.Errorf("port must be between 1024 and 65535, got %d", c.Port)
	}

	return nil
}

type validatedMultiFieldConfig struct {
	Host    string `mapstructure:"host"`
	Port    int    `mapstructure:"port"`
	Timeout int    `mapstructure:"timeout"`
}

func (c *validatedMultiFieldConfig) Validate() error {
	if c.Host == "" {
		return errors.New("host is required")
	}
	if c.Port < 1024 || c.Port > 65535 {
		return fmt.Errorf("port must be between 1024 and 65535, got %d", c.Port)
	}
	if c.Timeout < 0 {
		return fmt.Errorf("timeout must be non-negative, got %d", c.Timeout)
	}

	return nil
}

type nonValidatedConfig struct {
	Port int `mapstructure:"port"`
}

func TestValidation(t *testing.T) {
	t.Run("validates config after load - fails on invalid value", func(t *testing.T) {
		g := NewWithT(t)

		t.Setenv("CONTROLLER_PORT", "99999")

		cfg := &validatedPortConfig{Port: 8080}
		loader, err := config.For(cfg)
		g.Expect(err).ToNot(HaveOccurred())

		err = loader.Load()
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("validation failed"))
		g.Expect(err.Error()).To(ContainSubstring("99999"))
	})

	t.Run("passes validation with valid config", func(t *testing.T) {
		g := NewWithT(t)

		t.Setenv("CONTROLLER_PORT", "8080")

		cfg := &validatedPortConfig{Port: 3000}
		loader, err := config.For(cfg)
		g.Expect(err).ToNot(HaveOccurred())

		err = loader.Load()
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(cfg.Port).To(Equal(8080))
	})

	t.Run("validates multiple fields", func(t *testing.T) {
		g := NewWithT(t)

		t.Setenv("CONTROLLER_HOST", "")
		t.Setenv("CONTROLLER_PORT", "8080")
		t.Setenv("CONTROLLER_TIMEOUT", "-5")

		cfg := &validatedMultiFieldConfig{
			Host:    "localhost",
			Port:    3000,
			Timeout: 30,
		}

		loader, err := config.For(cfg)
		g.Expect(err).ToNot(HaveOccurred())

		err = loader.Load()
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(SatisfyAny(
			ContainSubstring("host is required"),
			ContainSubstring("timeout must be non-negative"),
		))
	})

	t.Run("skips validation if not implemented", func(t *testing.T) {
		g := NewWithT(t)

		t.Setenv("CONTROLLER_PORT", "99999") // Invalid but won't be checked

		cfg := &nonValidatedConfig{Port: 8080}
		loader, err := config.For(cfg)
		g.Expect(err).ToNot(HaveOccurred())

		err = loader.Load()
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(cfg.Port).To(Equal(99999)) // Loads even though invalid
	})

	t.Run("validates with default values", func(t *testing.T) {
		g := NewWithT(t)

		// No env vars set, use defaults
		cfg := &validatedPortConfig{Port: 8080}
		loader, err := config.For(cfg)
		g.Expect(err).ToNot(HaveOccurred())

		err = loader.Load()
		g.Expect(err).ToNot(HaveOccurred()) // Valid default
		g.Expect(cfg.Port).To(Equal(8080))
	})

	t.Run("validates with invalid default", func(t *testing.T) {
		g := NewWithT(t)

		// Invalid default value
		cfg := &validatedPortConfig{Port: 500}
		loader, err := config.For(cfg)
		g.Expect(err).ToNot(HaveOccurred())

		err = loader.Load()
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("validation failed"))
		g.Expect(err.Error()).To(ContainSubstring("500"))
	})
}

func TestNestedSeparator(t *testing.T) {
	t.Run("uses custom separator for nested flags", func(t *testing.T) {
		g := NewWithT(t)

		type NestedConfig struct {
			Server struct {
				Port int `flag:"port" mapstructure:"port"`
			} `mapstructure:"server"`
		}

		cfg := &NestedConfig{}
		cfg.Server.Port = 8080

		fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
		_, err := config.For(
			cfg,
			config.WithFlags(fs),
			config.WithNestedSeparator("_"),
		)
		g.Expect(err).ToNot(HaveOccurred())

		// Check that flag was created with underscore separator
		flag := fs.Lookup("server_port")
		g.Expect(flag).NotTo(BeNil())

		// And not with hyphen
		flag = fs.Lookup("server-port")
		g.Expect(flag).To(BeNil())
	})

	t.Run("default separator is hyphen", func(t *testing.T) {
		g := NewWithT(t)

		type NestedConfig struct {
			Database struct {
				Host string `flag:"host" mapstructure:"host"`
			} `mapstructure:"database"`
		}

		cfg := &NestedConfig{}
		cfg.Database.Host = "localhost"

		fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
		_, err := config.For(cfg, config.WithFlags(fs))
		g.Expect(err).ToNot(HaveOccurred())

		// Default separator should be hyphen
		flag := fs.Lookup("database-host")
		g.Expect(flag).NotTo(BeNil())
	})
}

func TestTimeFormats(t *testing.T) {
	t.Run("supports custom time formats", func(t *testing.T) {
		g := NewWithT(t)

		type TimeConfig struct {
			StartDate time.Time `mapstructure:"start_date"`
		}

		cfg := &TimeConfig{}
		t.Setenv("CONTROLLER_START_DATE", "2024-01-15")

		loader, err := config.For(
			cfg,
			config.WithTimeFormats(
				"2006-01-02", // Date only
				time.RFC3339, // Full datetime
			),
		)
		g.Expect(err).ToNot(HaveOccurred())

		err = loader.Load()
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(cfg.StartDate.Year()).To(Equal(2024))
		g.Expect(cfg.StartDate.Month()).To(Equal(time.January))
		g.Expect(cfg.StartDate.Day()).To(Equal(15))
	})

	t.Run("tries formats in order", func(t *testing.T) {
		g := NewWithT(t)

		type TimeConfig struct {
			Timestamp time.Time `mapstructure:"timestamp"`
		}

		cfg := &TimeConfig{}
		t.Setenv("CONTROLLER_TIMESTAMP", "2024-01-15 10:30:00")

		loader, err := config.For(
			cfg,
			config.WithTimeFormats(
				"2006-01-02",          // Won't match
				"2006-01-02 15:04:05", // Will match
				time.RFC3339,          // Won't be tried
			),
		)
		g.Expect(err).ToNot(HaveOccurred())

		err = loader.Load()
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(cfg.Timestamp.Year()).To(Equal(2024))
		g.Expect(cfg.Timestamp.Hour()).To(Equal(10))
		g.Expect(cfg.Timestamp.Minute()).To(Equal(30))
	})

	t.Run("fails when no format matches", func(t *testing.T) {
		g := NewWithT(t)

		type TimeConfig struct {
			Timestamp time.Time `mapstructure:"timestamp"`
		}

		cfg := &TimeConfig{}
		t.Setenv("CONTROLLER_TIMESTAMP", "15/01/2024")

		loader, err := config.For(
			cfg,
			config.WithTimeFormats("2006-01-02"),
		)
		g.Expect(err).ToNot(HaveOccurred())

		err = loader.Load()
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("failed to parse time"))
	})

	t.Run("default format is RFC3339", func(t *testing.T) {
		g := NewWithT(t)

		type TimeConfig struct {
			Timestamp time.Time `mapstructure:"timestamp"`
		}

		cfg := &TimeConfig{}
		t.Setenv("CONTROLLER_TIMESTAMP", "2024-01-15T10:30:00Z")

		loader, err := config.For(cfg)
		g.Expect(err).ToNot(HaveOccurred())

		err = loader.Load()
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(cfg.Timestamp.Year()).To(Equal(2024))
	})
}

func TestAllTypesIntegration(t *testing.T) {
	t.Run("complete workflow with all supported types", func(t *testing.T) {
		g := NewWithT(t)

		type AllTypesConfig struct {
			// Basic types
			Name       string        `flag:"name,Service name" mapstructure:"name"`
			Port       int           `flag:"port,Port number" mapstructure:"port"`
			Enabled    bool          `flag:"enabled,Enable feature" mapstructure:"enabled"`
			MaxRetries int32         `flag:"max-retries,Max retries" mapstructure:"max_retries"`
			RequestID  int64         `flag:"request-id,Request ID" mapstructure:"request_id"`
			Workers    uint          `flag:"workers,Number of workers" mapstructure:"workers"`
			Batch      uint32        `flag:"batch,Batch size" mapstructure:"batch"`
			Counter    uint64        `flag:"counter,Counter value" mapstructure:"counter"`
			Ratio      float32       `flag:"ratio,Ratio value" mapstructure:"ratio"`
			Score      float64       `flag:"score,Score value" mapstructure:"score"`
			Timeout    time.Duration `flag:"timeout,Timeout duration" mapstructure:"timeout"`
			Tags       []string      `flag:"tags,Tags list" mapstructure:"tags"`

			// Advanced types
			StartTime time.Time `flag:"start-time,Start time" mapstructure:"start_time"`
			Endpoint  url.URL   `flag:"endpoint,Service endpoint" mapstructure:"endpoint"`
			ServerIP  net.IP    `flag:"server-ip,Server IP" mapstructure:"server_ip"`

			// Pointer types
			OptionalPort *int    `flag:"optional-port,Optional port" mapstructure:"optional_port"`
			OptionalName *string `flag:"optional-name,Optional name" mapstructure:"optional_name"`
		}

		// Set struct defaults
		cfg := &AllTypesConfig{
			Name:       "default-service",
			Port:       8080,
			Enabled:    true,
			MaxRetries: 3,
			RequestID:  1000,
			Workers:    4,
			Batch:      100,
			Counter:    5000,
			Ratio:      0.5,
			Score:      99.9,
			Timeout:    30 * time.Second,
			Tags:       []string{"default"},
			StartTime:  time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			Endpoint:   url.URL{Scheme: "http", Host: "localhost"},
			ServerIP:   net.ParseIP("127.0.0.1"),
		}

		// Create config directory and file
		tempDir := t.TempDir()
		configFile := filepath.Join(tempDir, "config.yaml")

		configYAML := `
name: file-service
port: 9090
timeout: 60s
start_time: "2024-06-15T10:30:00Z"
endpoint: https://file.example.com
server_ip: 10.0.0.1
optional_port: 3000
`
		err := os.WriteFile(configFile, []byte(configYAML), 0600)
		g.Expect(err).ToNot(HaveOccurred())

		// Set environment variables (override some file values)
		t.Setenv("TEST_NAME", "env-service")
		t.Setenv("TEST_ENABLED", "false")
		t.Setenv("TEST_MAX_RETRIES", "5")
		t.Setenv("TEST_OPTIONAL_NAME", "from-env")

		// Create flag set and set some flag values (highest priority)
		fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
		loader, err := config.For(
			cfg,
			config.WithEnvPrefix("TEST"),
			config.WithConfigPathEnvVar("TEST_CONFIG_PATH"),
			config.WithFlags(fs),
		)
		g.Expect(err).ToNot(HaveOccurred())

		// Set flag values
		t.Setenv("TEST_CONFIG_PATH", configFile)
		err = fs.Parse([]string{
			"--port=7070",
			"--workers=8",
			"--tags=flag1,flag2,flag3",
		})
		g.Expect(err).ToNot(HaveOccurred())

		// Load configuration
		err = loader.Load()
		g.Expect(err).ToNot(HaveOccurred())

		// Verify precedence: defaults < file < env < flags
		g.Expect(cfg.Name).To(Equal("env-service"))                                   // from env (overrides file)
		g.Expect(cfg.Port).To(Equal(7070))                                            // from flag (overrides file and env)
		g.Expect(cfg.Enabled).To(BeFalse())                                           // from env (overrides default)
		g.Expect(cfg.MaxRetries).To(Equal(int32(5)))                                  // from env
		g.Expect(cfg.RequestID).To(Equal(int64(1000)))                                // from default
		g.Expect(cfg.Workers).To(Equal(uint(8)))                                      // from flag
		g.Expect(cfg.Batch).To(Equal(uint32(100)))                                    // from default
		g.Expect(cfg.Counter).To(Equal(uint64(5000)))                                 // from default
		g.Expect(cfg.Ratio).To(Equal(float32(0.5)))                                   // from default
		g.Expect(cfg.Score).To(Equal(99.9))                                           // from default
		g.Expect(cfg.Timeout).To(Equal(60 * time.Second))                             // from file
		g.Expect(cfg.Tags).To(Equal([]string{"flag1", "flag2", "flag3"}))             // from flag
		g.Expect(cfg.StartTime.Equal(time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC))).To(BeTrue()) // from file
		g.Expect(cfg.Endpoint.String()).To(Equal("https://file.example.com"))         // from file
		g.Expect(cfg.ServerIP.String()).To(Equal("10.0.0.1"))                         // from file
		g.Expect(cfg.OptionalPort).NotTo(BeNil())                                     // from file
		g.Expect(*cfg.OptionalPort).To(Equal(3000))                                   // from file
		g.Expect(cfg.OptionalName).NotTo(BeNil())                                     // from env
		g.Expect(*cfg.OptionalName).To(Equal("from-env"))                             // from env
	})
}
