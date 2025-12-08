package main

import (
	"flag"
	"os"

	"github.com/spf13/pflag"
	"go.uber.org/zap/zapcore"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	simpleApi "github.com/lburgazzoli/k8s-controller-lib/examples/simple-controller/api/v1alpha1"
	"github.com/lburgazzoli/k8s-controller-lib/examples/simple-controller/internal/controller/simple"
	"github.com/lburgazzoli/k8s-controller-lib/pkg/config"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

// SimpleControllerConfig holds the controller configuration
type SimpleControllerConfig struct {
	FeatureFlags    FeatureFlags `mapstructure:"feature_flags"`
	MetricsBindAddr string       `mapstructure:"metrics_bind_addr"`
	LeaderElection  bool         `mapstructure:"leader_election"`
}

// FeatureFlags contains feature flag settings
type FeatureFlags struct {
	EnableDebugLogging bool `mapstructure:"enable_debug_logging"`
}

// DefaultConfig returns configuration with sensible defaults
func DefaultConfig() *SimpleControllerConfig {
	return &SimpleControllerConfig{
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
	// Initialize configuration loader
	loader := config.NewLoader(
		config.WithEnvPrefix("SIMPLE_CONTROLLER"),
		config.WithConfigPathEnvVar("SIMPLE_CONTROLLER_CONFIG_PATH"),
	)

	opts := zap.Options{
		Development:     true,
		StacktraceLevel: zapcore.WarnLevel,
		DestWriter:      os.Stdout,
	}

	// Bind zap options to standard flag library
	opts.BindFlags(flag.CommandLine)

	// Bridge: Add standard flags to pflag so both work together
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)

	// Bind config loader to pflag
	loader.BindFlags(pflag.CommandLine)

	// Parse all flags (both standard and pflag)
	pflag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// Load configuration from all sources
	cfg := DefaultConfig()
	if err := loader.Load(cfg); err != nil {
		setupLog.Error(err, "failed to load configuration")
		os.Exit(1)
	}

	// Log loaded configuration for debugging
	setupLog.Info("loaded configuration",
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
