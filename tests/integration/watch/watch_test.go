package watch_test

import (
	"context"
	"testing"

	"github.com/lburgazzoli/k3s-envtest/pkg/k3senv"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/reconciler"
	"github.com/lburgazzoli/k8s-controller-lib/pkg/reconciler/watch"
	"github.com/lburgazzoli/k8s-controller-lib/pkg/resources/gvks"
	"github.com/lburgazzoli/k8s-controller-lib/tests/integration/support"

	. "github.com/onsi/gomega"
)

func TestWatchConfig(t *testing.T) {
	t.Run("watch config", func(t *testing.T) {
		g := NewWithT(t)

		cfg := watch.NewConfig(gvks.Deployment, watch.WithPredicates())

		g.Expect(cfg.GVK).To(Equal(gvks.Deployment))
		g.Expect(cfg.Predicates).To(BeEmpty())
		g.Expect(cfg.Handler).To(BeNil())
	})
}

func TestWatchConfig_WithPredicates(t *testing.T) {
	t.Run("watch config with predicates", func(t *testing.T) {
		g := NewWithT(t)

		pred := predicate.GenerationChangedPredicate{}
		cfg := watch.NewConfig(gvks.Deployment, watch.WithPredicates(pred))

		g.Expect(cfg.GVK).To(Equal(gvks.Deployment))
		g.Expect(cfg.Predicates).To(HaveLen(1))
		g.Expect(cfg.Predicates[0]).To(Equal(pred))
	})
}

func TestWatchConfig_WithHandler(t *testing.T) {
	t.Run("watch config with handler", func(t *testing.T) {
		g := NewWithT(t)

		cfg := watch.NewConfig(gvks.Deployment, watch.WithHandler(nil))

		g.Expect(cfg.GVK).To(Equal(gvks.Deployment))
		g.Expect(cfg.Handler).To(BeNil())
	})
}

func TestWatchConfig_MultipleOptions(t *testing.T) {
	t.Run("watch config with multiple options", func(t *testing.T) {
		g := NewWithT(t)

		pred1 := predicate.GenerationChangedPredicate{}
		pred2 := predicate.ResourceVersionChangedPredicate{}

		cfg := watch.NewConfig(gvks.Deployment, watch.WithPredicates(pred1, pred2), watch.WithHandler(nil))

		g.Expect(cfg.GVK).To(Equal(gvks.Deployment))
		g.Expect(cfg.Predicates).To(HaveLen(2))
		g.Expect(cfg.Predicates[0]).To(Equal(pred1))
		g.Expect(cfg.Predicates[1]).To(Equal(pred2))
		g.Expect(cfg.Handler).To(BeNil())
	})
}

func TestWatchConfig_Disabled(t *testing.T) {
	t.Run("watch config disabled", func(t *testing.T) {
		g := NewWithT(t)

		cfg := watch.NewConfig(gvks.Deployment, watch.Disabled())

		g.Expect(cfg.GVK).To(Equal(gvks.Deployment))
		g.Expect(cfg.Disabled).To(BeTrue())
	})
}

func TestConfig_StructBased(t *testing.T) {
	t.Run("config struct based", func(t *testing.T) {
		g := NewWithT(t)

		pred := predicate.GenerationChangedPredicate{}
		hdler := &handler.EnqueueRequestForObject{}

		cfg := watch.Config{
			GVK:        gvks.Deployment,
			Predicates: []predicate.Predicate{pred},
			Handler:    hdler,
			Disabled:   false,
		}

		g.Expect(cfg.GVK).To(Equal(gvks.Deployment))
		g.Expect(cfg.Predicates).To(HaveLen(1))
		g.Expect(cfg.Predicates[0]).To(Equal(pred))
		g.Expect(cfg.Handler).To(Equal(hdler))
		g.Expect(cfg.Disabled).To(BeFalse())
	})
}

func TestConfig_StructBased_Disabled(t *testing.T) {
	t.Run("config struct based disabled", func(t *testing.T) {
		g := NewWithT(t)

		cfg := watch.Config{
			GVK:      gvks.Job,
			Disabled: true,
		}

		g.Expect(cfg.GVK).To(Equal(gvks.Job))
		g.Expect(cfg.Predicates).To(BeEmpty())
		g.Expect(cfg.Handler).To(BeNil())
		g.Expect(cfg.Disabled).To(BeTrue())
	})
}

func TestConfigOptions_StructBased(t *testing.T) {
	t.Run("config options struct based", func(t *testing.T) {
		g := NewWithT(t)

		pred := predicate.GenerationChangedPredicate{}
		hdler := &handler.EnqueueRequestForObject{}

		cfg := watch.NewConfig(gvks.StatefulSet, &watch.ConfigOptions{
			Predicates: []predicate.Predicate{pred},
			Handler:    hdler,
			Disabled:   true,
		})

		g.Expect(cfg.GVK).To(Equal(gvks.StatefulSet))
		g.Expect(cfg.Predicates).To(HaveLen(1))
		g.Expect(cfg.Predicates[0]).To(Equal(pred))
		g.Expect(cfg.Handler).To(Equal(hdler))
		g.Expect(cfg.Disabled).To(BeTrue())
	})
}

func TestConfigOptions_PartialStructBased(t *testing.T) {
	t.Run("config options partial struct based", func(t *testing.T) {
		g := NewWithT(t)

		pred := predicate.GenerationChangedPredicate{}

		cfg := watch.NewConfig(gvks.DaemonSet, &watch.ConfigOptions{
			Predicates: []predicate.Predicate{pred},
		})

		g.Expect(cfg.GVK).To(Equal(gvks.DaemonSet))
		g.Expect(cfg.Predicates).To(HaveLen(1))
		g.Expect(cfg.Predicates[0]).To(Equal(pred))
		g.Expect(cfg.Handler).To(BeNil())
		g.Expect(cfg.Disabled).To(BeFalse())
	})
}

func TestWithConfigs(t *testing.T) {
	t.Run("with configs", func(t *testing.T) {
		g := NewWithT(t)

		cfg1 := watch.NewConfig(gvks.Deployment, watch.WithPredicates(predicate.GenerationChangedPredicate{}))
		cfg2 := watch.NewConfig(gvks.StatefulSet, watch.Disabled())

		opts := &watch.Options{
			Configs: []watch.Config{cfg1, cfg2},
		}

		g.Expect(opts.Configs).To(HaveLen(2))
		g.Expect(opts.Configs[0].GVK).To(Equal(gvks.Deployment))
		g.Expect(opts.Configs[1].GVK).To(Equal(gvks.StatefulSet))
		g.Expect(opts.Configs[1].Disabled).To(BeTrue())
	})
}

func TestOptions_StructBased(t *testing.T) {
	t.Run("options struct based", func(t *testing.T) {
		g := NewWithT(t)

		cfg1 := watch.NewConfig(gvks.Deployment, watch.WithPredicates())
		cfg2 := watch.NewConfig(gvks.Job, watch.WithHandler(nil))

		opts := &watch.Options{
			Configs: []watch.Config{cfg1, cfg2},
		}

		g.Expect(opts.Configs).To(HaveLen(2))
		g.Expect(opts.Configs[0].GVK).To(Equal(gvks.Deployment))
		g.Expect(opts.Configs[1].GVK).To(Equal(gvks.Job))
	})
}

func TestMixedConfigurationApproaches(t *testing.T) {
	t.Run("mixed configuration approaches", func(t *testing.T) {
		g := NewWithT(t)

		pred := predicate.GenerationChangedPredicate{}

		cfgFunctional := watch.NewConfig(
			gvks.Deployment,
			watch.WithPredicates(pred),
		)

		cfgStruct := watch.Config{
			GVK:      gvks.StatefulSet,
			Disabled: true,
		}

		opts := &watch.Options{
			Configs: []watch.Config{cfgFunctional, cfgStruct},
		}

		g.Expect(opts.Configs).To(HaveLen(2))

		g.Expect(opts.Configs[0].GVK).To(Equal(gvks.Deployment))
		g.Expect(opts.Configs[0].Predicates).To(HaveLen(1))
		g.Expect(opts.Configs[0].Predicates[0]).To(Equal(pred))
		g.Expect(opts.Configs[0].Disabled).To(BeFalse())

		g.Expect(opts.Configs[1].GVK).To(Equal(gvks.StatefulSet))
		g.Expect(opts.Configs[1].Disabled).To(BeTrue())
	})
}

func TestMixedOptionsInSingleCall(t *testing.T) {
	t.Run("mixed options in single call", func(t *testing.T) {
		g := NewWithT(t)

		pred := predicate.GenerationChangedPredicate{}
		hdler := &handler.EnqueueRequestForObject{}

		cfg := watch.NewConfig(
			gvks.ReplicaSet,
			&watch.ConfigOptions{
				Predicates: []predicate.Predicate{pred},
			},
			watch.WithHandler(hdler),
		)

		g.Expect(cfg.GVK).To(Equal(gvks.ReplicaSet))
		g.Expect(cfg.Predicates).To(HaveLen(1))
		g.Expect(cfg.Predicates[0]).To(Equal(pred))
		g.Expect(cfg.Handler).To(Equal(hdler))
		g.Expect(cfg.Disabled).To(BeFalse())
	})
}

func TestWatcherMetrics(t *testing.T) {
	// Setup shared test environment
	g := NewWithT(t)
	ctx := context.Background()

	s := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(s)).ShouldNot(HaveOccurred())
	g.Expect(appsv1.AddToScheme(s)).ShouldNot(HaveOccurred())

	env, err := k3senv.New(
		k3senv.WithScheme(s),
	)

	g.Expect(err).ToNot(HaveOccurred())

	err = env.Start(ctx)
	g.Expect(err).ToNot(HaveOccurred())
	t.Cleanup(func() {
		_ = env.Stop(ctx)
	})

	// Create cache
	cacheObj, err := cache.New(env.Config(), cache.Options{
		Scheme: s,
	})
	g.Expect(err).ToNot(HaveOccurred())

	// Start cache in background
	cacheCtx, cacheCancel := context.WithCancel(ctx)
	t.Cleanup(func() {
		cacheCancel()
	})

	go func() {
		_ = cacheObj.Start(cacheCtx)
	}()

	// Wait for cache to sync
	g.Expect(cacheObj.WaitForCacheSync(ctx)).To(BeTrue())

	cli := env.Client()

	t.Run("single watch increments metric", func(t *testing.T) {
		g := NewWithT(t)

		ctrl, err := controller.NewUnmanaged("test-single-watch", controller.Options{
			Reconciler: &support.NoOpReconciler{},
		})
		g.Expect(err).ToNot(HaveOccurred())

		watcher := watch.New(watch.WithClient(cli), watch.WithController(ctrl), watch.WithCache(cacheObj))

		// Create owner object
		owner := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "owner-pod",
				Namespace: "default",
			},
		}

		// Watch ConfigMap with controller name in context
		ctx := reconciler.WithControllerName(context.Background(), "test-single-watch")
		cm := &corev1.ConfigMap{}
		err = watcher.Watch(ctx, owner, []client.Object{cm})
		g.Expect(err).ToNot(HaveOccurred())

		// Verify metric
		value := testutil.ToFloat64(watch.DynamicWatchedResourcesTotal.WithLabelValues(
			"test-single-watch",
			"v1",
			"ConfigMap",
		))
		g.Expect(value).To(Equal(1.0))
	})

	t.Run("watch registered only once per GVK", func(t *testing.T) {
		g := NewWithT(t)

		ctrl, err := controller.NewUnmanaged("test-watch-once", controller.Options{
			Reconciler: &support.NoOpReconciler{},
		})
		g.Expect(err).ToNot(HaveOccurred())

		watcher := watch.New(watch.WithClient(cli), watch.WithController(ctrl), watch.WithCache(cacheObj))

		owner := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "owner-pod-2",
				Namespace: "default",
			},
		}

		// Watch 2 different ConfigMap instances with controller name in context
		ctx := reconciler.WithControllerName(context.Background(), "test-watch-once")
		cm1 := &corev1.ConfigMap{}
		cm2 := &corev1.ConfigMap{}
		err = watcher.Watch(ctx, owner, []client.Object{cm1, cm2})
		g.Expect(err).ToNot(HaveOccurred())

		// Metric should still be 1, not 2
		value := testutil.ToFloat64(watch.DynamicWatchedResourcesTotal.WithLabelValues(
			"test-watch-once",
			"v1",
			"ConfigMap",
		))
		g.Expect(value).To(Equal(1.0))
	})

	t.Run("multiple GVKs create separate metrics", func(t *testing.T) {
		g := NewWithT(t)

		ctrl, err := controller.NewUnmanaged("test-multi-gvk", controller.Options{
			Reconciler: &support.NoOpReconciler{},
		})
		g.Expect(err).ToNot(HaveOccurred())

		watcher := watch.New(watch.WithClient(cli), watch.WithController(ctrl), watch.WithCache(cacheObj))

		owner := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "owner-pod-3",
				Namespace: "default",
			},
		}

		// Watch ConfigMap with controller name in context
		ctx := reconciler.WithControllerName(context.Background(), "test-multi-gvk")
		cm := &corev1.ConfigMap{}
		err = watcher.Watch(ctx, owner, []client.Object{cm})
		g.Expect(err).ToNot(HaveOccurred())

		// Watch Deployment
		deploy := &appsv1.Deployment{}
		err = watcher.Watch(ctx, owner, []client.Object{deploy})
		g.Expect(err).ToNot(HaveOccurred())

		// Verify separate metrics
		cmValue := testutil.ToFloat64(watch.DynamicWatchedResourcesTotal.WithLabelValues(
			"test-multi-gvk",
			"v1",
			"ConfigMap",
		))
		g.Expect(cmValue).To(Equal(1.0))

		deployValue := testutil.ToFloat64(watch.DynamicWatchedResourcesTotal.WithLabelValues(
			"test-multi-gvk",
			"apps/v1",
			"Deployment",
		))
		g.Expect(deployValue).To(Equal(1.0))
	})

	t.Run("disabled watch does not increment metric", func(t *testing.T) {
		g := NewWithT(t)

		ctrl, err := controller.NewUnmanaged("test-disabled", controller.Options{
			Reconciler: &support.NoOpReconciler{},
		})
		g.Expect(err).ToNot(HaveOccurred())

		watcher := watch.New(
			watch.WithClient(cli),
			watch.WithController(ctrl),
			watch.WithCache(cacheObj),
			watch.WithConfigs(
				watch.NewConfig(gvks.Secret, watch.Disabled()),
			),
		)

		owner := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "owner-pod-4",
				Namespace: "default",
			},
		}

		// Try to watch Secret (but it's disabled) with controller name in context
		ctx := reconciler.WithControllerName(context.Background(), "test-disabled")
		secret := &corev1.Secret{}
		err = watcher.Watch(ctx, owner, []client.Object{secret})
		g.Expect(err).ToNot(HaveOccurred())

		// Metric should be 0
		value := testutil.ToFloat64(watch.DynamicWatchedResourcesTotal.WithLabelValues(
			"test-disabled",
			"v1",
			"Secret",
		))
		g.Expect(value).To(Equal(0.0))
	})

	t.Run("disabled config skips registration and metric", func(t *testing.T) {
		g := NewWithT(t)

		ctrl, err := controller.NewUnmanaged("test-external", controller.Options{
			Reconciler: &support.NoOpReconciler{},
		})
		g.Expect(err).ToNot(HaveOccurred())

		// Create watcher with ConfigMap marked as disabled (externally watched)
		watcher := watch.New(
			watch.WithClient(cli),
			watch.WithController(ctrl),
			watch.WithCache(cacheObj),
			watch.WithConfigs(watch.NewConfig(gvks.ConfigMap, watch.Disabled())),
		)

		owner := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "owner-pod-external",
				Namespace: "default",
			},
		}

		// Try to watch ConfigMap (but it's disabled - should be skipped)
		ctx := reconciler.WithControllerName(context.Background(), "test-external")
		cm := &corev1.ConfigMap{}
		err = watcher.Watch(ctx, owner, []client.Object{cm})
		g.Expect(err).ToNot(HaveOccurred())

		// Metric should be 0 (no dynamic watch was registered)
		value := testutil.ToFloat64(watch.DynamicWatchedResourcesTotal.WithLabelValues(
			"test-external",
			"v1",
			"ConfigMap",
		))
		g.Expect(value).To(Equal(0.0))

		// Verify state shows it as disabled
		state := watcher.State(gvks.ConfigMap)
		g.Expect(state).ToNot(BeNil())
		g.Expect(state.Config.Disabled).To(BeTrue(), "Config should be disabled")
	})

	t.Run("disabled config allows non-disabled GVKs to be watched", func(t *testing.T) {
		g := NewWithT(t)

		ctrl, err := controller.NewUnmanaged("test-external-mixed", controller.Options{
			Reconciler: &support.NoOpReconciler{},
		})
		g.Expect(err).ToNot(HaveOccurred())

		// Create watcher with ConfigMap marked as disabled (externally watched)
		watcher := watch.New(
			watch.WithClient(cli),
			watch.WithController(ctrl),
			watch.WithCache(cacheObj),
			watch.WithConfigs(watch.NewConfig(gvks.ConfigMap, watch.Disabled())),
		)

		owner := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "owner-pod-external-mixed",
				Namespace: "default",
			},
		}

		ctx := reconciler.WithControllerName(context.Background(), "test-external-mixed")

		// Watch both ConfigMap (external) and Secret (not external)
		cm := &corev1.ConfigMap{}
		secret := &corev1.Secret{}
		err = watcher.Watch(ctx, owner, []client.Object{cm, secret})
		g.Expect(err).ToNot(HaveOccurred())

		// ConfigMap metric should be 0 (external)
		cmValue := testutil.ToFloat64(watch.DynamicWatchedResourcesTotal.WithLabelValues(
			"test-external-mixed",
			"v1",
			"ConfigMap",
		))
		g.Expect(cmValue).To(Equal(0.0), "ConfigMap should not increment metric (external)")

		// Secret metric should be 1 (dynamically watched)
		secretValue := testutil.ToFloat64(watch.DynamicWatchedResourcesTotal.WithLabelValues(
			"test-external-mixed",
			"v1",
			"Secret",
		))
		g.Expect(secretValue).To(Equal(1.0), "Secret should be dynamically watched")

		// ConfigMap should be disabled (not watched), Secret should be watched
		cmState := watcher.State(gvks.ConfigMap)
		g.Expect(cmState).ToNot(BeNil())
		g.Expect(cmState.Config.Disabled).To(BeTrue(), "ConfigMap should be disabled")
		g.Expect(cmState.Watched).To(BeFalse(), "ConfigMap should not be watched (disabled)")

		secretState := watcher.State(gvks.Secret)
		g.Expect(secretState).ToNot(BeNil())
		g.Expect(secretState.Watched).To(BeTrue(), "Secret should be watched")
	})
}
