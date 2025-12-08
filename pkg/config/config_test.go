package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/gomega"
)

// Test configuration structures for testing

type testConfig struct {
	StringValue  string        `mapstructure:"string_value"`
	IntValue     int           `mapstructure:"int_value"`
	BoolValue    bool          `mapstructure:"bool_value"`
	DurationValue time.Duration `mapstructure:"duration_value"`
	NestedConfig nestedConfig  `mapstructure:"nested_config"`
}

type nestedConfig struct {
	NestedString string `mapstructure:"nested_string"`
	NestedInt    int    `mapstructure:"nested_int"`
}

func defaultTestConfig() *testConfig {
	return &testConfig{
		StringValue:  "default-string",
		IntValue:     42,
		BoolValue:    true,
		DurationValue: 5 * time.Second,
		NestedConfig: nestedConfig{
			NestedString: "default-nested",
			NestedInt:    100,
		},
	}
}

func TestDefaultLoaderOptions(t *testing.T) {
	g := NewWithT(t)

	opts := defaultLoaderOptions()

	g.Expect(opts.EnvPrefix).To(Equal("CONTROLLER"))
	g.Expect(opts.ConfigPathEnvVar).To(Equal("CONTROLLER_CONFIGURATION_PATH"))
	g.Expect(opts.AutomaticEnv).To(BeTrue())
}

func TestWithEnvPrefix(t *testing.T) {
	g := NewWithT(t)

	opts := defaultLoaderOptions()
	opt := WithEnvPrefix("MYAPP")
	opt.ApplyTo(opts)

	g.Expect(opts.EnvPrefix).To(Equal("MYAPP"))
}

func TestWithConfigPathEnvVar(t *testing.T) {
	g := NewWithT(t)

	opts := defaultLoaderOptions()
	opt := WithConfigPathEnvVar("MYAPP_CONFIG_PATH")
	opt.ApplyTo(opts)

	g.Expect(opts.ConfigPathEnvVar).To(Equal("MYAPP_CONFIG_PATH"))
}

func TestWithAutomaticEnv(t *testing.T) {
	g := NewWithT(t)

	opts := defaultLoaderOptions()
	opt := WithAutomaticEnv(false)
	opt.ApplyTo(opts)

	g.Expect(opts.AutomaticEnv).To(BeFalse())
}

func TestApplyOptions(t *testing.T) {
	g := NewWithT(t)

	opts := defaultLoaderOptions()
	opts.ApplyOptions([]LoaderOption{
		WithEnvPrefix("CUSTOM"),
		WithConfigPathEnvVar("CUSTOM_CONFIG"),
		WithAutomaticEnv(false),
	})

	g.Expect(opts.EnvPrefix).To(Equal("CUSTOM"))
	g.Expect(opts.ConfigPathEnvVar).To(Equal("CUSTOM_CONFIG"))
	g.Expect(opts.AutomaticEnv).To(BeFalse())
}

func TestNewLoader(t *testing.T) {
	g := NewWithT(t)

	loader := NewLoader()

	g.Expect(loader).ToNot(BeNil())
	g.Expect(loader.v).ToNot(BeNil())
	g.Expect(loader.opts).ToNot(BeNil())
	g.Expect(loader.opts.EnvPrefix).To(Equal("CONTROLLER"))
}

func TestNewLoaderWithOptions(t *testing.T) {
	g := NewWithT(t)

	loader := NewLoader(
		WithEnvPrefix("TESTAPP"),
		WithConfigPathEnvVar("TESTAPP_CONFIG_PATH"),
	)

	g.Expect(loader).ToNot(BeNil())
	g.Expect(loader.opts.EnvPrefix).To(Equal("TESTAPP"))
	g.Expect(loader.opts.ConfigPathEnvVar).To(Equal("TESTAPP_CONFIG_PATH"))
}

func TestLoadWithDefaults(t *testing.T) {
	g := NewWithT(t)

	loader := NewLoader()
	cfg := defaultTestConfig()

	err := loader.Load(cfg)
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(cfg.StringValue).To(Equal("default-string"))
	g.Expect(cfg.IntValue).To(Equal(42))
	g.Expect(cfg.BoolValue).To(BeTrue())
	g.Expect(cfg.DurationValue).To(Equal(5 * time.Second))
	g.Expect(cfg.NestedConfig.NestedString).To(Equal("default-nested"))
	g.Expect(cfg.NestedConfig.NestedInt).To(Equal(100))
}

func TestLoadWithNilConfig(t *testing.T) {
	g := NewWithT(t)

	loader := NewLoader()
	err := loader.Load(nil)

	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("cannot be nil"))
}

func TestLoadWithNonPointerConfig(t *testing.T) {
	g := NewWithT(t)

	loader := NewLoader()
	cfg := testConfig{}
	err := loader.Load(cfg)

	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("must be a pointer"))
}

func TestLoadFromYAMLFile(t *testing.T) {
	g := NewWithT(t)

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `
string_value: file-string
int_value: 99
bool_value: false
duration_value: 10s
nested_config:
  nested_string: file-nested
  nested_int: 200
`
	err := os.WriteFile(configFile, []byte(yamlContent), 0644)
	g.Expect(err).ToNot(HaveOccurred())

	t.Setenv("CONTROLLER_CONFIGURATION_PATH", configFile)

	loader := NewLoader()
	cfg := defaultTestConfig()

	err = loader.Load(cfg)
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(cfg.StringValue).To(Equal("file-string"))
	g.Expect(cfg.IntValue).To(Equal(99))
	g.Expect(cfg.BoolValue).To(BeFalse())
	g.Expect(cfg.DurationValue).To(Equal(10 * time.Second))
	g.Expect(cfg.NestedConfig.NestedString).To(Equal("file-nested"))
	g.Expect(cfg.NestedConfig.NestedInt).To(Equal(200))
}

func TestLoadFromJSONFile(t *testing.T) {
	g := NewWithT(t)

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.json")

	jsonContent := `{
  "string_value": "json-string",
  "int_value": 77,
  "bool_value": false,
  "nested_config": {
    "nested_string": "json-nested",
    "nested_int": 300
  }
}`
	err := os.WriteFile(configFile, []byte(jsonContent), 0644)
	g.Expect(err).ToNot(HaveOccurred())

	t.Setenv("CONTROLLER_CONFIGURATION_PATH", configFile)

	loader := NewLoader()
	cfg := defaultTestConfig()

	err = loader.Load(cfg)
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(cfg.StringValue).To(Equal("json-string"))
	g.Expect(cfg.IntValue).To(Equal(77))
	g.Expect(cfg.BoolValue).To(BeFalse())
	g.Expect(cfg.NestedConfig.NestedString).To(Equal("json-nested"))
	g.Expect(cfg.NestedConfig.NestedInt).To(Equal(300))
}

func TestLoadFromDirectory(t *testing.T) {
	g := NewWithT(t)

	tmpDir := t.TempDir()

	// Create multiple config files
	config1 := `
string_value: dir-string
int_value: 55
`
	err := os.WriteFile(filepath.Join(tmpDir, "config1.yaml"), []byte(config1), 0644)
	g.Expect(err).ToNot(HaveOccurred())

	config2 := `
bool_value: false
nested_config:
  nested_string: dir-nested
`
	err = os.WriteFile(filepath.Join(tmpDir, "config2.yaml"), []byte(config2), 0644)
	g.Expect(err).ToNot(HaveOccurred())

	t.Setenv("CONTROLLER_CONFIGURATION_PATH", tmpDir)

	loader := NewLoader()
	cfg := defaultTestConfig()

	err = loader.Load(cfg)
	g.Expect(err).ToNot(HaveOccurred())

	// Values from both files should be merged
	g.Expect(cfg.StringValue).To(Equal("dir-string"))
	g.Expect(cfg.IntValue).To(Equal(55))
	g.Expect(cfg.BoolValue).To(BeFalse())
	g.Expect(cfg.NestedConfig.NestedString).To(Equal("dir-nested"))
	// Default value for nested_int since not set in files
	g.Expect(cfg.NestedConfig.NestedInt).To(Equal(100))
}

func TestLoadSimpleKeyValueFiles(t *testing.T) {
	g := NewWithT(t)

	tmpDir := t.TempDir()

	// Create simple key-value files (ConfigMap pattern)
	err := os.WriteFile(filepath.Join(tmpDir, "string_value"), []byte("simple-string"), 0644)
	g.Expect(err).ToNot(HaveOccurred())

	err = os.WriteFile(filepath.Join(tmpDir, "int_value"), []byte("88"), 0644)
	g.Expect(err).ToNot(HaveOccurred())

	err = os.WriteFile(filepath.Join(tmpDir, "bool_value"), []byte("false"), 0644)
	g.Expect(err).ToNot(HaveOccurred())

	t.Setenv("CONTROLLER_CONFIGURATION_PATH", tmpDir)

	loader := NewLoader()
	cfg := defaultTestConfig()

	err = loader.Load(cfg)
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(cfg.StringValue).To(Equal("simple-string"))
	g.Expect(cfg.IntValue).To(Equal(88))
	g.Expect(cfg.BoolValue).To(BeFalse())
}

func TestLoadMixedStructuredAndSimpleFiles(t *testing.T) {
	g := NewWithT(t)

	tmpDir := t.TempDir()

	// Create a structured YAML file
	yamlContent := `
nested_config:
  nested_string: yaml-nested
  nested_int: 500
`
	err := os.WriteFile(filepath.Join(tmpDir, "config.yaml"), []byte(yamlContent), 0644)
	g.Expect(err).ToNot(HaveOccurred())

	// Create simple key-value files
	err = os.WriteFile(filepath.Join(tmpDir, "string_value"), []byte("simple-string"), 0644)
	g.Expect(err).ToNot(HaveOccurred())

	err = os.WriteFile(filepath.Join(tmpDir, "int_value"), []byte("66"), 0644)
	g.Expect(err).ToNot(HaveOccurred())

	t.Setenv("CONTROLLER_CONFIGURATION_PATH", tmpDir)

	loader := NewLoader()
	cfg := defaultTestConfig()

	err = loader.Load(cfg)
	g.Expect(err).ToNot(HaveOccurred())

	// Simple files
	g.Expect(cfg.StringValue).To(Equal("simple-string"))
	g.Expect(cfg.IntValue).To(Equal(66))
	// Structured file
	g.Expect(cfg.NestedConfig.NestedString).To(Equal("yaml-nested"))
	g.Expect(cfg.NestedConfig.NestedInt).To(Equal(500))
}

func TestLoadWithEnvironmentVariables(t *testing.T) {
	g := NewWithT(t)

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `
string_value: file-string
int_value: 50
`
	err := os.WriteFile(configFile, []byte(yamlContent), 0644)
	g.Expect(err).ToNot(HaveOccurred())

	t.Setenv("CONTROLLER_CONFIGURATION_PATH", configFile)
	// Environment variables should override file values
	t.Setenv("CONTROLLER_STRING_VALUE", "env-string")
	t.Setenv("CONTROLLER_INT_VALUE", "999")

	loader := NewLoader()
	cfg := defaultTestConfig()

	err = loader.Load(cfg)
	g.Expect(err).ToNot(HaveOccurred())

	// Env vars override file values
	g.Expect(cfg.StringValue).To(Equal("env-string"))
	g.Expect(cfg.IntValue).To(Equal(999))
}

func TestLoadWithNestedEnvironmentVariables(t *testing.T) {
	g := NewWithT(t)

	// Set nested environment variable
	t.Setenv("CONTROLLER_NESTED_CONFIG_NESTED_STRING", "env-nested-string")
	t.Setenv("CONTROLLER_NESTED_CONFIG_NESTED_INT", "777")

	loader := NewLoader()
	cfg := defaultTestConfig()

	err := loader.Load(cfg)
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(cfg.NestedConfig.NestedString).To(Equal("env-nested-string"))
	g.Expect(cfg.NestedConfig.NestedInt).To(Equal(777))
}

func TestLoadWithCustomEnvPrefix(t *testing.T) {
	g := NewWithT(t)

	t.Setenv("MYAPP_STRING_VALUE", "myapp-string")
	t.Setenv("MYAPP_INT_VALUE", "123")

	loader := NewLoader(WithEnvPrefix("MYAPP"))
	cfg := defaultTestConfig()

	err := loader.Load(cfg)
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(cfg.StringValue).To(Equal("myapp-string"))
	g.Expect(cfg.IntValue).To(Equal(123))
}

func TestLoadWithCustomConfigPathEnvVar(t *testing.T) {
	g := NewWithT(t)

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `
string_value: custom-path-string
`
	err := os.WriteFile(configFile, []byte(yamlContent), 0644)
	g.Expect(err).ToNot(HaveOccurred())

	t.Setenv("CUSTOM_CONFIG_PATH", configFile)

	loader := NewLoader(WithConfigPathEnvVar("CUSTOM_CONFIG_PATH"))
	cfg := defaultTestConfig()

	err = loader.Load(cfg)
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(cfg.StringValue).To(Equal("custom-path-string"))
}

func TestLoadPrecedenceOrder(t *testing.T) {
	g := NewWithT(t)

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	// File sets value to 10
	yamlContent := `
int_value: 10
`
	err := os.WriteFile(configFile, []byte(yamlContent), 0644)
	g.Expect(err).ToNot(HaveOccurred())

	t.Setenv("CONTROLLER_CONFIGURATION_PATH", configFile)
	// Env var sets value to 20 (should override file)
	t.Setenv("CONTROLLER_INT_VALUE", "20")

	loader := NewLoader()
	// Default sets value to 42
	cfg := defaultTestConfig()

	err = loader.Load(cfg)
	g.Expect(err).ToNot(HaveOccurred())

	// Env var (20) should win over file (10) and default (42)
	g.Expect(cfg.IntValue).To(Equal(20))
}

func TestLoadFromNonExistentPath(t *testing.T) {
	g := NewWithT(t)

	t.Setenv("CONTROLLER_CONFIGURATION_PATH", "/nonexistent/path/config.yaml")

	loader := NewLoader()
	cfg := defaultTestConfig()

	err := loader.Load(cfg)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("failed to stat config path"))
}

func TestLoadFromEmptyDirectory(t *testing.T) {
	g := NewWithT(t)

	tmpDir := t.TempDir()
	t.Setenv("CONTROLLER_CONFIGURATION_PATH", tmpDir)

	loader := NewLoader()
	cfg := defaultTestConfig()

	err := loader.Load(cfg)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("no configuration files found"))
}

func TestLoadSkipsHiddenFiles(t *testing.T) {
	g := NewWithT(t)

	tmpDir := t.TempDir()

	// Create a hidden file
	err := os.WriteFile(filepath.Join(tmpDir, ".hidden"), []byte("should-not-load"), 0644)
	g.Expect(err).ToNot(HaveOccurred())

	// Create a normal file
	err = os.WriteFile(filepath.Join(tmpDir, "string_value"), []byte("visible-file"), 0644)
	g.Expect(err).ToNot(HaveOccurred())

	t.Setenv("CONTROLLER_CONFIGURATION_PATH", tmpDir)

	loader := NewLoader()
	cfg := defaultTestConfig()

	err = loader.Load(cfg)
	g.Expect(err).ToNot(HaveOccurred())

	// Should load from visible file
	g.Expect(cfg.StringValue).To(Equal("visible-file"))
}

func TestLoadSkipsDirectoriesInDirectory(t *testing.T) {
	g := NewWithT(t)

	tmpDir := t.TempDir()

	// Create a subdirectory
	subDir := filepath.Join(tmpDir, "subdir")
	err := os.Mkdir(subDir, 0755)
	g.Expect(err).ToNot(HaveOccurred())

	// Create a file in the subdirectory
	err = os.WriteFile(filepath.Join(subDir, "config.yaml"), []byte("string_value: sub"), 0644)
	g.Expect(err).ToNot(HaveOccurred())

	// Create a file in the main directory
	err = os.WriteFile(filepath.Join(tmpDir, "string_value"), []byte("main"), 0644)
	g.Expect(err).ToNot(HaveOccurred())

	t.Setenv("CONTROLLER_CONFIGURATION_PATH", tmpDir)

	loader := NewLoader()
	cfg := defaultTestConfig()

	err = loader.Load(cfg)
	g.Expect(err).ToNot(HaveOccurred())

	// Should only load from main directory, not subdirectory
	g.Expect(cfg.StringValue).To(Equal("main"))
}

