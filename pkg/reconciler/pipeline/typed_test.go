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

	p := NewTyped[*TestResource](WithFieldOwner("test-controller"))

	g.Expect(p).ToNot(BeNil())
	g.Expect(p.Pipeline).ToNot(BeNil())
	g.Expect(p.Pipeline.opts.FieldOwner).To(Equal("test-controller"))
}

func TestNewTyped_WithAutoWatch(t *testing.T) {
	g := NewWithT(t)

	p := NewTyped[*TestResource](
		WithFieldOwner("test-controller"),
		WithAutoWatch(),
	)

	g.Expect(p).ToNot(BeNil())
	g.Expect(p.Pipeline.opts.AutoWatch).To(BeTrue())
}

func TestTypedPipeline_SetClient(t *testing.T) {
	g := NewWithT(t)

	p := NewTyped[*TestResource](WithFieldOwner("test-controller"))

	// Pipeline should not have a client yet
	g.Expect(p.Pipeline.opts.Client).To(BeNil())

	// Set client via aware interface
	c := newTestClient()
	p.SetClient(c)

	g.Expect(p.Pipeline.opts.Client).To(Equal(c))
}

func TestTypedPipeline_SetCache(t *testing.T) {
	g := NewWithT(t)

	p := NewTyped[*TestResource](WithFieldOwner("test-controller"))

	// Pipeline should not have a cache yet
	g.Expect(p.Pipeline.opts.Cache).To(BeNil())

	// Set cache via aware interface
	mockCache := &fakeCache{}
	p.SetCache(mockCache)

	g.Expect(p.Pipeline.opts.Cache).To(Equal(mockCache))
}

func TestTypedPipeline_SetController(t *testing.T) {
	g := NewWithT(t)

	p := NewTyped[*TestResource](WithFieldOwner("test-controller"))

	// Pipeline should not have a controller yet
	g.Expect(p.Pipeline.opts.Controller).To(BeNil())

	// Set controller via aware interface
	mockCtrl := mocks.NewController()
	p.SetController(mockCtrl)

	g.Expect(p.Pipeline.opts.Controller).To(Equal(mockCtrl))
}

func TestTypedPipeline_SetExternalWatches(t *testing.T) {
	g := NewWithT(t)

	p := NewTyped[*TestResource](
		WithFieldOwner("test-controller"),
		WithAutoWatch(),
	)

	// Set external watches - these should be added as disabled configs
	externalGVKs := []schema.GroupVersionKind{
		{Group: "", Version: "v1", Kind: "ConfigMap"},
		{Group: "", Version: "v1", Kind: "Secret"},
	}
	p.SetExternalWatches(externalGVKs)

	// Verify the GVKs were added as disabled configs
	g.Expect(p.Pipeline.opts.WatchConfigs).To(HaveLen(2))
	g.Expect(p.Pipeline.opts.WatchConfigs[0].GVK.Kind).To(Equal("ConfigMap"))
	g.Expect(p.Pipeline.opts.WatchConfigs[0].Disabled).To(BeTrue())
	g.Expect(p.Pipeline.opts.WatchConfigs[1].GVK.Kind).To(Equal("Secret"))
	g.Expect(p.Pipeline.opts.WatchConfigs[1].Disabled).To(BeTrue())
}

func TestTypedPipeline_Reconcile_PanicsWhenClientNotSet(t *testing.T) {
	g := NewWithT(t)

	p := NewTyped[*TestResource](WithFieldOwner("test-controller"))

	resource := &TestResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
	}

	req := &reconciler.TypedRequest[*TestResource]{
		Client: nil, // Not used by Pipeline, it uses its own client
		Object: resource,
	}

	// Reconcile should panic because client is not set
	g.Expect(func() {
		_, _ = p.Reconcile(t.Context(), req)
	}).To(Panic())
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
		WithClient(c),
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

func TestTypedPipeline_ImplementsAwareInterfaces(t *testing.T) {
	g := NewWithT(t)

	p := NewTyped[*TestResource]()

	// Verify TypedPipeline implements the aware interfaces (inherited from Pipeline)
	var _ reconciler.ControllerAware = p
	var _ reconciler.ClientAware = p
	var _ reconciler.CacheAware = p
	var _ reconciler.ExternalWatchesAware = p

	// Also verify it implements TypedReconciler
	var _ reconciler.TypedReconciler[*TestResource] = p

	g.Expect(p).ToNot(BeNil())
}

func TestTypedPipeline_LazyWatcherCreation(t *testing.T) {
	g := NewWithT(t)

	c := newTestClient()
	mockCtrl := mocks.NewController()
	mockCache := &fakeCache{}

	// Create pipeline with auto-watch enabled
	p := NewTyped[*TestResource](
		WithFieldOwner("test-controller"),
		WithAutoWatch(),
	)

	// Watcher should not exist yet (missing dependencies)
	g.Expect(p.Pipeline.watcher).To(BeNil())

	// Set dependencies
	p.SetClient(c)
	p.SetController(mockCtrl)
	p.SetCache(mockCache)

	// Watcher is still nil until getWatcher() is called (lazy creation)
	g.Expect(p.Pipeline.watcher).To(BeNil())

	// Calling getWatcher() should create it
	watcher := p.getWatcher()
	g.Expect(watcher).ToNot(BeNil())
	g.Expect(p.Pipeline.watcher).ToNot(BeNil())
}

func TestTypedPipeline_ConcurrentAccess(t *testing.T) {
	g := NewWithT(t)

	p := NewTyped[*TestResource](
		WithFieldOwner("test-controller"),
		WithAutoWatch(),
	)

	c := newTestClient()
	mockCtrl := mocks.NewController()
	mockCache := &fakeCache{}

	// Set dependencies concurrently
	done := make(chan struct{})

	go func() {
		p.SetClient(c)
		close(done)
	}()

	go func() {
		p.SetController(mockCtrl)
	}()

	go func() {
		p.SetCache(mockCache)
	}()

	<-done

	// All dependencies should eventually be set
	g.Eventually(func() bool {
		return p.Pipeline.opts.Client != nil &&
			p.Pipeline.opts.Controller != nil &&
			p.Pipeline.opts.Cache != nil
	}).Should(BeTrue())
}

// fakeCache implements cache.Cache for testing.
type fakeCache struct {
	cache.Cache
}

// Verify mocks.Controller implements controller.Controller at compile time.
var _ controller.Controller = (*mocks.Controller)(nil)
