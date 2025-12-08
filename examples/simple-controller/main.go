package main

import (
	"errors"
	"os"

	"github.com/spf13/pflag"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	simpleApi "github.com/lburgazzoli/k8s-controller-lib/examples/simple-controller/api/v1alpha1"
	"github.com/lburgazzoli/k8s-controller-lib/examples/simple-controller/internal/controller/simple"
	"github.com/lburgazzoli/k8s-controller-lib/pkg/config"
	configzap "github.com/lburgazzoli/k8s-controller-lib/pkg/config/zap"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

// SimpleControllerConfig holds the controller configuration.
type SimpleControllerConfig struct {
	Zap             configzap.Config `mapstructure:"zap"`
	FeatureFlags    FeatureFlags     `mapstructure:"feature_flags"`
	MetricsBindAddr string           `flag:"metrics-bind-addr,Address for metrics server" mapstructure:"metrics_bind_addr"`
	LeaderElection  bool             `flag:"leader-election,Enable leader election"      mapstructure:"leader_election"`
}

// FeatureFlags contains feature flag settings.
type FeatureFlags struct {
	EnableDebugLogging bool `flag:"enable-debug-logging,Enable verbose debug logging" mapstructure:"enable_debug_logging"`
}

// Validate implements config.Validator to validate the controller configuration.
func (c *SimpleControllerConfig) Validate() error {
	if c.MetricsBindAddr == "" {
		return errors.New("metrics-bind-addr cannot be empty")
	}

	return nil
}

// DefaultConfig returns configuration with sensible defaults
func DefaultConfig() *SimpleControllerConfig {
	return &SimpleControllerConfig{
		Zap: configzap.Config{
			Development:     true,
			Level:           "info",
			StacktraceLevel: "warn",
			Encoder:         "console",
			TimeEncoding:    "iso8601",
		},
		FeatureFlags: FeatureFlags{
			EnableDebugLogging: false,
		},
		MetricsBindAddr: ":8080",
		LeaderElection:  false,
	}
}

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = simpleApi.AddToScheme(scheme)
}

func main() {
	// Get default configuration
	cfg := DefaultConfig()

	// Create configuration loader with automatic flag binding
	loader, err := config.For(
		cfg,
		config.WithEnvPrefix("SIMPLE_CONTROLLER"),
		config.WithConfigPathEnvVar("SIMPLE_CONTROLLER_CONFIG_PATH"),
		config.WithFlags(pflag.CommandLine),
	)
	if err != nil {
		setupLog.Error(err, "failed to create config loader")
		os.Exit(1)
	}

	// Parse flags
	pflag.Parse()

	// Load configuration from all sources (precedence: defaults → files → env → flags)
	if err := loader.Load(); err != nil {
		setupLog.Error(err, "failed to load configuration")
		os.Exit(1)
	}

	// Convert zap config to options and initialize logger
	zapOpts, err := cfg.Zap.ToOptions()
	if err != nil {
		setupLog.Error(err, "failed to create zap options")
		os.Exit(1)
	}
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&zapOpts)))

	// Log loaded configuration for debugging
	setupLog.Info("loaded configuration",
		"zapDevelopment", cfg.Zap.Development,
		"zapLevel", cfg.Zap.Level,
		"zapEncoder", cfg.Zap.Encoder,
		"enableDebugLogging", cfg.FeatureFlags.EnableDebugLogging,
		"metricsBindAddr", cfg.MetricsBindAddr,
		"leaderElection", cfg.LeaderElection)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:           scheme,
		LeaderElection:   cfg.LeaderElection,
		LeaderElectionID: "simple-controller.example.com",
		Metrics:          metricsserver.Options{BindAddress: cfg.MetricsBindAddr},
	})
	if err != nil {
		setupLog.Error(err, "unable to create manager")
		os.Exit(1)
	}

	// Setup the SimpleApp controller
	if err := simple.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to setup controller")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
