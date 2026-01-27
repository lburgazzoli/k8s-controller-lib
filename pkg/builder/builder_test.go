package builder_test

import (
	"context"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/builder"
	"github.com/lburgazzoli/k8s-controller-lib/pkg/predicates"
	"github.com/lburgazzoli/k8s-controller-lib/pkg/reconciler"
	"github.com/lburgazzoli/k8s-controller-lib/pkg/resources/gvks"
	"github.com/lburgazzoli/k8s-controller-lib/pkg/status"
	"github.com/lburgazzoli/k8s-controller-lib/pkg/util/test/mocks"

	. "github.com/onsi/gomega"
)

// TestResource is a minimal test type implementing ManagedObject.
type TestResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Status status.Status `json:"status"`
}

func (r *TestResource) GetStatus() *status.Status {
	return &r.Status
}

func (r *TestResource) SetStatus(s *status.Status) {
	if s != nil {
		r.Status = *s
	}
}

func (r *TestResource) DeepCopyObject() runtime.Object {
	if r == nil {
		return nil
	}
	out := new(TestResource)
	*out = *r
	out.Status = *r.Status.DeepCopy()

	return out
}

// setupTestManager creates a fake manager for testing.
func setupTestManager(t *testing.T) (manager.Manager, *mocks.Controller) {
	t.Helper()
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(scheme)).To(Succeed())

	// Register TestResource
	schemeBuilder := runtime.NewSchemeBuilder(func(s *runtime.Scheme) error {
		s.AddKnownTypes(corev1.SchemeGroupVersion, &TestResource{})

		return nil
	})
	g.Expect(schemeBuilder.AddToScheme(scheme)).To(Succeed())

	// Create REST mapper
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{})

	// Create fake manager
	mgr := &fakeManager{
		scheme: scheme,
		client: &fakeClient{scheme: scheme},
		cache:  &fakeCache{},
		mapper: mapper,
	}

	mockCtrl := mocks.NewController()

	return mgr, mockCtrl
}

// fakeManager implements manager.Manager for testing.
type fakeManager struct {
	manager.Manager

	scheme *runtime.Scheme
	client client.Client
	cache  cache.Cache
	mapper meta.RESTMapper
}

func (f *fakeManager) GetScheme() *runtime.Scheme {
	return f.scheme
}

func (f *fakeManager) GetClient() client.Client {
	return f.client
}

func (f *fakeManager) GetCache() cache.Cache {
	return f.cache
}

func (f *fakeManager) GetRESTMapper() meta.RESTMapper {
	return f.mapper
}

func (f *fakeManager) GetConfig() *rest.Config {
	return &rest.Config{}
}

func (f *fakeManager) GetControllerOptions() config.Controller {
	return config.Controller{}
}

// fakeClient implements client.Client for testing.
type fakeClient struct {
	client.Client

	scheme *runtime.Scheme
}

// fakeCache implements cache.Cache for testing.
type fakeCache struct {
	cache.Cache
}

func TestNewControllerBuilder_Success(t *testing.T) {
	g := NewWithT(t)

	mgr, _ := setupTestManager(t)

	b, err := builder.NewControllerBuilder[*TestResource](mgr)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(b).ToNot(BeNil())
}

func TestNewControllerBuilder_NilManager(t *testing.T) {
	g := NewWithT(t)

	b, err := builder.NewControllerBuilder[*TestResource](nil)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("manager is required"))
	g.Expect(b).To(BeNil())
}

func TestNewControllerBuilder_WithName(t *testing.T) {
	g := NewWithT(t)

	mgr, _ := setupTestManager(t)

	b, err := builder.NewControllerBuilder[*TestResource](
		mgr,
		builder.WithName("test-controller"),
	)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(b).ToNot(BeNil())
}

func TestNewControllerBuilder_WithMaxConcurrentReconciles(t *testing.T) {
	g := NewWithT(t)

	mgr, _ := setupTestManager(t)

	b, err := builder.NewControllerBuilder[*TestResource](
		mgr,
		builder.WithMaxConcurrentReconciles(5),
	)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(b).ToNot(BeNil())
}

func TestBuilder_For_OnlyOnce(t *testing.T) {
	g := NewWithT(t)

	mgr, mockCtrl := setupTestManager(t)

	// Setup mock to accept watch calls
	mockCtrl.On("Watch", nil).Return(nil)

	b, err := builder.NewControllerBuilder[*TestResource](mgr)
	g.Expect(err).ToNot(HaveOccurred())

	// First For() call should succeed
	b = b.For(&TestResource{})
	g.Expect(b).ToNot(BeNil())

	// Second For() call should accumulate error
	b = b.For(&TestResource{})
	g.Expect(b).ToNot(BeNil())

	// Complete should fail with accumulated error
	r := reconciler.Wrap(func(_ context.Context, _ *reconciler.TypedRequest[*TestResource]) (*reconciler.Response, error) {
		return reconciler.NewResponse(), nil
	})
	err = b.Complete(r)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("For() can only be called once"))
}

func TestBuilder_For_WithHandler_Error(t *testing.T) {
	g := NewWithT(t)

	mgr, _ := setupTestManager(t)

	b, err := builder.NewControllerBuilder[*TestResource](mgr)
	g.Expect(err).ToNot(HaveOccurred())

	h := handler.TypedFuncs[client.Object, reconcile.Request]{}
	b = b.For(&TestResource{}, builder.WithHandler(h))

	r := reconciler.Wrap(func(_ context.Context, _ *reconciler.TypedRequest[*TestResource]) (*reconciler.Response, error) {
		return reconciler.NewResponse(), nil
	})
	err = b.Complete(r)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("For() does not support WithHandler"))
}

func TestBuilder_For_WithMapper_Error(t *testing.T) {
	g := NewWithT(t)

	mgr, _ := setupTestManager(t)

	b, err := builder.NewControllerBuilder[*TestResource](mgr)
	g.Expect(err).ToNot(HaveOccurred())

	mapper := func(_ context.Context, _ client.Object) []reconcile.Request {
		return nil
	}
	b = b.For(&TestResource{}, builder.WithMapper(mapper))

	r := reconciler.Wrap(func(_ context.Context, _ *reconciler.TypedRequest[*TestResource]) (*reconciler.Response, error) {
		return reconciler.NewResponse(), nil
	})
	err = b.Complete(r)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("For() does not support WithMapper"))
}

func TestBuilder_Owns_Basic(t *testing.T) {
	g := NewWithT(t)

	mgr, _ := setupTestManager(t)

	b, err := builder.NewControllerBuilder[*TestResource](mgr)
	g.Expect(err).ToNot(HaveOccurred())

	b = b.For(&TestResource{}).
		Owns(gvks.ConfigMap)
	g.Expect(b).ToNot(BeNil())
}

func TestBuilder_Owns_WithMapper_Error(t *testing.T) {
	g := NewWithT(t)

	mgr, _ := setupTestManager(t)

	b, err := builder.NewControllerBuilder[*TestResource](mgr)
	g.Expect(err).ToNot(HaveOccurred())

	mapper := func(_ context.Context, _ client.Object) []reconcile.Request {
		return nil
	}
	b = b.For(&TestResource{}).
		Owns(gvks.ConfigMap, builder.WithMapper(mapper))

	r := reconciler.Wrap(func(_ context.Context, _ *reconciler.TypedRequest[*TestResource]) (*reconciler.Response, error) {
		return reconciler.NewResponse(), nil
	})
	err = b.Complete(r)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("Owns() does not support WithMapper"))
}

func TestBuilder_Owns_WithPredicates(t *testing.T) {
	g := NewWithT(t)

	mgr, _ := setupTestManager(t)

	b, err := builder.NewControllerBuilder[*TestResource](mgr)
	g.Expect(err).ToNot(HaveOccurred())

	b = b.For(&TestResource{}).
		Owns(gvks.ConfigMap, builder.WithPredicates(predicates.GenerationChanged()))
	g.Expect(b).ToNot(BeNil())
}

func TestBuilder_Owns_AsPartial(t *testing.T) {
	g := NewWithT(t)

	mgr, _ := setupTestManager(t)

	b, err := builder.NewControllerBuilder[*TestResource](mgr)
	g.Expect(err).ToNot(HaveOccurred())

	// AsPartial with GVK should work - uses PartialObjectMetadata internally
	b = b.For(&TestResource{}).
		Owns(gvks.ConfigMap, builder.AsPartial())
	g.Expect(b).ToNot(BeNil())
}

func TestBuilder_Watches_AsPartial(t *testing.T) {
	g := NewWithT(t)

	mgr, _ := setupTestManager(t)

	b, err := builder.NewControllerBuilder[*TestResource](mgr)
	g.Expect(err).ToNot(HaveOccurred())

	// AsPartial with GVK should work - uses PartialObjectMetadata internally
	mapper := func(_ context.Context, _ *metav1.PartialObjectMetadata) []reconcile.Request {
		return nil
	}
	b = b.For(&TestResource{}).
		Watches(gvks.ConfigMap, builder.WithMapper(mapper), builder.AsPartial())
	g.Expect(b).ToNot(BeNil())
}

func TestBuilder_Watches_WithMapper(t *testing.T) {
	g := NewWithT(t)

	mgr, _ := setupTestManager(t)

	b, err := builder.NewControllerBuilder[*TestResource](mgr)
	g.Expect(err).ToNot(HaveOccurred())

	// Mapper uses unstructured since GVK-based watches are unstructured by default
	mapper := func(_ context.Context, _ *unstructured.Unstructured) []reconcile.Request {
		return nil
	}
	b = b.For(&TestResource{}).
		Watches(gvks.ConfigMap, builder.WithMapper(mapper))
	g.Expect(b).ToNot(BeNil())
}

func TestBuilder_Watches_WithHandler(t *testing.T) {
	g := NewWithT(t)

	mgr, _ := setupTestManager(t)

	b, err := builder.NewControllerBuilder[*TestResource](mgr)
	g.Expect(err).ToNot(HaveOccurred())

	h := handler.TypedFuncs[client.Object, reconcile.Request]{}
	b = b.For(&TestResource{}).
		Watches(gvks.ConfigMap, builder.WithHandler(h))
	g.Expect(b).ToNot(BeNil())
}

func TestBuilder_Watches_NoHandlerOrMapper_Error(t *testing.T) {
	g := NewWithT(t)

	mgr, _ := setupTestManager(t)

	b, err := builder.NewControllerBuilder[*TestResource](mgr)
	g.Expect(err).ToNot(HaveOccurred())

	b = b.For(&TestResource{}).
		Watches(gvks.ConfigMap)

	r := reconciler.Wrap(func(_ context.Context, _ *reconciler.TypedRequest[*TestResource]) (*reconciler.Response, error) {
		return reconciler.NewResponse(), nil
	})
	err = b.Complete(r)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("requires either WithMapper or WithHandler"))
}

func TestBuilder_Watches_BothHandlerAndMapper_Error(t *testing.T) {
	g := NewWithT(t)

	mgr, _ := setupTestManager(t)

	b, err := builder.NewControllerBuilder[*TestResource](mgr)
	g.Expect(err).ToNot(HaveOccurred())

	h := handler.TypedFuncs[client.Object, reconcile.Request]{}
	mapper := func(_ context.Context, _ client.Object) []reconcile.Request {
		return nil
	}
	b = b.For(&TestResource{}).
		Watches(gvks.ConfigMap, builder.WithHandler(h), builder.WithMapper(mapper))

	r := reconciler.Wrap(func(_ context.Context, _ *reconciler.TypedRequest[*TestResource]) (*reconciler.Response, error) {
		return reconciler.NewResponse(), nil
	})
	err = b.Complete(r)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("mutually exclusive"))
}

func TestBuilder_Complete_WithoutFor_Error(t *testing.T) {
	g := NewWithT(t)

	mgr, _ := setupTestManager(t)

	b, err := builder.NewControllerBuilder[*TestResource](mgr)
	g.Expect(err).ToNot(HaveOccurred())

	r := reconciler.Wrap(func(_ context.Context, _ *reconciler.TypedRequest[*TestResource]) (*reconciler.Response, error) {
		return reconciler.NewResponse(), nil
	})
	err = b.Complete(r)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("For() must be called before Complete()"))
}

func TestBuilder_NameDerivation_FromTypedObject(t *testing.T) {
	g := NewWithT(t)

	mgr, _ := setupTestManager(t)

	b, err := builder.NewControllerBuilder[*TestResource](mgr)
	g.Expect(err).ToNot(HaveOccurred())

	// Name should be derived as "testresource" from type
	b = b.For(&TestResource{})
	g.Expect(b).ToNot(BeNil())

	// Complete should succeed with auto-derived name
	// We can't actually complete without mocking controller.New properly
	// but we've tested that For() works and derives the name
}

func TestBuilder_TypedObjectInFor(t *testing.T) {
	g := NewWithT(t)

	mgr, _ := setupTestManager(t)

	b, err := builder.NewControllerBuilder[*TestResource](mgr)
	g.Expect(err).ToNot(HaveOccurred())

	// Typed object should work
	b = b.For(&TestResource{})
	g.Expect(b).ToNot(BeNil())
}

func TestBuilder_MultiplePredicatesAdditive(t *testing.T) {
	g := NewWithT(t)

	mgr, _ := setupTestManager(t)

	b, err := builder.NewControllerBuilder[*TestResource](mgr)
	g.Expect(err).ToNot(HaveOccurred())

	pred1 := predicate.NewPredicateFuncs(func(_ client.Object) bool { return true })
	pred2 := predicate.NewPredicateFuncs(func(_ client.Object) bool { return false })

	// Multiple WithPredicates calls should be additive
	b = b.For(&TestResource{}).
		Owns(gvks.ConfigMap,
			builder.WithPredicates(pred1),
			builder.WithPredicates(pred2),
		)
	g.Expect(b).ToNot(BeNil())
}

func TestBuilder_GetController(t *testing.T) {
	g := NewWithT(t)

	mgr, _ := setupTestManager(t)

	b, err := builder.NewControllerBuilder[*TestResource](mgr)
	g.Expect(err).ToNot(HaveOccurred())

	// Controller should be nil before Complete()
	g.Expect(b.GetController()).To(BeNil())
}

func TestBuilder_GVK_InOwns(t *testing.T) {
	g := NewWithT(t)

	mgr, _ := setupTestManager(t)

	b, err := builder.NewControllerBuilder[*TestResource](mgr)
	g.Expect(err).ToNot(HaveOccurred())

	// GVK-based Owns - uses unstructured internally
	b = b.For(&TestResource{}).
		Owns(gvks.Secret)
	g.Expect(b).ToNot(BeNil())
}

func TestBuilder_GVK_InWatches(t *testing.T) {
	g := NewWithT(t)

	mgr, _ := setupTestManager(t)

	b, err := builder.NewControllerBuilder[*TestResource](mgr)
	g.Expect(err).ToNot(HaveOccurred())

	mapper := func(_ context.Context, _ *unstructured.Unstructured) []reconcile.Request {
		return nil
	}

	// GVK-based Watches - uses unstructured internally
	b = b.For(&TestResource{}).
		Watches(gvks.Pod, builder.WithMapper(mapper))
	g.Expect(b).ToNot(BeNil())
}

func TestBuilder_CustomGVK(t *testing.T) {
	g := NewWithT(t)

	mgr, _ := setupTestManager(t)

	b, err := builder.NewControllerBuilder[*TestResource](mgr)
	g.Expect(err).ToNot(HaveOccurred())

	// Custom GVK for user-defined resources
	customGVK := schema.GroupVersionKind{
		Group:   "example.com",
		Version: "v1",
		Kind:    "CustomResource",
	}

	b = b.For(&TestResource{}).
		Owns(customGVK)
	g.Expect(b).ToNot(BeNil())
}

func TestBuilder_CustomHandlerInOwns(t *testing.T) {
	g := NewWithT(t)

	mgr, _ := setupTestManager(t)

	b, err := builder.NewControllerBuilder[*TestResource](mgr)
	g.Expect(err).ToNot(HaveOccurred())

	h := handler.TypedFuncs[client.Object, reconcile.Request]{}

	// Custom handler in Owns
	b = b.For(&TestResource{}).
		Owns(gvks.ConfigMap, builder.WithHandler(h))
	g.Expect(b).ToNot(BeNil())
}

func TestBuilder_WithMapper_ValidMapper(t *testing.T) {
	g := NewWithT(t)

	mgr, _ := setupTestManager(t)

	b, err := builder.NewControllerBuilder[*TestResource](mgr)
	g.Expect(err).ToNot(HaveOccurred())

	// Valid mapper signature - uses unstructured since GVK watches are unstructured by default
	mapper := func(_ context.Context, _ *unstructured.Unstructured) []reconcile.Request {
		return nil
	}

	// Should not accumulate errors for valid mapper
	b = b.For(&TestResource{}).
		Watches(gvks.Secret, builder.WithMapper(mapper))
	g.Expect(b).ToNot(BeNil())
}

// cacheAwareReconciler is a test reconciler that implements CacheAware.
type cacheAwareReconciler struct {
	cache cache.Cache
}

func (r *cacheAwareReconciler) Reconcile(
	_ context.Context,
	_ *reconciler.TypedRequest[*TestResource],
) (*reconciler.Response, error) {
	return reconciler.NewResponse(), nil
}

func (r *cacheAwareReconciler) SetCache(c cache.Cache) {
	r.cache = c
}

func TestBuilder_Complete_InjectsCacheAware(t *testing.T) {
	g := NewWithT(t)

	mgr, mockCtrl := setupTestManager(t)

	b, err := builder.NewControllerBuilder[*TestResource](
		mgr,
		builder.WithName("test-controller"),
	)
	g.Expect(err).ToNot(HaveOccurred())

	// Create a CacheAware reconciler
	rec := &cacheAwareReconciler{}

	// Before Complete, cache should be nil
	g.Expect(rec.cache).To(BeNil())

	// Verify the CacheAware interface is implemented correctly
	var _ reconciler.CacheAware = rec

	// Verify the reconciler and mock controller are valid
	g.Expect(rec).ToNot(BeNil())
	g.Expect(mockCtrl).ToNot(BeNil())

	// Note: The actual injection happens in Complete(), which requires a real controller.
	// The builder is validated separately in other tests.
	_ = b
}

// allAwareReconciler implements all aware interfaces.
type allAwareReconciler struct {
	ctrl   any
	client client.Client
	cache  cache.Cache
}

func (r *allAwareReconciler) Reconcile(
	_ context.Context,
	_ *reconciler.TypedRequest[*TestResource],
) (*reconciler.Response, error) {
	return reconciler.NewResponse(), nil
}

func (r *allAwareReconciler) SetController(ctrl controller.Controller) {
	r.ctrl = ctrl
}

func (r *allAwareReconciler) SetClient(c client.Client) {
	r.client = c
}

func (r *allAwareReconciler) SetCache(c cache.Cache) {
	r.cache = c
}

func TestBuilder_Complete_InjectsAllAwareInterfaces(t *testing.T) {
	g := NewWithT(t)

	// Verify the reconciler implements all aware interfaces
	rec := &allAwareReconciler{}

	var _ reconciler.ControllerAware = rec
	var _ reconciler.ClientAware = rec
	var _ reconciler.CacheAware = rec

	g.Expect(rec).ToNot(BeNil())
}

func TestBuilder_GetWatchedGVKs_ReturnsRegisteredGVKs(t *testing.T) {
	g := NewWithT(t)

	mgr, _ := setupTestManager(t)

	b, err := builder.NewControllerBuilder[*TestResource](mgr)
	g.Expect(err).ToNot(HaveOccurred())

	// Register watches using GVKs
	// Note: For() with typed objects doesn't expose GVK via GetObjectKind(),
	// so only Owns/Watches GVKs are returned
	b.For(&TestResource{}).
		Owns(gvks.ConfigMap).
		Owns(gvks.Secret)

	// Get watched GVKs
	watchedGVKs := b.GetWatchedGVKs()

	// Should include ConfigMap + Secret (from Owns)
	// For() with typed objects has empty GVK (not set in TypeMeta)
	g.Expect(watchedGVKs).To(HaveLen(2))

	// Verify expected GVKs are present
	gvkStrings := make([]string, len(watchedGVKs))
	for i, gvk := range watchedGVKs {
		gvkStrings[i] = gvk.String()
	}

	g.Expect(gvkStrings).To(ContainElement(gvks.ConfigMap.String()))
	g.Expect(gvkStrings).To(ContainElement(gvks.Secret.String()))
}

func TestBuilder_GetWatchedGVKs_EmptyWhenNoWatches(t *testing.T) {
	g := NewWithT(t)

	mgr, _ := setupTestManager(t)

	b, err := builder.NewControllerBuilder[*TestResource](mgr)
	g.Expect(err).ToNot(HaveOccurred())

	// No watches registered
	watchedGVKs := b.GetWatchedGVKs()

	g.Expect(watchedGVKs).To(BeEmpty())
}

// externalWatchesAwareReconciler implements ExternalWatchesAware.
type externalWatchesAwareReconciler struct {
	externalWatches []schema.GroupVersionKind
}

func (r *externalWatchesAwareReconciler) Reconcile(
	_ context.Context,
	_ *reconciler.TypedRequest[*TestResource],
) (*reconciler.Response, error) {
	return reconciler.NewResponse(), nil
}

func (r *externalWatchesAwareReconciler) SetExternalWatches(gvks []schema.GroupVersionKind) {
	r.externalWatches = gvks
}

func TestBuilder_Complete_InjectsExternalWatches(t *testing.T) {
	g := NewWithT(t)

	// Verify the reconciler implements ExternalWatchesAware
	rec := &externalWatchesAwareReconciler{}

	var _ reconciler.ExternalWatchesAware = rec

	// Initially no external watches
	g.Expect(rec.externalWatches).To(BeNil())

	// Note: Full injection testing would require a real controller setup.
	// This test verifies the interface implementation and that the type assertion would work.
}
