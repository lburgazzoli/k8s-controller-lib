package watch_test

import (
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/mock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/reconciler"
	"github.com/lburgazzoli/k8s-controller-lib/pkg/reconciler/watch"
	"github.com/lburgazzoli/k8s-controller-lib/pkg/resources/gvks"
	"github.com/lburgazzoli/k8s-controller-lib/pkg/util/test/mocks"

	. "github.com/onsi/gomega"
)

// setupTestWatcher creates a watcher with mock dependencies for testing.
func setupTestWatcher(
	t *testing.T,
	cli client.Client,
	opts ...watch.Option,
) (*watch.Watcher, *mocks.Controller, client.Client, *runtime.Scheme) {
	t.Helper()

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	if cli == nil {
		cli = fake.NewClientBuilder().WithScheme(scheme).Build()
	}

	mockCtrl := mocks.NewController()
	mockCacheObj := mocks.NewCache()

	// Set up default mock behaviors
	mockCtrl.On("Watch", mock.Anything).Return(nil)

	allOpts := make([]watch.Option, 0, 3+len(opts))
	allOpts = append(allOpts, watch.WithClient(cli), watch.WithController(mockCtrl), watch.WithCache(mockCacheObj))
	allOpts = append(allOpts, opts...)

	watcher := watch.New(allOpts...)

	return watcher, mockCtrl, cli, scheme
}

func TestWatcher_ConcurrentWatchSameGVK(t *testing.T) {
	g := NewWithT(t)

	watcher, mockCtrl, cli, _ := setupTestWatcher(t, nil)

	owner := &corev1.Pod{}
	owner.SetName("owner")
	owner.SetNamespace("default")
	owner.SetUID("owner-uid")

	ctx := reconciler.WithControllerName(t.Context(), "test-controller")

	// Create owner in the fake client so SetControllerReference works
	err := cli.Create(ctx, owner)
	g.Expect(err).ToNot(HaveOccurred())

	const goroutines = 10
	var wg sync.WaitGroup

	// Launch multiple goroutines trying to watch the same GVK concurrently
	for range goroutines {
		wg.Go(func() {
			cm := &corev1.ConfigMap{}
			cm.SetName("test")
			cm.SetNamespace("default")

			err := watcher.Watch(ctx, owner, cm)
			g.Expect(err).ToNot(HaveOccurred())
		})
	}

	wg.Wait()

	// Verify watch was registered exactly once despite concurrent calls
	mockCtrl.AssertNumberOfCalls(t, "Watch", 1)

	// Verify state is consistent
	state := watcher.State(gvks.ConfigMap)

	g.Expect(state).ToNot(BeNil())
	g.Expect(state.Watched).To(BeTrue())
}

func TestWatcher_ConcurrentWatchDifferentGVKs(t *testing.T) {
	g := NewWithT(t)

	watcher, mockCtrl, cli, _ := setupTestWatcher(t, nil)

	owner := &corev1.Pod{}
	owner.SetName("owner")
	owner.SetNamespace("default")
	owner.SetUID("owner-uid")

	ctx := reconciler.WithControllerName(t.Context(), "test-controller")

	// Create owner in the fake client
	err := cli.Create(ctx, owner)
	g.Expect(err).ToNot(HaveOccurred())

	var wg sync.WaitGroup

	// Goroutine 1: Watch ConfigMap
	wg.Go(func() {
		cm := &corev1.ConfigMap{}
		cm.SetName("test-cm")
		cm.SetNamespace("default")

		err := watcher.Watch(ctx, owner, cm)
		g.Expect(err).ToNot(HaveOccurred())
	})

	// Goroutine 2: Watch Secret
	wg.Go(func() {
		secret := &corev1.Secret{}
		secret.SetName("test-secret")
		secret.SetNamespace("default")

		err := watcher.Watch(ctx, owner, secret)
		g.Expect(err).ToNot(HaveOccurred())
	})

	wg.Wait()

	// Both watches should succeed
	mockCtrl.AssertNumberOfCalls(t, "Watch", 2)

	// Verify both states exist
	cmState := watcher.State(gvks.ConfigMap)
	secretState := watcher.State(gvks.Secret)

	g.Expect(cmState).ToNot(BeNil())
	g.Expect(cmState.Watched).To(BeTrue())
	g.Expect(secretState).ToNot(BeNil())
	g.Expect(secretState.Watched).To(BeTrue())
}

func TestWatcher_WatchIdempotency(t *testing.T) {
	g := NewWithT(t)

	watcher, mockCtrl, cli, _ := setupTestWatcher(t, nil)

	owner := &corev1.Pod{}
	owner.SetName("owner")
	owner.SetNamespace("default")
	owner.SetUID("owner-uid")

	ctx := reconciler.WithControllerName(t.Context(), "test-controller")

	err := cli.Create(ctx, owner)
	g.Expect(err).ToNot(HaveOccurred())

	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
	}
	cm.SetName("test")
	cm.SetNamespace("default")

	// Call Watch multiple times sequentially
	for i := range 5 {
		err = watcher.Watch(ctx, owner, cm)
		if err != nil {
			t.Fatalf("Watch() iteration %d failed: %v", i, err)
		}
	}

	// Verify watch was only registered once
	mockCtrl.AssertNumberOfCalls(t, "Watch", 1)

	state := watcher.State(gvks.ConfigMap)
	g.Expect(state).ToNot(BeNil(), "State is nil for ConfigMap GVK=%v - watch failed silently", gvks.ConfigMap)
	g.Expect(state.Watched).To(BeTrue())
}

func TestWatcher_SetupWatchError(t *testing.T) {
	g := NewWithT(t)

	// Create mocks with error return
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()

	mockCtrl := mocks.NewController()
	mockCacheObj := mocks.NewCache()

	// Configure mock to return error
	mockCtrl.On("Watch", mock.Anything).Return(errors.New("watch registration failed"))

	watcher := watch.New(watch.WithClient(cli), watch.WithController(mockCtrl), watch.WithCache(mockCacheObj))

	owner := &corev1.Pod{}
	owner.SetName("owner")
	owner.SetNamespace("default")
	owner.SetUID("owner-uid")

	ctx := reconciler.WithControllerName(t.Context(), "test-controller")

	// Create owner in client
	err := cli.Create(ctx, owner)
	g.Expect(err).ToNot(HaveOccurred())

	cm := &corev1.ConfigMap{}
	cm.SetName("test")
	cm.SetNamespace("default")

	err = watcher.Watch(ctx, owner, cm)

	// Error should be propagated
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("failed to watch"))
	g.Expect(err.Error()).To(ContainSubstring("watch registration failed"))

	// State should exist but not be marked as watched
	state := watcher.State(gvks.ConfigMap)

	g.Expect(state).ToNot(BeNil())
	g.Expect(state.Watched).To(BeFalse())
}

func TestWatcher_DisabledWatch(t *testing.T) {
	g := NewWithT(t)

	// Create watcher with disabled ConfigMap watch
	watcher, mockCtrl, cli, _ := setupTestWatcher(t, nil,
		watch.WithConfigs(
			watch.NewConfig(gvks.ConfigMap, watch.Disabled()),
		),
	)

	owner := &corev1.Pod{}
	owner.SetName("owner")
	owner.SetNamespace("default")
	owner.SetUID("owner-uid")

	ctx := reconciler.WithControllerName(t.Context(), "test-controller")

	// Create owner in client
	err := cli.Create(ctx, owner)
	g.Expect(err).ToNot(HaveOccurred())

	cm := &corev1.ConfigMap{}
	cm.SetName("test")
	cm.SetNamespace("default")

	err = watcher.Watch(ctx, owner, cm)

	// Should return nil (no error) but not register watch
	g.Expect(err).ToNot(HaveOccurred())
	mockCtrl.AssertNotCalled(t, "Watch")

	state := watcher.State(gvks.ConfigMap)

	g.Expect(state).ToNot(BeNil())
	g.Expect(state.Config.Disabled).To(BeTrue())
	g.Expect(state.Watched).To(BeFalse())
}

func TestWatcher_NilObjectHandling(t *testing.T) {
	g := NewWithT(t)

	watcher, mockCtrl, cli, _ := setupTestWatcher(t, nil)

	owner := &corev1.Pod{}
	owner.SetName("owner")
	owner.SetNamespace("default")
	owner.SetUID("owner-uid")

	ctx := reconciler.WithControllerName(t.Context(), "test-controller")

	// Create owner in client
	err := cli.Create(ctx, owner)
	g.Expect(err).ToNot(HaveOccurred())

	// Pass nil object directly
	err = watcher.Watch(ctx, owner, nil)

	// Should not error, should skip nil object
	g.Expect(err).ToNot(HaveOccurred())
	mockCtrl.AssertNotCalled(t, "Watch")
}

func TestWatcher_CustomPredicates(t *testing.T) {
	g := NewWithT(t)

	customPred := predicate.NewPredicateFuncs(func(_ client.Object) bool {
		return true
	})

	watcher, mockCtrl, cli, _ := setupTestWatcher(t, nil,
		watch.WithConfigs(
			watch.NewConfig(gvks.ConfigMap, watch.WithPredicates(customPred)),
		),
	)

	owner := &corev1.Pod{}
	owner.SetName("owner")
	owner.SetNamespace("default")
	owner.SetUID("owner-uid")

	ctx := reconciler.WithControllerName(t.Context(), "test-controller")

	// Create owner in client
	err := cli.Create(ctx, owner)
	g.Expect(err).ToNot(HaveOccurred())

	cm := &corev1.ConfigMap{}
	cm.SetName("test")
	cm.SetNamespace("default")

	err = watcher.Watch(ctx, owner, cm)

	g.Expect(err).ToNot(HaveOccurred())
	mockCtrl.AssertNumberOfCalls(t, "Watch", 1)

	// Verify custom predicate was used
	state := watcher.State(gvks.ConfigMap)

	g.Expect(state.Config.Predicates).To(HaveLen(1))
}

func TestWatcher_CustomHandler(t *testing.T) {
	g := NewWithT(t)

	customHandler := handler.Funcs{}

	watcher, mockCtrl, cli, _ := setupTestWatcher(t, nil,
		watch.WithConfigs(
			watch.NewConfig(gvks.ConfigMap, watch.WithHandler(customHandler)),
		),
	)

	owner := &corev1.Pod{}
	owner.SetName("owner")
	owner.SetNamespace("default")
	owner.SetUID("owner-uid")

	ctx := reconciler.WithControllerName(t.Context(), "test-controller")

	// Create owner in client
	err := cli.Create(ctx, owner)
	g.Expect(err).ToNot(HaveOccurred())

	cm := &corev1.ConfigMap{}
	cm.SetName("test")
	cm.SetNamespace("default")

	err = watcher.Watch(ctx, owner, cm)

	g.Expect(err).ToNot(HaveOccurred())
	mockCtrl.AssertNumberOfCalls(t, "Watch", 1)

	// Verify custom handler was used
	state := watcher.State(gvks.ConfigMap)

	g.Expect(state.Config.Handler).ToNot(BeNil())
}

func TestWatcher_PartialMetadata(t *testing.T) {
	g := NewWithT(t)

	watcher, mockCtrl, cli, _ := setupTestWatcher(t, nil,
		watch.WithConfigs(
			watch.NewConfig(gvks.ConfigMap, watch.Partial()),
		),
	)

	owner := &corev1.Pod{}
	owner.SetName("owner")
	owner.SetNamespace("default")
	owner.SetUID("owner-uid")

	ctx := reconciler.WithControllerName(t.Context(), "test-controller")

	// Create owner in client
	err := cli.Create(ctx, owner)
	g.Expect(err).ToNot(HaveOccurred())

	cm := &corev1.ConfigMap{}
	cm.SetName("test")
	cm.SetNamespace("default")

	err = watcher.Watch(ctx, owner, cm)

	g.Expect(err).ToNot(HaveOccurred())
	mockCtrl.AssertNumberOfCalls(t, "Watch", 1)

	state := watcher.State(gvks.ConfigMap)

	g.Expect(state.Config.Partial).To(BeTrue())
}

func TestWatcher_ControllerNameInContext(t *testing.T) {
	g := NewWithT(t)

	watcher, mockCtrl, cli, _ := setupTestWatcher(t, nil)

	owner := &corev1.Pod{}
	owner.SetName("owner")
	owner.SetNamespace("default")
	owner.SetUID("owner-uid")

	err := cli.Create(t.Context(), owner)
	g.Expect(err).ToNot(HaveOccurred())

	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
	}
	cm.SetName("test")
	cm.SetNamespace("default")

	ctx := reconciler.WithControllerName(t.Context(), "my-controller")

	err = watcher.Watch(ctx, owner, cm)
	g.Expect(err).ToNot(HaveOccurred())
	mockCtrl.AssertNumberOfCalls(t, "Watch", 1)
}

func TestWatcher_DisabledConfigsSkipWatch(t *testing.T) {
	g := NewWithT(t)

	// Create watcher with disabled configs (external watches are now configs with Disabled: true)
	watcher, mockCtrl, cli, _ := setupTestWatcher(t, nil,
		watch.WithConfigs(
			watch.NewConfig(gvks.ConfigMap, watch.Disabled()),
			watch.NewConfig(gvks.Secret, watch.Disabled()),
		),
	)

	// Verify disabled configs are tracked
	cmState := watcher.State(gvks.ConfigMap)
	secretState := watcher.State(gvks.Secret)

	g.Expect(cmState).ToNot(BeNil())
	g.Expect(cmState.Config.Disabled).To(BeTrue(), "ConfigMap should be disabled")
	g.Expect(secretState).ToNot(BeNil())
	g.Expect(secretState.Config.Disabled).To(BeTrue(), "Secret should be disabled")

	// Attempting to watch these GVKs should be a no-op (disabled)
	owner := &corev1.Pod{}
	owner.SetName("owner")
	owner.SetNamespace("default")
	owner.SetUID("owner-uid")

	ctx := reconciler.WithControllerName(t.Context(), "test-controller")

	err := cli.Create(ctx, owner)
	g.Expect(err).ToNot(HaveOccurred())

	cm := &corev1.ConfigMap{}
	cm.SetName("test")
	cm.SetNamespace("default")

	err = watcher.Watch(ctx, owner, cm)
	g.Expect(err).ToNot(HaveOccurred())

	// No watch should have been registered (disabled)
	mockCtrl.AssertNotCalled(t, "Watch")
}

func TestWatcher_DisabledConfigPrecedence(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()

	mockCtrl := mocks.NewController()
	mockCacheObj := mocks.NewCache()

	// Create watcher with disabled config
	watcher := watch.New(
		watch.WithClient(cli),
		watch.WithController(mockCtrl),
		watch.WithCache(mockCacheObj),
		watch.WithConfigs(
			watch.NewConfig(gvks.ConfigMap, watch.Disabled()),
		),
	)

	// Verify the config is tracked with Disabled=true
	state := watcher.State(gvks.ConfigMap)

	g.Expect(state).ToNot(BeNil())
	g.Expect(state.Config.Disabled).To(BeTrue(), "Config should be disabled")
	g.Expect(state.Watched).To(BeFalse(), "Disabled config should have Watched=false")
}

func TestWatcher_CallSitePredicates(t *testing.T) {
	g := NewWithT(t)

	watcher, mockCtrl, cli, _ := setupTestWatcher(t, nil)

	owner := &corev1.Pod{}
	owner.SetName("owner")
	owner.SetNamespace("default")
	owner.SetUID("owner-uid")

	ctx := reconciler.WithControllerName(t.Context(), "test-controller")

	err := cli.Create(ctx, owner)
	g.Expect(err).ToNot(HaveOccurred())

	cm := &corev1.ConfigMap{}
	cm.SetName("test")
	cm.SetNamespace("default")

	customPred := predicate.NewPredicateFuncs(func(_ client.Object) bool {
		return true
	})

	err = watcher.Watch(ctx, owner, cm, watch.WithPredicates(customPred))

	g.Expect(err).ToNot(HaveOccurred())
	mockCtrl.AssertNumberOfCalls(t, "Watch", 1)

	state := watcher.State(gvks.ConfigMap)

	g.Expect(state).ToNot(BeNil())
	g.Expect(state.Config.Predicates).To(HaveLen(1))
}

func TestWatcher_CallSitePartial(t *testing.T) {
	g := NewWithT(t)

	watcher, mockCtrl, cli, _ := setupTestWatcher(t, nil)

	owner := &corev1.Pod{}
	owner.SetName("owner")
	owner.SetNamespace("default")
	owner.SetUID("owner-uid")

	ctx := reconciler.WithControllerName(t.Context(), "test-controller")

	err := cli.Create(ctx, owner)
	g.Expect(err).ToNot(HaveOccurred())

	cm := &corev1.ConfigMap{}
	cm.SetName("test")
	cm.SetNamespace("default")

	err = watcher.Watch(ctx, owner, cm, watch.Partial())

	g.Expect(err).ToNot(HaveOccurred())
	mockCtrl.AssertNumberOfCalls(t, "Watch", 1)

	state := watcher.State(gvks.ConfigMap)

	g.Expect(state).ToNot(BeNil())
	g.Expect(state.Config.Partial).To(BeTrue())
}

func TestWatcher_CallSiteDisabled(t *testing.T) {
	g := NewWithT(t)

	watcher, mockCtrl, cli, _ := setupTestWatcher(t, nil)

	owner := &corev1.Pod{}
	owner.SetName("owner")
	owner.SetNamespace("default")
	owner.SetUID("owner-uid")

	ctx := reconciler.WithControllerName(t.Context(), "test-controller")

	err := cli.Create(ctx, owner)
	g.Expect(err).ToNot(HaveOccurred())

	cm := &corev1.ConfigMap{}
	cm.SetName("test")
	cm.SetNamespace("default")

	err = watcher.Watch(ctx, owner, cm, watch.Disabled())

	g.Expect(err).ToNot(HaveOccurred())
	mockCtrl.AssertNotCalled(t, "Watch")

	state := watcher.State(gvks.ConfigMap)

	g.Expect(state).ToNot(BeNil())
	g.Expect(state.Config.Disabled).To(BeTrue())
	g.Expect(state.Watched).To(BeFalse())
}

func TestWatcher_CallSiteOptsOverridePreConfigured(t *testing.T) {
	g := NewWithT(t)

	preConfigPred := predicate.NewPredicateFuncs(func(_ client.Object) bool {
		return false
	})

	watcher, mockCtrl, cli, _ := setupTestWatcher(t, nil,
		watch.WithConfigs(
			watch.NewConfig(gvks.ConfigMap, watch.WithPredicates(preConfigPred)),
		),
	)

	owner := &corev1.Pod{}
	owner.SetName("owner")
	owner.SetNamespace("default")
	owner.SetUID("owner-uid")

	ctx := reconciler.WithControllerName(t.Context(), "test-controller")

	err := cli.Create(ctx, owner)
	g.Expect(err).ToNot(HaveOccurred())

	cm := &corev1.ConfigMap{}
	cm.SetName("test")
	cm.SetNamespace("default")

	callSitePred := predicate.NewPredicateFuncs(func(_ client.Object) bool {
		return true
	})

	err = watcher.Watch(ctx, owner, cm, watch.WithPredicates(callSitePred))

	g.Expect(err).ToNot(HaveOccurred())
	mockCtrl.AssertNumberOfCalls(t, "Watch", 1)

	// Both predicates should be present (append behavior)
	state := watcher.State(gvks.ConfigMap)

	g.Expect(state).ToNot(BeNil())
	g.Expect(state.Config.Predicates).To(HaveLen(2))
}

func TestAll_BatchWatch(t *testing.T) {
	g := NewWithT(t)

	watcher, mockCtrl, cli, _ := setupTestWatcher(t, nil)

	owner := &corev1.Pod{}
	owner.SetName("owner")
	owner.SetNamespace("default")
	owner.SetUID("owner-uid")

	ctx := reconciler.WithControllerName(t.Context(), "test-controller")

	err := cli.Create(ctx, owner)
	g.Expect(err).ToNot(HaveOccurred())

	cm := &corev1.ConfigMap{}
	cm.SetName("cm1")
	cm.SetNamespace("default")

	secret := &corev1.Secret{}
	secret.SetName("secret")
	secret.SetNamespace("default")

	hook := watch.All(watcher)
	err = hook(ctx, owner, []client.Object{cm, secret})

	g.Expect(err).ToNot(HaveOccurred())
	mockCtrl.AssertNumberOfCalls(t, "Watch", 2)

	cmState := watcher.State(gvks.ConfigMap)
	secretState := watcher.State(gvks.Secret)

	g.Expect(cmState.Watched).To(BeTrue())
	g.Expect(secretState.Watched).To(BeTrue())
}

func TestAll_NilObjectHandling(t *testing.T) {
	g := NewWithT(t)

	watcher, mockCtrl, cli, _ := setupTestWatcher(t, nil)

	owner := &corev1.Pod{}
	owner.SetName("owner")
	owner.SetNamespace("default")
	owner.SetUID("owner-uid")

	ctx := reconciler.WithControllerName(t.Context(), "test-controller")

	err := cli.Create(ctx, owner)
	g.Expect(err).ToNot(HaveOccurred())

	cm := &corev1.ConfigMap{}
	cm.SetName("test")
	cm.SetNamespace("default")

	hook := watch.All(watcher)
	err = hook(ctx, owner, []client.Object{nil, cm, nil})

	g.Expect(err).ToNot(HaveOccurred())
	mockCtrl.AssertNumberOfCalls(t, "Watch", 1)
}

func TestAll_EmptySlice(t *testing.T) {
	g := NewWithT(t)

	watcher, mockCtrl, cli, _ := setupTestWatcher(t, nil)

	owner := &corev1.Pod{}
	owner.SetName("owner")
	owner.SetNamespace("default")
	owner.SetUID("owner-uid")

	ctx := reconciler.WithControllerName(t.Context(), "test-controller")

	err := cli.Create(ctx, owner)
	g.Expect(err).ToNot(HaveOccurred())

	hook := watch.All(watcher)
	err = hook(ctx, owner, []client.Object{})

	g.Expect(err).ToNot(HaveOccurred())
	mockCtrl.AssertNotCalled(t, "Watch")
}

func TestAll_MixedGVKsInSingleCall(t *testing.T) {
	g := NewWithT(t)

	watcher, mockCtrl, cli, _ := setupTestWatcher(t, nil)

	owner := &corev1.Pod{}
	owner.SetName("owner")
	owner.SetNamespace("default")
	owner.SetUID("owner-uid")

	ctx := reconciler.WithControllerName(t.Context(), "test-controller")

	err := cli.Create(ctx, owner)
	g.Expect(err).ToNot(HaveOccurred())

	cm1 := &corev1.ConfigMap{}
	cm1.SetName("cm1")
	cm1.SetNamespace("default")

	cm2 := &corev1.ConfigMap{}
	cm2.SetName("cm2")
	cm2.SetNamespace("default")

	secret := &corev1.Secret{}
	secret.SetName("secret")
	secret.SetNamespace("default")

	hook := watch.All(watcher)
	err = hook(ctx, owner, []client.Object{cm1, cm2, secret})

	g.Expect(err).ToNot(HaveOccurred())

	// Should register watches for 2 unique GVKs (ConfigMap counted once)
	mockCtrl.AssertNumberOfCalls(t, "Watch", 2)

	cmState := watcher.State(gvks.ConfigMap)
	secretState := watcher.State(gvks.Secret)

	g.Expect(cmState.Watched).To(BeTrue())
	g.Expect(secretState.Watched).To(BeTrue())
}
