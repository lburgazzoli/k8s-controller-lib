//nolint:testpackage // White-box testing: tests internal TypedPipeline initialization
package pipeline

import (
	"context"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/reconciler"
	"github.com/lburgazzoli/k8s-controller-lib/pkg/util/test/mocks"

	. "github.com/onsi/gomega"
)

// newTestClient creates a fake client with TestResource registered.
func newTestClient() client.Client {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	schemeBuilder := runtime.NewSchemeBuilder(func(s *runtime.Scheme) error {
		s.AddKnownTypes(schema.GroupVersion{Group: "test.example.com", Version: "v1"}, &TestResource{})

		return nil
	})
	_ = schemeBuilder.AddToScheme(scheme)

	return fake.NewClientBuilder().WithScheme(scheme).Build()
}

func TestNewTyped_CreatesTypedPipeline(t *testing.T) {
	g := NewWithT(t)

	c := newTestClient()
	p := NewTyped[*TestResource](c, WithFieldOwner("test-controller"))

	g.Expect(p).ToNot(BeNil())
	g.Expect(p.client).To(Equal(c))
	g.Expect(p.opts).To(HaveLen(1))
	// Without DeferredAutoWatch, pipeline initializes immediately
	g.Expect(p.pipeline).ToNot(BeNil())
}

func TestNewTyped_WithDeferredAutoWatch_PipelineIsNil(t *testing.T) {
	g := NewWithT(t)

	c := newTestClient()
	p := NewTyped[*TestResource](c,
		WithFieldOwner("test-controller"),
		DeferredAutoWatch(),
	)

	g.Expect(p).ToNot(BeNil())
	g.Expect(p.client).To(Equal(c))
	// With DeferredAutoWatch, pipeline is nil until controller and cache are set
	g.Expect(p.pipeline).To(BeNil())
}

func TestTypedPipeline_InitializesWithoutDeferredAutoWatch(t *testing.T) {
	g := NewWithT(t)

	c := newTestClient()
	p := NewTyped[*TestResource](
		c,
		WithFieldOwner("test-controller"),
	)

	// Without DeferredAutoWatch, pipeline should initialize immediately
	// when any of the aware methods is called
	g.Expect(p.GetPipeline()).ToNot(BeNil())
}

func TestTypedPipeline_InitializesAfterControllerAndCacheSet(t *testing.T) {
	g := NewWithT(t)

	c := newTestClient()
	p := NewTyped[*TestResource](
		c,
		WithFieldOwner("test-controller"),
		DeferredAutoWatch(),
	)

	// Pipeline should not be initialized yet (waiting for controller and cache)
	g.Expect(p.GetPipeline()).To(BeNil())

	// Set controller - still needs cache
	mockCtrl := mocks.NewController()
	p.SetController(mockCtrl)
	g.Expect(p.GetPipeline()).To(BeNil())

	// Set cache - now it should initialize
	mockCache := &fakeCache{}
	p.SetCache(mockCache)
	g.Expect(p.GetPipeline()).ToNot(BeNil())
}

func TestTypedPipeline_InitializesInReverseOrder(t *testing.T) {
	g := NewWithT(t)

	c := newTestClient()
	p := NewTyped[*TestResource](
		c,
		WithFieldOwner("test-controller"),
		DeferredAutoWatch(),
	)

	// Set cache first - should not initialize (needs controller)
	mockCache := &fakeCache{}
	p.SetCache(mockCache)
	g.Expect(p.GetPipeline()).To(BeNil())

	// Set controller - now it should initialize
	mockCtrl := mocks.NewController()
	p.SetController(mockCtrl)
	g.Expect(p.GetPipeline()).ToNot(BeNil())
}

func TestTypedPipeline_Reconcile_ReturnsErrorWhenNotInitialized(t *testing.T) {
	g := NewWithT(t)

	c := newTestClient()
	p := NewTyped[*TestResource](
		c,
		WithFieldOwner("test-controller"),
		DeferredAutoWatch(),
	)

	// Pipeline is not initialized - Reconcile should return error
	resource := &TestResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
	}

	req := &reconciler.TypedRequest[*TestResource]{
		Client: c,
		Object: resource,
	}

	resp, err := p.Reconcile(t.Context(), req)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("pipeline not initialized"))
	g.Expect(resp).To(BeNil())
}

func TestTypedPipeline_Reconcile_Success(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	schemeBuilder := runtime.NewSchemeBuilder(func(s *runtime.Scheme) error {
		s.AddKnownTypes(schema.GroupVersion{Group: "test.example.com", Version: "v1"}, &TestResource{})

		return nil
	})
	_ = schemeBuilder.AddToScheme(scheme)

	resource := &TestResource{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "test.example.com/v1",
			Kind:       "TestResource",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(resource).
		WithStatusSubresource(resource).
		Build()

	var actionExecuted bool
	action := func(_ context.Context, _ *reconciler.Request, _ *reconciler.Response) error {
		actionExecuted = true

		return nil
	}

	p := NewTyped[*TestResource](
		c,
		WithFieldOwner("test-controller"),
		WithActions(action),
	)

	req := &reconciler.TypedRequest[*TestResource]{
		Client: c,
		Object: resource,
	}

	// Add controller name to context (required by Pipeline)
	ctx := reconciler.WithControllerName(t.Context(), "test-controller")

	resp, err := p.Reconcile(ctx, req)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(resp).ToNot(BeNil())
	g.Expect(actionExecuted).To(BeTrue())
}

func TestTypedPipeline_SetClient_OverridesInitialClient(t *testing.T) {
	g := NewWithT(t)

	initialClient := newTestClient()
	p := NewTyped[*TestResource](
		initialClient,
		WithFieldOwner("test-controller"),
	)

	// Verify initial client is set
	g.Expect(p.client).To(Equal(initialClient))

	// Override client
	newClient := newTestClient()
	p.SetClient(newClient)

	g.Expect(p.client).To(Equal(newClient))
}

func TestTypedPipeline_ImplementsAwareInterfaces(t *testing.T) {
	g := NewWithT(t)

	c := newTestClient()
	p := NewTyped[*TestResource](c)

	// Verify TypedPipeline implements the aware interfaces
	var _ reconciler.ControllerAware = p
	var _ reconciler.ClientAware = p
	var _ reconciler.CacheAware = p
	var _ reconciler.ExternalWatchesAware = p

	// Also verify it implements TypedReconciler
	var _ reconciler.TypedReconciler[*TestResource] = p

	g.Expect(p).ToNot(BeNil())
}

func TestTypedPipeline_SetExternalWatches_StoresGVKs(t *testing.T) {
	g := NewWithT(t)

	c := newTestClient()
	p := NewTyped[*TestResource](
		c,
		WithFieldOwner("test-controller"),
		DeferredAutoWatch(),
	)

	// Set external watches
	externalGVKs := []schema.GroupVersionKind{
		{Group: "", Version: "v1", Kind: "ConfigMap"},
		{Group: "", Version: "v1", Kind: "Secret"},
	}
	p.SetExternalWatches(externalGVKs)

	// Verify external watches are stored
	g.Expect(p.externalWatches).To(HaveLen(2))
	g.Expect(p.externalWatches).To(ConsistOf(externalGVKs))
}

func TestTypedPipeline_SetExternalWatches_TriggersInitialization(t *testing.T) {
	g := NewWithT(t)

	c := newTestClient()
	p := NewTyped[*TestResource](
		c,
		WithFieldOwner("test-controller"),
		DeferredAutoWatch(),
	)

	// Set controller and cache first
	mockCtrl := mocks.NewController()
	mockCache := &fakeCache{}
	p.SetController(mockCtrl)
	p.SetCache(mockCache)

	// Pipeline should be initialized now
	g.Expect(p.GetPipeline()).ToNot(BeNil())

	// Setting external watches after initialization should still store them
	externalGVKs := []schema.GroupVersionKind{
		{Group: "", Version: "v1", Kind: "ConfigMap"},
	}
	p.SetExternalWatches(externalGVKs)

	g.Expect(p.externalWatches).To(HaveLen(1))
}

func TestDeferredAutoWatch_IsMarkerOption(t *testing.T) {
	g := NewWithT(t)

	opt := DeferredAutoWatch()
	g.Expect(opt).ToNot(BeNil())

	// DeferredAutoWatch should not modify Options directly
	// (it's a marker option processed by TypedPipeline)
	opts := &Options{}
	opt.ApplyTo(opts)

	// AutoWatch should not be set (it's deferred)
	g.Expect(opts.AutoWatch).To(BeNil())
}

func TestDeferredAutoWatch_WithConfigs(t *testing.T) {
	g := NewWithT(t)

	c := newTestClient()
	mockCtrl := mocks.NewController()
	mockCache := &fakeCache{}

	// Create TypedPipeline with DeferredAutoWatch that has watch configs
	// Note: We can't fully test auto-watch without a real controller, but we can
	// verify the configs are stored and would be passed to WithAutoWatch
	p := NewTyped[*TestResource](
		c,
		WithFieldOwner("test-controller"),
		DeferredAutoWatch(), // With no configs for now
	)

	p.SetController(mockCtrl)
	p.SetCache(mockCache)

	g.Expect(p.GetPipeline()).ToNot(BeNil())
}

func TestTypedPipeline_ConcurrentAccess(t *testing.T) {
	g := NewWithT(t)

	c := newTestClient()
	p := NewTyped[*TestResource](
		c,
		WithFieldOwner("test-controller"),
		DeferredAutoWatch(),
	)

	mockCtrl := mocks.NewController()
	mockCache := &fakeCache{}

	// Set controller and cache concurrently
	done := make(chan struct{})

	go func() {
		p.SetController(mockCtrl)
		close(done)
	}()

	go func() {
		p.SetCache(mockCache)
	}()

	<-done

	// Wait a bit for the other goroutine
	// Eventually the pipeline should be initialized
	g.Eventually(func() *Pipeline {
		return p.GetPipeline()
	}).ShouldNot(BeNil())
}

// fakeCache implements cache.Cache for testing.
type fakeCache struct {
	cache.Cache
}

// Verify mocks.Controller implements controller.Controller at compile time.
var _ controller.Controller = (*mocks.Controller)(nil)
