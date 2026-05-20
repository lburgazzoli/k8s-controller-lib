//nolint:testpackage // White-box testing: tests internal TypedPipeline initialization
package pipeline

import (
	"context"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/reconciler"
	"github.com/lburgazzoli/k8s-controller-lib/pkg/reconciler/watch"

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

func TestNewTyped_WithPostApply(t *testing.T) {
	g := NewWithT(t)

	w := watch.New()
	p := NewTyped[*TestResource](
		WithFieldOwner("test-controller"),
		WithPostApply(watch.All(w)),
	)

	g.Expect(p).ToNot(BeNil())
	g.Expect(p.Pipeline.opts.PostApply).To(HaveLen(1))
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
	var _ reconciler.ClientAware = p

	// Also verify it implements TypedReconciler
	var _ reconciler.TypedReconciler[*TestResource] = p

	g.Expect(p).ToNot(BeNil())
}

func TestTypedPipeline_WatcherNilSafe(t *testing.T) {
	g := NewWithT(t)

	// Pipeline without hooks should work fine
	p := NewTyped[*TestResource](
		WithFieldOwner("test-controller"),
	)

	g.Expect(p.Pipeline.opts.PostApply).To(BeEmpty())
}

func TestTypedPipeline_ConcurrentClientAccess(t *testing.T) {
	g := NewWithT(t)

	p := NewTyped[*TestResource](
		WithFieldOwner("test-controller"),
	)

	c := newTestClient()

	// Set client concurrently
	done := make(chan struct{})

	go func() {
		p.SetClient(c)
		close(done)
	}()

	<-done

	g.Expect(p.Pipeline.opts.Client).ToNot(BeNil())
}
