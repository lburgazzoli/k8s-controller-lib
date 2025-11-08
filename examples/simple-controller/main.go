package main

import (
	"flag"
	"os"

	"go.uber.org/zap/zapcore"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	simpleApi "github.com/lburgazzoli/k8s-controller-lib/examples/simple-controller/api/v1alpha1"
	"github.com/lburgazzoli/k8s-controller-lib/examples/simple-controller/internal/controller/simple"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = simpleApi.AddToScheme(scheme)
}

func main() {
	opts := zap.Options{
		Development:     true,
		StacktraceLevel: zapcore.WarnLevel,
		DestWriter:      os.Stdout,
	}

	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:           scheme,
		LeaderElection:   false,
		LeaderElectionID: "simple-controller.example.com",
		Metrics:          metricsserver.Options{BindAddress: ":8080"},
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
