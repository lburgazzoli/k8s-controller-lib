package builder_test

import (
	"context"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lburgazzoli/k3s-envtest/pkg/k3senv"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/builder"
	"github.com/lburgazzoli/k8s-controller-lib/pkg/predicates"
	"github.com/lburgazzoli/k8s-controller-lib/pkg/reconciler"
	"github.com/lburgazzoli/k8s-controller-lib/pkg/resources/gvks"
	"github.com/lburgazzoli/k8s-controller-lib/pkg/status"

	. "github.com/onsi/gomega"
)

//nolint:gochecknoglobals // Shared test environment for all integration tests
var (
	testEnv *k3senv.K3sEnv
	testMgr manager.Manager
)

// TestApp is a custom resource for testing.
type TestApp struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Status status.Status `json:"status"`
}

func (r *TestApp) GetStatus() *status.Status {
	return &r.Status
}

func (r *TestApp) SetStatus(s *status.Status) {
	if s != nil {
		r.Status = *s
	}
}

func (r *TestApp) DeepCopyObject() runtime.Object {
	if r == nil {
		return nil
	}
	out := new(TestApp)
	r.DeepCopyInto(out)

	return out
}

func (r *TestApp) DeepCopyInto(out *TestApp) {
	*out = *r
	out.TypeMeta = r.TypeMeta
	r.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Status = *r.Status.DeepCopy()
}

func (r *TestApp) GetObjectKind() schema.ObjectKind {
	return r
}

func (r *TestApp) GroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   "test.example.com",
		Version: "v1",
		Kind:    "TestApp",
	}
}

func (r *TestApp) SetGroupVersionKind(gvk schema.GroupVersionKind) {
	r.APIVersion = gvk.GroupVersion().String()
	r.Kind = gvk.Kind
}

type TestAppList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []TestApp `json:"items"`
}

func (r *TestAppList) DeepCopyObject() runtime.Object {
	if r == nil {
		return nil
	}
	out := new(TestAppList)
	r.DeepCopyInto(out)

	return out
}

func (r *TestAppList) DeepCopyInto(out *TestAppList) {
	*out = *r
	r.ListMeta.DeepCopyInto(&out.ListMeta)
	if r.Items != nil {
		out.Items = make([]TestApp, len(r.Items))
		for i := range r.Items {
			r.Items[i].DeepCopyInto(&out.Items[i])
		}
	}
}

// TestMain sets up a shared k3s environment for all tests.
func TestMain(m *testing.M) {
	ctx := context.Background()
	g := NewGomegaWithT(&testing.T{})

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		panic(err)
	}
	if err := apiextensionsv1.AddToScheme(scheme); err != nil {
		panic(err)
	}

	// Register custom resource
	schemeBuilder := runtime.NewSchemeBuilder(func(s *runtime.Scheme) error {
		gv := schema.GroupVersion{Group: "test.example.com", Version: "v1"}
		s.AddKnownTypes(gv, &TestApp{}, &TestAppList{})
		metav1.AddToGroupVersion(s, gv)

		return nil
	})
	if err := schemeBuilder.AddToScheme(scheme); err != nil {
		panic(err)
	}

	// Create shared k3s environment
	env, err := k3senv.New(k3senv.WithScheme(scheme))
	if err != nil {
		panic(err)
	}
	if err := env.Start(ctx); err != nil {
		panic(err)
	}
	testEnv = env

	// Create CRD
	crd := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "testapps.test.example.com",
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: "test.example.com",
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Kind:     "TestApp",
				ListKind: "TestAppList",
				Plural:   "testapps",
				Singular: "testapp",
			},
			Scope: apiextensionsv1.NamespaceScoped,
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{{
				Name:    "v1",
				Served:  true,
				Storage: true,
				Schema: &apiextensionsv1.CustomResourceValidation{
					OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
						Type: "object",
						Properties: map[string]apiextensionsv1.JSONSchemaProps{
							"status": {
								Type:                   "object",
								XPreserveUnknownFields: ptr.To(true),
							},
						},
					},
				},
				Subresources: &apiextensionsv1.CustomResourceSubresources{
					Status: &apiextensionsv1.CustomResourceSubresourceStatus{},
				},
			}},
		},
	}
	if err := env.InstallCRD(ctx, crd); err != nil {
		panic(err)
	}

	// Create shared manager
	mgr, err := manager.New(env.Config(), manager.Options{
		Scheme: scheme,
	})
	if err != nil {
		panic(err)
	}
	testMgr = mgr

	// Start manager
	mgrCtx, mgrCancel := context.WithCancel(ctx)
	go func() {
		_ = mgr.Start(mgrCtx)
	}()

	// Wait for cache to sync
	cache := mgr.GetCache()
	g.Eventually(func() bool {
		return cache.WaitForCacheSync(ctx)
	}).WithTimeout(10 * time.Second).Should(BeTrue())

	// Run tests
	code := m.Run()

	// Cleanup
	mgrCancel()
	_ = env.Stop(ctx)

	os.Exit(code)
}

// setupTestEnv returns the shared k3s environment and manager.
func setupTestEnv(t *testing.T) (*k3senv.K3sEnv, manager.Manager) {
	t.Helper()

	return testEnv, testMgr
}

func TestBuilder_BasicGVKWatches(t *testing.T) {
	g := NewWithT(t)

	_, mgr := setupTestEnv(t)
	ctx := t.Context()

	reconcileCount := atomic.Int32{}
	reconcileFn := func(_ context.Context, _ *reconciler.TypedRequest[*TestApp]) (*reconciler.Response, error) {
		reconcileCount.Add(1)

		return reconciler.NewResponse(), nil
	}

	// Build controller with GVK-based watches
	b, err := builder.NewControllerBuilder[*TestApp](mgr, builder.WithName("basic-gvk-watches"))
	g.Expect(err).ToNot(HaveOccurred())

	err = b.For(&TestApp{}).
		Owns(gvks.ConfigMap).
		Owns(gvks.Secret).
		Complete(reconciler.Wrap(reconcileFn))
	g.Expect(err).ToNot(HaveOccurred())

	// Create test object
	app := &TestApp{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "test.example.com/v1",
			Kind:       "TestApp",
		},
	}
	app.SetName("test-app-basic")
	app.SetNamespace("default")

	cli := mgr.GetClient()
	g.Expect(cli.Create(ctx, app)).To(Succeed())
	t.Cleanup(func() { _ = cli.Delete(t.Context(), app) })

	// Wait for initial reconcile
	g.Eventually(reconcileCount.Load).WithTimeout(5 * time.Second).Should(BeNumerically(">", 0))

	initialCount := reconcileCount.Load()

	// Create owned ConfigMap
	cm := &corev1.ConfigMap{}
	cm.SetName("owned-cm")
	cm.SetNamespace("default")
	cm.SetOwnerReferences([]metav1.OwnerReference{{
		APIVersion: "test.example.com/v1",
		Kind:       "TestApp",
		Name:       app.Name,
		UID:        app.UID,
		Controller: ptr.To(true),
	}})

	g.Expect(cli.Create(ctx, cm)).To(Succeed())
	t.Cleanup(func() { _ = cli.Delete(t.Context(), cm) })

	// Wait for reconcile triggered by owned resource
	g.Eventually(reconcileCount.Load).WithTimeout(5 * time.Second).Should(BeNumerically(">", initialCount))
}

func TestBuilder_GVKWithAsPartial(t *testing.T) {
	g := NewWithT(t)

	_, mgr := setupTestEnv(t)

	reconcileCount := atomic.Int32{}
	reconcileFn := func(_ context.Context, _ *reconciler.TypedRequest[*TestApp]) (*reconciler.Response, error) {
		reconcileCount.Add(1)

		return reconciler.NewResponse(), nil
	}

	// Build controller with GVK watches - unstructured by default, partial with AsPartial()
	b, err := builder.NewControllerBuilder[*TestApp](mgr, builder.WithName("gvk-aspartial"))
	g.Expect(err).ToNot(HaveOccurred())

	err = b.For(&TestApp{}).
		Owns(gvks.ConfigMap).                   // Unstructured (default)
		Owns(gvks.Secret, builder.AsPartial()). // Partial metadata
		Complete(reconciler.Wrap(reconcileFn))
	g.Expect(err).ToNot(HaveOccurred())

	// Verify controller created
	g.Expect(b.GetController()).ToNot(BeNil())
}

func TestBuilder_CustomMapper(t *testing.T) {
	g := NewWithT(t)

	_, mgr := setupTestEnv(t)
	ctx := t.Context()

	reconcileCount := atomic.Int32{}
	var lastRequestName string

	reconcileFn := func(_ context.Context, req *reconciler.TypedRequest[*TestApp]) (*reconciler.Response, error) {
		reconcileCount.Add(1)
		lastRequestName = req.Object.GetName()

		return reconciler.NewResponse(), nil
	}

	// Mapper that maps secrets to test-app-mapper (uses unstructured since GVK-based watches are unstructured by default)
	secretMapper := func(_ context.Context, u *unstructured.Unstructured) []reconcile.Request {
		secretType, _, _ := unstructured.NestedString(u.Object, "type")
		if secretType == string(corev1.SecretTypeTLS) {
			return []reconcile.Request{
				{NamespacedName: types.NamespacedName{
					Name:      "test-app-mapper",
					Namespace: u.GetNamespace(),
				}},
			}
		}

		return nil
	}

	// Build controller with mapper
	b, err := builder.NewControllerBuilder[*TestApp](mgr, builder.WithName("custom-mapper"))
	g.Expect(err).ToNot(HaveOccurred())

	err = b.For(&TestApp{}).
		Watches(gvks.Secret, builder.WithMapper(secretMapper)).
		Complete(reconciler.Wrap(reconcileFn))
	g.Expect(err).ToNot(HaveOccurred())

	// Create test app
	app := &TestApp{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "test.example.com/v1",
			Kind:       "TestApp",
		},
	}
	app.SetName("test-app-mapper")
	app.SetNamespace("default")

	cli := mgr.GetClient()
	g.Expect(cli.Create(ctx, app)).To(Succeed())
	t.Cleanup(func() { _ = cli.Delete(t.Context(), app) })

	// Wait for initial reconcile
	g.Eventually(reconcileCount.Load).WithTimeout(10 * time.Second).Should(BeNumerically(">", 0))

	initialCount := reconcileCount.Load()

	// Give watches time to fully start
	time.Sleep(2 * time.Second)

	// Create TLS secret (should trigger mapper)
	secret := &corev1.Secret{}
	secret.SetName("tls-secret")
	secret.SetNamespace("default")
	secret.Type = corev1.SecretTypeTLS
	secret.Data = map[string][]byte{
		"tls.crt": []byte("fake-cert"),
		"tls.key": []byte("fake-key"),
	}

	g.Expect(cli.Create(ctx, secret)).To(Succeed())
	t.Cleanup(func() { _ = cli.Delete(t.Context(), secret) })

	// Wait for reconcile triggered by mapper
	g.Eventually(reconcileCount.Load).WithTimeout(10 * time.Second).Should(BeNumerically(">", initialCount))

	g.Expect(lastRequestName).To(Equal("test-app-mapper"))
}

func TestBuilder_AsPartial(t *testing.T) {
	g := NewWithT(t)

	_, mgr := setupTestEnv(t)

	reconcileFn := func(_ context.Context, _ *reconciler.TypedRequest[*TestApp]) (*reconciler.Response, error) {
		return reconciler.NewResponse(), nil
	}

	// Build controller with AsPartial for memory optimization
	// With GVK-based API, AsPartial simply switches from unstructured to partial metadata
	b, err := builder.NewControllerBuilder[*TestApp](mgr, builder.WithName("as-partial"))
	g.Expect(err).ToNot(HaveOccurred())

	err = b.For(&TestApp{}).
		Owns(gvks.ConfigMap, builder.AsPartial()).
		Owns(gvks.Secret, builder.AsPartial()).
		Complete(reconciler.Wrap(reconcileFn))
	g.Expect(err).ToNot(HaveOccurred())
}

func TestBuilder_WithPredicates(t *testing.T) {
	g := NewWithT(t)

	_, mgr := setupTestEnv(t)
	ctx := t.Context()

	reconcileCount := atomic.Int32{}
	reconcileFn := func(_ context.Context, _ *reconciler.TypedRequest[*TestApp]) (*reconciler.Response, error) {
		reconcileCount.Add(1)

		return reconciler.NewResponse(), nil
	}

	// Build controller with predicates
	b, err := builder.NewControllerBuilder[*TestApp](mgr, builder.WithName("with-predicates"))
	g.Expect(err).ToNot(HaveOccurred())

	err = b.For(&TestApp{}).
		Owns(gvks.ConfigMap, builder.WithPredicates(predicates.GenerationChanged())).
		Complete(reconciler.Wrap(reconcileFn))
	g.Expect(err).ToNot(HaveOccurred())

	// Create test app
	app := &TestApp{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "test.example.com/v1",
			Kind:       "TestApp",
		},
	}
	app.SetName("test-app-predicates")
	app.SetNamespace("default")

	cli := mgr.GetClient()
	g.Expect(cli.Create(ctx, app)).To(Succeed())
	t.Cleanup(func() { _ = cli.Delete(t.Context(), app) })

	g.Eventually(reconcileCount.Load).WithTimeout(5 * time.Second).Should(BeNumerically(">", 0))
}

func TestBuilder_StructBasedConfiguration(t *testing.T) {
	g := NewWithT(t)

	_, mgr := setupTestEnv(t)

	reconcileFn := func(_ context.Context, _ *reconciler.TypedRequest[*TestApp]) (*reconciler.Response, error) {
		return reconciler.NewResponse(), nil
	}

	// Define reusable option set
	strictWatch := &builder.WatchOptions{
		Predicates: []predicate.Predicate{
			predicates.GenerationChanged(),
		},
	}

	// Build controller with struct-based configuration
	b, err := builder.NewControllerBuilder[*TestApp](mgr, builder.WithName("struct-based-config"))
	g.Expect(err).ToNot(HaveOccurred())

	err = b.For(&TestApp{}).
		Owns(gvks.ConfigMap, strictWatch).
		Owns(gvks.Secret, strictWatch).
		Complete(reconciler.Wrap(reconcileFn))
	g.Expect(err).ToNot(HaveOccurred())
}

func TestBuilder_HybridConfiguration(t *testing.T) {
	g := NewWithT(t)

	_, mgr := setupTestEnv(t)

	reconcileFn := func(_ context.Context, _ *reconciler.TypedRequest[*TestApp]) (*reconciler.Response, error) {
		return reconciler.NewResponse(), nil
	}

	// Configuration with WatchPartial (partial metadata)
	partialOpts := &builder.WatchOptions{
		Predicates: []predicate.Predicate{
			predicates.GenerationChanged(),
		},
		Strategy: builder.WatchPartial,
	}

	// Configuration with WatchFull (unstructured) - default
	unstructuredOpts := &builder.WatchOptions{
		Predicates: []predicate.Predicate{
			predicates.GenerationChanged(),
		},
		Strategy: builder.WatchFull,
	}

	// Custom handler
	customHandler := handler.TypedFuncs[client.Object, reconcile.Request]{}

	// Build controller with hybrid configuration
	b, err := builder.NewControllerBuilder[*TestApp](mgr, builder.WithName("hybrid-config"))
	g.Expect(err).ToNot(HaveOccurred())

	// GVK with WatchPartial (partial metadata) vs WatchFull (unstructured)
	err = b.For(&TestApp{}).
		Owns(gvks.ConfigMap, partialOpts).
		Owns(gvks.Secret, unstructuredOpts, builder.WithHandler(customHandler)).
		Complete(reconciler.Wrap(reconcileFn))
	g.Expect(err).ToNot(HaveOccurred())
}

func TestBuilder_ValidationErrors(t *testing.T) {
	_, mgr := setupTestEnv(t)

	t.Run("For called twice", func(t *testing.T) {
		g := NewWithT(t)

		reconcileFn := func(_ context.Context, _ *reconciler.TypedRequest[*TestApp]) (*reconciler.Response, error) {
			return reconciler.NewResponse(), nil
		}

		b, err := builder.NewControllerBuilder[*TestApp](mgr, builder.WithName("validation-for-twice"))
		g.Expect(err).ToNot(HaveOccurred())

		err = b.For(&TestApp{}).
			For(&TestApp{}). // Second call
			Complete(reconciler.Wrap(reconcileFn))

		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("For() can only be called once"))
	})

	t.Run("For with WithHandler", func(t *testing.T) {
		g := NewWithT(t)

		reconcileFn := func(_ context.Context, _ *reconciler.TypedRequest[*TestApp]) (*reconciler.Response, error) {
			return reconciler.NewResponse(), nil
		}

		b, err := builder.NewControllerBuilder[*TestApp](mgr, builder.WithName("validation-for-handler"))
		g.Expect(err).ToNot(HaveOccurred())

		h := handler.TypedFuncs[client.Object, reconcile.Request]{}
		err = b.For(&TestApp{}, builder.WithHandler(h)).
			Complete(reconciler.Wrap(reconcileFn))

		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("For() does not support WithHandler"))
	})

	t.Run("For with WithMapper", func(t *testing.T) {
		g := NewWithT(t)

		reconcileFn := func(_ context.Context, _ *reconciler.TypedRequest[*TestApp]) (*reconciler.Response, error) {
			return reconciler.NewResponse(), nil
		}

		b, err := builder.NewControllerBuilder[*TestApp](mgr, builder.WithName("validation-for-mapper"))
		g.Expect(err).ToNot(HaveOccurred())

		mapper := func(_ context.Context, _ client.Object) []reconcile.Request {
			return nil
		}
		err = b.For(&TestApp{}, builder.WithMapper(mapper)).
			Complete(reconciler.Wrap(reconcileFn))

		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("For() does not support WithMapper"))
	})

	t.Run("Owns with WithMapper", func(t *testing.T) {
		g := NewWithT(t)

		reconcileFn := func(_ context.Context, _ *reconciler.TypedRequest[*TestApp]) (*reconciler.Response, error) {
			return reconciler.NewResponse(), nil
		}

		b, err := builder.NewControllerBuilder[*TestApp](mgr, builder.WithName("validation-owns-mapper"))
		g.Expect(err).ToNot(HaveOccurred())

		mapper := func(_ context.Context, _ client.Object) []reconcile.Request {
			return nil
		}
		err = b.For(&TestApp{}).
			Owns(gvks.ConfigMap, builder.WithMapper(mapper)).
			Complete(reconciler.Wrap(reconcileFn))

		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("Owns() does not support WithMapper"))
	})

	t.Run("Watches without handler or mapper", func(t *testing.T) {
		g := NewWithT(t)

		reconcileFn := func(_ context.Context, _ *reconciler.TypedRequest[*TestApp]) (*reconciler.Response, error) {
			return reconciler.NewResponse(), nil
		}

		b, err := builder.NewControllerBuilder[*TestApp](mgr, builder.WithName("validation-watches-none"))
		g.Expect(err).ToNot(HaveOccurred())

		err = b.For(&TestApp{}).
			Watches(gvks.ConfigMap).
			Complete(reconciler.Wrap(reconcileFn))

		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("requires either WithMapper or WithHandler"))
	})

	t.Run("Watches with both handler and mapper", func(t *testing.T) {
		g := NewWithT(t)

		reconcileFn := func(_ context.Context, _ *reconciler.TypedRequest[*TestApp]) (*reconciler.Response, error) {
			return reconciler.NewResponse(), nil
		}

		b, err := builder.NewControllerBuilder[*TestApp](mgr, builder.WithName("validation-watches-both"))
		g.Expect(err).ToNot(HaveOccurred())

		h := handler.TypedFuncs[client.Object, reconcile.Request]{}
		mapper := func(_ context.Context, _ client.Object) []reconcile.Request {
			return nil
		}

		err = b.For(&TestApp{}).
			Watches(gvks.ConfigMap, builder.WithHandler(h), builder.WithMapper(mapper)).
			Complete(reconciler.Wrap(reconcileFn))

		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("mutually exclusive"))
	})

	t.Run("Complete without For", func(t *testing.T) {
		g := NewWithT(t)

		reconcileFn := func(_ context.Context, _ *reconciler.TypedRequest[*TestApp]) (*reconciler.Response, error) {
			return reconciler.NewResponse(), nil
		}

		b, err := builder.NewControllerBuilder[*TestApp](mgr, builder.WithName("validation-no-for"))
		g.Expect(err).ToNot(HaveOccurred())

		err = b.Complete(reconciler.Wrap(reconcileFn))

		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("For() must be called before Complete()"))
	})
}

func TestBuilder_WithCustomHandler(t *testing.T) {
	g := NewWithT(t)

	_, mgr := setupTestEnv(t)

	reconcileFn := func(_ context.Context, _ *reconciler.TypedRequest[*TestApp]) (*reconciler.Response, error) {
		return reconciler.NewResponse(), nil
	}

	// Custom handler
	customHandler := handler.TypedFuncs[client.Object, reconcile.Request]{}

	// Build controller with custom handler
	b, err := builder.NewControllerBuilder[*TestApp](mgr, builder.WithName("with-custom-handler"))
	g.Expect(err).ToNot(HaveOccurred())

	err = b.For(&TestApp{}).
		Owns(gvks.ConfigMap, builder.WithHandler(customHandler)).
		Complete(reconciler.Wrap(reconcileFn))
	g.Expect(err).ToNot(HaveOccurred())
}

func TestBuilder_ControllerOptions(t *testing.T) {
	_, mgr := setupTestEnv(t)
	g := NewWithT(t)

	reconcileFn := func(_ context.Context, _ *reconciler.TypedRequest[*TestApp]) (*reconciler.Response, error) {
		return reconciler.NewResponse(), nil
	}

	// Build controller with options including custom name
	b, err := builder.NewControllerBuilder[*TestApp](
		mgr,
		builder.WithName("test-opts"),
		builder.WithMaxConcurrentReconciles(3),
	)
	g.Expect(err).ToNot(HaveOccurred())

	err = b.For(&TestApp{}).
		Complete(reconciler.Wrap(reconcileFn))
	g.Expect(err).ToNot(HaveOccurred())
}
