package reconciler_test

import (
	"context"
	"testing"
	"time"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/reconciler"
	"github.com/lburgazzoli/k8s-controller-lib/pkg/status"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

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
	fn := reconciler.ActionFunc(func(ctx context.Context, req *reconciler.Request, resp *reconciler.Response) error {
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

	duration := resp.ShouldRequeue()
	g.Expect(duration).To(Equal(time.Duration(0)))
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
	action := reconciler.ActionFunc(func(ctx context.Context, req *reconciler.Request, resp *reconciler.Response) error {
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
