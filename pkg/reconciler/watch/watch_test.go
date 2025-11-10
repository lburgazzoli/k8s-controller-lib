package watch_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/reconciler"
	"github.com/lburgazzoli/k8s-controller-lib/pkg/reconciler/watch"
	"github.com/lburgazzoli/k8s-controller-lib/pkg/resources/gvks"

	. "github.com/onsi/gomega"
)

// mockController implements controller.Controller for testing.
type mockController struct {
	mu           sync.Mutex
	watchCalls   int
	watchError   error
	watchedGVKs  map[schema.GroupVersionKind]int
	watchSources []source.Source
}

func newMockController() *mockController {
	return &mockController{
		watchedGVKs:  make(map[schema.GroupVersionKind]int),
		watchSources: []source.Source{},
	}
}

func (m *mockController) Reconcile(_ context.Context, _ reconcile.Request) (reconcile.Result, error) {
	return reconcile.Result{}, nil
}

func (m *mockController) Watch(src source.Source) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.watchCalls++
	m.watchSources = append(m.watchSources, src)

	return m.watchError
}

func (m *mockController) Start(_ context.Context) error {
	return nil
}

func (m *mockController) GetLogger() logr.Logger {
	return ctrl.Log
}

func (m *mockController) NeedLeaderElection() bool {
	return false
}

// mockCache implements cache.Cache for testing.
type mockCache struct {
	cache.Cache
}

func newMockCache() *mockCache {
	return &mockCache{}
}

func (m *mockCache) Get(_ context.Context, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
	return nil
}

// setupTestWatcher creates a watcher with mock dependencies for testing.
func setupTestWatcher(
	t *testing.T,
	cli client.Client,
	opts ...watch.Option,
) (*watch.Watcher, *mockController, client.Client, *runtime.Scheme) {
	t.Helper()

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	if cli == nil {
		cli = fake.NewClientBuilder().WithScheme(scheme).Build()
	}

	mockCtrl := newMockController()
	mockCacheObj := newMockCache()

	watcher := watch.New(mockCtrl, mockCacheObj, cli, opts...)

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
		wg.Add(1)
		go func() {
			defer wg.Done()

			cm := &corev1.ConfigMap{}
			cm.SetName("test")
			cm.SetNamespace("default")

			err := watcher.Watch(ctx, owner, []client.Object{cm})
			g.Expect(err).ToNot(HaveOccurred())
		}()
	}

	wg.Wait()

	// Verify watch was registered exactly once despite concurrent calls
	g.Expect(mockCtrl.watchCalls).To(Equal(1), "controller.Watch should be called exactly once")

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
	wg.Add(1)
	go func() {
		defer wg.Done()

		cm := &corev1.ConfigMap{}
		cm.SetName("test-cm")
		cm.SetNamespace("default")

		err := watcher.Watch(ctx, owner, []client.Object{cm})
		g.Expect(err).ToNot(HaveOccurred())
	}()

	// Goroutine 2: Watch Secret
	wg.Add(1)
	go func() {
		defer wg.Done()

		secret := &corev1.Secret{}
		secret.SetName("test-secret")
		secret.SetNamespace("default")

		err := watcher.Watch(ctx, owner, []client.Object{secret})
		g.Expect(err).ToNot(HaveOccurred())
	}()

	wg.Wait()

	// Both watches should succeed
	g.Expect(mockCtrl.watchCalls).To(Equal(2), "both GVKs should be watched")

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
		err = watcher.Watch(ctx, owner, []client.Object{cm})
		if err != nil {
			t.Fatalf("Watch() iteration %d failed: %v", i, err)
		}
		t.Logf("Iteration %d: watchCalls=%d", i, mockCtrl.watchCalls)
	}

	// Verify watch was only registered once
	g.Expect(mockCtrl.watchCalls).To(Equal(1), "watch should be idempotent")

	state := watcher.State(gvks.ConfigMap)

	if state == nil {
		t.Fatalf("State is nil for ConfigMap GVK=%v - watch failed silently", gvks.ConfigMap)
	}
	g.Expect(state.Watched).To(BeTrue())
}

func TestWatcher_SetupWatchError(t *testing.T) {
	g := NewWithT(t)

	watcher, mockCtrl, cli, _ := setupTestWatcher(t, nil)

	// Configure mock to return error
	mockCtrl.watchError = errors.New("watch registration failed")

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

	err = watcher.Watch(ctx, owner, []client.Object{cm})

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
			watch.For(gvks.ConfigMap, watch.Disabled()),
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

	err = watcher.Watch(ctx, owner, []client.Object{cm})

	// Should return nil (no error) but not register watch
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(mockCtrl.watchCalls).To(Equal(0), "disabled watch should not call controller.Watch")

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

	// Pass slice with nil objects
	err = watcher.Watch(ctx, owner, []client.Object{nil, nil})

	// Should not error, should skip nil objects
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(mockCtrl.watchCalls).To(Equal(0), "nil objects should be skipped")
}

func TestWatcher_EmptyObjectsSlice(t *testing.T) {
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

	// Pass empty slice
	err = watcher.Watch(ctx, owner, []client.Object{})

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(mockCtrl.watchCalls).To(Equal(0))
}

func TestWatcher_CustomPredicates(t *testing.T) {
	g := NewWithT(t)

	customPred := predicate.NewPredicateFuncs(func(_ client.Object) bool {
		return true
	})

	watcher, mockCtrl, cli, _ := setupTestWatcher(t, nil,
		watch.WithConfigs(
			watch.For(gvks.ConfigMap, watch.WithPredicates(customPred)),
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

	err = watcher.Watch(ctx, owner, []client.Object{cm})

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(mockCtrl.watchCalls).To(Equal(1))

	// Verify custom predicate was used
	state := watcher.State(gvks.ConfigMap)

	g.Expect(state.Config.Predicates).To(HaveLen(1))
}

func TestWatcher_CustomHandler(t *testing.T) {
	g := NewWithT(t)

	customHandler := handler.Funcs{}

	watcher, mockCtrl, cli, _ := setupTestWatcher(t, nil,
		watch.WithConfigs(
			watch.For(gvks.ConfigMap, watch.WithHandler(customHandler)),
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

	err = watcher.Watch(ctx, owner, []client.Object{cm})

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(mockCtrl.watchCalls).To(Equal(1))

	// Verify custom handler was used
	state := watcher.State(gvks.ConfigMap)

	g.Expect(state.Config.Handler).ToNot(BeNil())
}

func TestWatcher_MixedGVKsInSingleCall(t *testing.T) {
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

	cm1 := &corev1.ConfigMap{}
	cm1.SetName("cm1")
	cm1.SetNamespace("default")

	cm2 := &corev1.ConfigMap{}
	cm2.SetName("cm2")
	cm2.SetNamespace("default")

	secret := &corev1.Secret{}
	secret.SetName("secret")
	secret.SetNamespace("default")

	// Watch multiple objects of different GVKs in single call
	err = watcher.Watch(ctx, owner, []client.Object{cm1, cm2, secret})

	g.Expect(err).ToNot(HaveOccurred())

	// Should register watches for 2 unique GVKs (ConfigMap counted once)
	g.Expect(mockCtrl.watchCalls).To(Equal(2), "should watch 2 unique GVKs")

	cmState := watcher.State(gvks.ConfigMap)
	secretState := watcher.State(gvks.Secret)

	g.Expect(cmState.Watched).To(BeTrue())
	g.Expect(secretState.Watched).To(BeTrue())
}

func TestWatcher_PartialMetadata(t *testing.T) {
	g := NewWithT(t)

	watcher, mockCtrl, cli, _ := setupTestWatcher(t, nil,
		watch.WithConfigs(
			watch.For(gvks.ConfigMap, watch.Partial()),
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

	err = watcher.Watch(ctx, owner, []client.Object{cm})

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(mockCtrl.watchCalls).To(Equal(1))

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

	err = watcher.Watch(ctx, owner, []client.Object{cm})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(mockCtrl.watchCalls).To(Equal(1))
}
