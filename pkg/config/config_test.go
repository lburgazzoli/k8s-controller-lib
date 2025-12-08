package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/config"

	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
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

func TestDefaultLoaderOptions(t *testing.T) {
	g := NewWithT(t)

	loader := config.NewLoader()
	cfg := defaultTestConfig()

	err := loader.Load(cfg)
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

func TestWithEnvPrefix(t *testing.T) {
	g := NewWithT(t)

	t.Setenv("MYAPP_STRING_VALUE", "myapp-string")
	t.Setenv("MYAPP_INT_VALUE", "123")

	loader := config.NewLoader(config.WithEnvPrefix("MYAPP"))
	cfg := defaultTestConfig()

	err := loader.Load(cfg)
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(cfg).To(PointTo(MatchFields(IgnoreExtras, Fields{
		"StringValue": Equal("myapp-string"),
		"IntValue":    Equal(123),
	})))
}

func TestWithConfigPathEnvVar(t *testing.T) {
	g := NewWithT(t)

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	err := os.WriteFile(configFile, []byte(testCustomPrefixYAML), 0600)
	g.Expect(err).ToNot(HaveOccurred())

	t.Setenv("CUSTOM_CONFIG_PATH", configFile)

	loader := config.NewLoader(config.WithConfigPathEnvVar("CUSTOM_CONFIG_PATH"))
	cfg := defaultTestConfig()

	err = loader.Load(cfg)
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(cfg).To(PointTo(MatchFields(IgnoreExtras, Fields{
		"StringValue": Equal("custom-path-string"),
	})))
}

func TestWithAutomaticEnv(t *testing.T) {
	g := NewWithT(t)

	t.Setenv("CONTROLLER_STRING_VALUE", "env-value")

	loader := config.NewLoader(config.WithAutomaticEnv(false))
	cfg := defaultTestConfig()

	err := loader.Load(cfg)
	g.Expect(err).ToNot(HaveOccurred())

	// With AutomaticEnv disabled, env var should not override
	g.Expect(cfg.StringValue).To(Equal("default-string"))
}

func TestLoadWithDefaults(t *testing.T) {
	g := NewWithT(t)

	loader := config.NewLoader()
	cfg := defaultTestConfig()

	err := loader.Load(cfg)
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

func TestLoadWithNilConfig(t *testing.T) {
	g := NewWithT(t)

	loader := config.NewLoader()
	err := loader.Load(nil)

	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("cannot be nil"))
}

func TestLoadWithNonPointerConfig(t *testing.T) {
	g := NewWithT(t)

	loader := config.NewLoader()
	cfg := testConfig{}
	err := loader.Load(cfg)

	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("must be a pointer"))
}

func TestLoadFromYAMLFile(t *testing.T) {
	g := NewWithT(t)

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	err := os.WriteFile(configFile, []byte(testYAMLConfig), 0600)
	g.Expect(err).ToNot(HaveOccurred())

	t.Setenv("CONTROLLER_CONFIGURATION_PATH", configFile)

	loader := config.NewLoader()
	cfg := defaultTestConfig()

	err = loader.Load(cfg)
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

	loader := config.NewLoader()
	cfg := defaultTestConfig()

	err = loader.Load(cfg)
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

	loader := config.NewLoader()
	cfg := defaultTestConfig()

	err = loader.Load(cfg)
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

	loader := config.NewLoader()
	cfg := defaultTestConfig()

	err = loader.Load(cfg)
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

	loader := config.NewLoader()
	cfg := defaultTestConfig()

	err = loader.Load(cfg)
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

	loader := config.NewLoader()
	cfg := defaultTestConfig()

	err = loader.Load(cfg)
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

	loader := config.NewLoader()
	cfg := defaultTestConfig()

	err := loader.Load(cfg)
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

	loader := config.NewLoader()
	cfg := defaultTestConfig()

	err = loader.Load(cfg)
	g.Expect(err).ToNot(HaveOccurred())

	// Env var (20) should win over file (10) and default (42)
	g.Expect(cfg.IntValue).To(Equal(20))
}

func TestLoadFromNonExistentPath(t *testing.T) {
	g := NewWithT(t)

	t.Setenv("CONTROLLER_CONFIGURATION_PATH", "/nonexistent/path/config.yaml")

	loader := config.NewLoader()
	cfg := defaultTestConfig()

	err := loader.Load(cfg)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("failed to stat config path"))
}

func TestLoadFromEmptyDirectory(t *testing.T) {
	g := NewWithT(t)

	tmpDir := t.TempDir()
	t.Setenv("CONTROLLER_CONFIGURATION_PATH", tmpDir)

	loader := config.NewLoader()
	cfg := defaultTestConfig()

	err := loader.Load(cfg)
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

	loader := config.NewLoader()
	cfg := defaultTestConfig()

	err = loader.Load(cfg)
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

	loader := config.NewLoader()
	cfg := defaultTestConfig()

	err = loader.Load(cfg)
	g.Expect(err).ToNot(HaveOccurred())

	// Should only load from main directory, not subdirectory
	g.Expect(cfg.StringValue).To(Equal("main"))
}
