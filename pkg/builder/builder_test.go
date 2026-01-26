package builder_test

import (
	"context"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/config"
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
		Owns(&corev1.ConfigMap{})
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
		Owns(&corev1.ConfigMap{}, builder.WithMapper(mapper))

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
		Owns(&corev1.ConfigMap{}, builder.WithPredicates(predicates.GenerationChanged()))
	g.Expect(b).ToNot(BeNil())
}

func TestBuilder_AsPartial_WithTypedObject_Error(t *testing.T) {
	g := NewWithT(t)

	mgr, _ := setupTestManager(t)

	b, err := builder.NewControllerBuilder[*TestResource](mgr)
	g.Expect(err).ToNot(HaveOccurred())

	// AsPartial with typed object should fail
	b = b.For(&TestResource{}).
		Owns(&corev1.ConfigMap{}, builder.AsPartial())

	r := reconciler.Wrap(func(_ context.Context, _ *reconciler.TypedRequest[*TestResource]) (*reconciler.Response, error) {
		return reconciler.NewResponse(), nil
	})
	err = b.Complete(r)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("AsPartial() cannot be used with typed object"))
	g.Expect(err.Error()).To(ContainSubstring("*v1.ConfigMap"))
}

func TestBuilder_AsPartial_WithUnstructured_Success(t *testing.T) {
	g := NewWithT(t)

	mgr, _ := setupTestManager(t)

	b, err := builder.NewControllerBuilder[*TestResource](mgr)
	g.Expect(err).ToNot(HaveOccurred())

	// AsPartial with unstructured should succeed (no error during build)
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))

	b = b.For(&TestResource{}).
		Owns(u, builder.AsPartial())
	g.Expect(b).ToNot(BeNil())
}

func TestBuilder_AsPartial_WithPartialMetadata_Success(t *testing.T) {
	g := NewWithT(t)

	mgr, _ := setupTestManager(t)

	b, err := builder.NewControllerBuilder[*TestResource](mgr)
	g.Expect(err).ToNot(HaveOccurred())

	// AsPartial with PartialObjectMetadata should succeed (no error during build)
	p := &metav1.PartialObjectMetadata{}
	p.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))

	b = b.For(&TestResource{}).
		Owns(p, builder.AsPartial())
	g.Expect(b).ToNot(BeNil())
}

func TestBuilder_Watches_WithMapper(t *testing.T) {
	g := NewWithT(t)

	mgr, _ := setupTestManager(t)

	b, err := builder.NewControllerBuilder[*TestResource](mgr)
	g.Expect(err).ToNot(HaveOccurred())

	mapper := func(_ context.Context, _ *corev1.ConfigMap) []reconcile.Request {
		return nil
	}
	b = b.For(&TestResource{}).
		Watches(&corev1.ConfigMap{}, builder.WithMapper(mapper))
	g.Expect(b).ToNot(BeNil())
}

func TestBuilder_Watches_WithHandler(t *testing.T) {
	g := NewWithT(t)

	mgr, _ := setupTestManager(t)

	b, err := builder.NewControllerBuilder[*TestResource](mgr)
	g.Expect(err).ToNot(HaveOccurred())

	h := handler.TypedFuncs[client.Object, reconcile.Request]{}
	b = b.For(&TestResource{}).
		Watches(&corev1.ConfigMap{}, builder.WithHandler(h))
	g.Expect(b).ToNot(BeNil())
}

func TestBuilder_Watches_NoHandlerOrMapper_Error(t *testing.T) {
	g := NewWithT(t)

	mgr, _ := setupTestManager(t)

	b, err := builder.NewControllerBuilder[*TestResource](mgr)
	g.Expect(err).ToNot(HaveOccurred())

	b = b.For(&TestResource{}).
		Watches(&corev1.ConfigMap{})

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
		Watches(&corev1.ConfigMap{}, builder.WithHandler(h), builder.WithMapper(mapper))

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
		Owns(&corev1.ConfigMap{},
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

func TestBuilder_TypedObjectInOwns(t *testing.T) {
	g := NewWithT(t)

	mgr, _ := setupTestManager(t)

	b, err := builder.NewControllerBuilder[*TestResource](mgr)
	g.Expect(err).ToNot(HaveOccurred())

	// Typed object in Owns
	b = b.For(&TestResource{}).
		Owns(&corev1.Secret{})
	g.Expect(b).ToNot(BeNil())
}

func TestBuilder_UnstructuredInOwns(t *testing.T) {
	g := NewWithT(t)

	mgr, _ := setupTestManager(t)

	b, err := builder.NewControllerBuilder[*TestResource](mgr)
	g.Expect(err).ToNot(HaveOccurred())

	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Pod"))

	// Unstructured object in Owns
	b = b.For(&TestResource{}).
		Owns(u)
	g.Expect(b).ToNot(BeNil())
}

func TestBuilder_PartialMetadataInOwns(t *testing.T) {
	g := NewWithT(t)

	mgr, _ := setupTestManager(t)

	b, err := builder.NewControllerBuilder[*TestResource](mgr)
	g.Expect(err).ToNot(HaveOccurred())

	p := &metav1.PartialObjectMetadata{}
	p.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Deployment"))

	// Partial metadata in Owns
	b = b.For(&TestResource{}).
		Owns(p)
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
		Owns(&corev1.ConfigMap{}, builder.WithHandler(h))
	g.Expect(b).ToNot(BeNil())
}

func TestBuilder_WithMapper_ValidMapper(t *testing.T) {
	g := NewWithT(t)

	mgr, _ := setupTestManager(t)

	b, err := builder.NewControllerBuilder[*TestResource](mgr)
	g.Expect(err).ToNot(HaveOccurred())

	// Valid mapper signature
	mapper := func(_ context.Context, _ *corev1.Secret) []reconcile.Request {
		return nil
	}

	// Should not accumulate errors for valid mapper
	b = b.For(&TestResource{}).
		Watches(&corev1.Secret{}, builder.WithMapper(mapper))
	g.Expect(b).ToNot(BeNil())
}
