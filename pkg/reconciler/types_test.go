package reconciler_test

import (
	"context"
	"testing"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/reconciler"
	"github.com/lburgazzoli/k8s-controller-lib/pkg/status"

	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

// TestResource is a minimal test type implementing ManagedObject.
type TestResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   TestResourceSpec `json:"spec"`
	Status status.Status    `json:"status"`
}

type TestResourceSpec struct {
	Image string `json:"image,omitempty"`
}

func (r *TestResource) DeepCopy() *TestResource {
	if r == nil {
		return nil
	}
	out := new(TestResource)
	r.DeepCopyInto(out)

	return out
}

func (r *TestResource) DeepCopyInto(out *TestResource) {
	*out = *r
	r.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.TypeMeta = r.TypeMeta
}

func (r *TestResource) DeepCopyObject() runtime.Object {
	return r.DeepCopy()
}

func (r *TestResource) GetStatus() *status.Status {
	return &r.Status
}

func (r *TestResource) SetStatus(s *status.Status) {
	if s != nil {
		r.Status = *s
	}
}

func TestActionFunc_CanBeCalled(t *testing.T) {
	g := NewWithT(t)

	// Test that ActionFunc can be called as a function
	called := false
	fn := reconciler.ActionFunc(func(_ context.Context, _ *reconciler.Request, _ *reconciler.Response) error {
		called = true

		return nil
	})

	req := &reconciler.Request{
		Object: &TestResource{},
	}
	resp := reconciler.NewResponse()

	err := fn(t.Context(), req, resp)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(called).To(BeTrue())
}

func TestResponse_Objects(t *testing.T) {
	g := NewWithT(t)

	resp := reconciler.NewResponse()

	cm1 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "cm1"},
	}
	cm2 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "cm2"},
	}
	cm3 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "cm3"},
	}

	// Test variadic arguments
	resp.Objects(cm1, cm2, cm3)

	objects := resp.GetObjects()
	g.Expect(objects).To(ConsistOf(cm1, cm2, cm3))
}

func TestResponse_Objects_Chainable(t *testing.T) {
	g := NewWithT(t)

	resp := reconciler.NewResponse()

	cm1 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "cm1"},
	}
	cm2 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "cm2"},
	}

	// Test chaining
	result := resp.Objects(cm1).Objects(cm2)

	g.Expect(result).To(BeIdenticalTo(resp))
	g.Expect(resp.GetObjects()).To(HaveLen(2))
}

func TestResponse_Requeue(t *testing.T) {
	g := NewWithT(t)

	resp := reconciler.NewResponse()

	requeueDuration := 5 * time.Minute
	result := resp.Requeue(requeueDuration)

	g.Expect(result).To(BeIdenticalTo(resp))

	duration := resp.ShouldRequeue()
	g.Expect(duration).To(Equal(requeueDuration))
}

func TestResponse_DefaultState(t *testing.T) {
	g := NewWithT(t)

	resp := reconciler.NewResponse()

	g.Expect(resp.GetObjects()).To(BeEmpty())
	g.Expect(resp.GetEntries()).To(BeEmpty())

	duration := resp.ShouldRequeue()
	g.Expect(duration).To(Equal(time.Duration(0)))
}

func TestResponse_Object_WithOptions(t *testing.T) {
	g := NewWithT(t)

	resp := reconciler.NewResponse()

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "cm1"},
	}

	resp.Object(cm, reconciler.WithOwnership(false))

	g.Expect(resp.GetObjects()).To(ConsistOf(cm))

	entries := resp.GetEntries()
	g.Expect(entries).To(HaveLen(1))
	g.Expect(entries[0].Object).To(Equal(cm))
	g.Expect(entries[0].Options.Ownership).ToNot(BeNil())
	g.Expect(*entries[0].Options.Ownership).To(BeFalse())
}

func TestResponse_Object_DefaultOptions(t *testing.T) {
	g := NewWithT(t)

	resp := reconciler.NewResponse()

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "cm1"},
	}

	resp.Object(cm)

	entries := resp.GetEntries()
	g.Expect(entries).To(HaveLen(1))
	g.Expect(entries[0].Options.Ownership).To(BeNil())
}

func TestResponse_MixedObjectAndObjects(t *testing.T) {
	g := NewWithT(t)

	resp := reconciler.NewResponse()

	cm1 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "cm1"},
	}
	cm2 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "cm2"},
	}
	cm3 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "cm3"},
	}

	resp.Objects(cm1, cm2).Object(cm3, reconciler.WithOwnership(false))

	g.Expect(resp.GetObjects()).To(ConsistOf(cm1, cm2, cm3))

	entries := resp.GetEntries()
	g.Expect(entries).To(HaveLen(3))
	g.Expect(entries[0].Options.Ownership).To(BeNil())
	g.Expect(entries[1].Options.Ownership).To(BeNil())
	g.Expect(entries[2].Options.Ownership).ToNot(BeNil())
	g.Expect(*entries[2].Options.Ownership).To(BeFalse())
}

// StatefulAction is a test implementation showing stateful actions.
type StatefulAction struct {
	Image string
}

func (a *StatefulAction) Execute(_ context.Context, req *reconciler.Request, resp *reconciler.Response) error {
	resource := req.Object.(*TestResource).DeepCopy()
	resource.Spec.Image = a.Image

	resp.Objects(resource)

	return nil
}

func TestStatefulAction(t *testing.T) {
	g := NewWithT(t)

	action := &StatefulAction{Image: "nginx:latest"}

	resource := &TestResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-resource",
			Namespace: "default",
		},
		Spec: TestResourceSpec{
			Image: "old-image",
		},
	}

	req := &reconciler.Request{
		Object: resource,
	}
	resp := reconciler.NewResponse()

	err := action.Execute(t.Context(), req, resp)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(resp.GetObjects()).To(HaveLen(1))

	resultResource := resp.GetObjects()[0].(*TestResource)
	g.Expect(resultResource.Spec.Image).To(Equal("nginx:latest"))
}

func TestRequest_TypedAccess(t *testing.T) {
	g := NewWithT(t)

	resource := &TestResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-resource",
			Namespace: "default",
		},
	}

	req := &reconciler.Request{
		Object: resource,
	}

	// Type-safe access without assertion
	g.Expect(req.Object).To(PointTo(MatchFields(IgnoreExtras, Fields{
		"ObjectMeta": MatchFields(IgnoreExtras, Fields{
			"Name":      Equal("test-resource"),
			"Namespace": Equal("default"),
		}),
	})))
}

func TestResponse_MultipleObjects_DifferentTypes(t *testing.T) {
	g := NewWithT(t)

	resp := reconciler.NewResponse()

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "config"},
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "secret"},
	}

	resp.Objects(cm, secret)

	objects := resp.GetObjects()
	g.Expect(objects).To(HaveLen(2))

	// Verify types
	g.Expect(objects[0]).To(BeAssignableToTypeOf(&corev1.ConfigMap{}))
	g.Expect(objects[1]).To(BeAssignableToTypeOf(&corev1.Secret{}))
}

func TestActionFunc_WithContext(t *testing.T) {
	g := NewWithT(t)

	// Demonstrate action that receives context
	action := reconciler.ActionFunc(func(ctx context.Context, req *reconciler.Request, _ *reconciler.Response) error {
		// Actions receive context as first parameter
		// Logger can be retrieved via log.FromContext(ctx)
		g.Expect(ctx).ToNot(BeNil())
		g.Expect(req.Client).ToNot(BeNil())
		g.Expect(req.Object).ToNot(BeNil())

		return nil
	})

	req := &reconciler.Request{
		Object: &TestResource{},
		Client: fake.NewFakeClient(),
	}
	resp := reconciler.NewResponse()

	err := action(t.Context(), req, resp)
	g.Expect(err).ToNot(HaveOccurred())
}

func TestWrap_FunctionToReconciler(t *testing.T) {
	g := NewWithT(t)

	// Test that Wrap converts a function to TypedReconciler
	called := false
	fn := func(ctx context.Context, req *reconciler.TypedRequest[*TestResource]) (*reconciler.Response, error) {
		called = true
		g.Expect(ctx).ToNot(BeNil())
		g.Expect(req).ToNot(BeNil())
		g.Expect(req.Object).ToNot(BeNil())

		return reconciler.NewResponse(), nil
	}

	r := reconciler.Wrap(fn)
	g.Expect(r).ToNot(BeNil())

	resource := &TestResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-resource",
			Namespace: "default",
		},
	}

	req := &reconciler.TypedRequest[*TestResource]{
		Object: resource,
		Client: fake.NewFakeClient(),
	}

	resp, err := r.Reconcile(t.Context(), req)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(resp).ToNot(BeNil())
	g.Expect(called).To(BeTrue())
}

func TestWrap_ReturnsError(t *testing.T) {
	g := NewWithT(t)

	// Test that errors are properly propagated
	expectedErr := context.DeadlineExceeded
	fn := func(_ context.Context, _ *reconciler.TypedRequest[*TestResource]) (*reconciler.Response, error) {
		return nil, expectedErr
	}

	r := reconciler.Wrap(fn)

	resource := &TestResource{}
	req := &reconciler.TypedRequest[*TestResource]{
		Object: resource,
		Client: fake.NewFakeClient(),
	}

	resp, err := r.Reconcile(t.Context(), req)
	g.Expect(err).To(Equal(expectedErr))
	g.Expect(resp).To(BeNil())
}
