package main

import (
	"errors"
	"flag"
	"os"

	"github.com/spf13/pflag"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	cleanupApi "github.com/lburgazzoli/k8s-controller-lib/examples/cleanup-controller/api/v1alpha1"
	"github.com/lburgazzoli/k8s-controller-lib/examples/cleanup-controller/internal/controller/cleanup"
	"github.com/lburgazzoli/k8s-controller-lib/pkg/config"
	configzap "github.com/lburgazzoli/k8s-controller-lib/pkg/config/zap"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

// CleanupControllerConfig holds the controller configuration.
type CleanupControllerConfig struct {
	Zap             configzap.Config `mapstructure:"zap"`
	MetricsBindAddr string           `flag:"metrics-bind-addr,Address for metrics server" mapstructure:"metrics_bind_addr"`
	LeaderElection  bool             `flag:"leader-election,Enable leader election" mapstructure:"leader_election"`
}

// Validate implements config.Validator to validate the controller configuration.
func (c *CleanupControllerConfig) Validate() error {
	if c.MetricsBindAddr == "" {
		return errors.New("metrics-bind-addr cannot be empty")
	}

	return nil
}

// DefaultConfig returns configuration with sensible defaults.
func DefaultConfig() *CleanupControllerConfig {
	return &CleanupControllerConfig{
		Zap: configzap.Config{
			Development:     true,
			Level:           "info",
			StacktraceLevel: "warn",
			Encoder:         "console",
			TimeEncoding:    "iso8601",
		},
		MetricsBindAddr: ":8080",
		LeaderElection:  false,
	}
}

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = cleanupApi.AddToScheme(scheme)
}

func main() {
	cfg := DefaultConfig()

	// Bridge standard flag package with pflag for zap compatibility
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)

	loader, err := config.For(
		cfg,
		config.WithEnvPrefix("CLEANUP_CONTROLLER"),
		config.WithConfigPathEnvVar("CLEANUP_CONTROLLER_CONFIG_PATH"),
		config.WithFlags(pflag.CommandLine),
	)
	if err != nil {
		setupLog.Error(err, "failed to create config loader")
		os.Exit(1)
	}

	pflag.Parse()

	if err := loader.Load(); err != nil {
		setupLog.Error(err, "failed to load configuration")
		os.Exit(1)
	}

	// Configure logger from config
	zapOpts, err := cfg.Zap.ToOptions()
	if err != nil {
		setupLog.Error(err, "failed to configure logger")
		os.Exit(1)
	}
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&zapOpts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:           scheme,
		LeaderElection:   cfg.LeaderElection,
		LeaderElectionID: "cleanup-controller.example.com",
		Metrics:          metricsserver.Options{BindAddress: cfg.MetricsBindAddr},
	})
	if err != nil {
		setupLog.Error(err, "unable to create manager")
		os.Exit(1)
	}

	// Setup the CleanupApp controller
	if err := cleanup.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to setup controller")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
