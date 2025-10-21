//nolint:testpackage // Need access to private methods
package pipeline

import (
	"context"
	"errors"
	"testing"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/conditions"
	"github.com/lburgazzoli/k8s-controller-lib/pkg/reconciler"
	"github.com/lburgazzoli/k8s-controller-lib/pkg/status"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	. "github.com/onsi/gomega"
)

// TestResource is a custom resource type with status for testing status updates.
type TestResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TestResourceSpec `json:"spec,omitempty"`
	Status status.Status    `json:"status,omitempty"`
}

type TestResourceSpec struct {
	Field string `json:"field,omitempty"`
}

// GetStatus implements status.Accessor.
func (r *TestResource) GetStatus() *status.Status {
	return &r.Status
}

// SetStatus implements status.Accessor.
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
	r.DeepCopyInto(out)
	return out
}

func (r *TestResource) DeepCopyInto(out *TestResource) {
	*out = *r
	out.TypeMeta = r.TypeMeta
	r.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = r.Spec
	out.Status = *r.Status.DeepCopy()
}

func (r *TestResource) GetObjectKind() schema.ObjectKind {
	return r
}

func (r *TestResource) GroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   "test.example.com",
		Version: "v1",
		Kind:    "TestResource",
	}
}

func (r *TestResource) SetGroupVersionKind(gvk schema.GroupVersionKind) {
	r.APIVersion = gvk.GroupVersion().String()
	r.Kind = gvk.Kind
}

func TestReconcile_StatusUpdate_Success(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	// Register TestResource with the scheme
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
			Name:       "test-resource",
			Namespace:  "default",
			Generation: 5,
		},
		Spec: TestResourceSpec{
			Field: "value",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(resource).
		WithStatusSubresource(resource).
		Build()

	var actionExecuted bool
	action := func(_ context.Context, _ *reconciler.Request, _ *reconciler.Response) error {
		actionExecuted = true
		return nil
	}

	p, err := NewPipeline(fakeClient,
		WithFieldOwner("test-controller"),
		WithActions(action),
	)
	g.Expect(err).ToNot(HaveOccurred())

	req := &reconciler.Request{
		Client: fakeClient,
		Object: resource,
	}

	resp, err := p.run(t.Context(), req)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(resp).ToNot(BeNil())
	g.Expect(actionExecuted).To(BeTrue())

	// Verify status was updated
	updatedResource := &TestResource{}
	_ = fakeClient.Get(t.Context(), client.ObjectKeyFromObject(resource), updatedResource)

	// Check ObservedGeneration
	g.Expect(updatedResource.Status.ObservedGeneration).To(Equal(int64(5)))

	// Check ProvisioningFailed condition is True (success)
	cond := conditions.Get(&updatedResource.Status, conditionTypeProvisioningFailed)
	g.Expect(cond).ToNot(BeNil())
	g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))
	g.Expect(cond.Reason).To(Equal("ReconciliationSucceeded"))
}

func TestReconcile_StatusUpdate_Failure(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	// Register TestResource with the scheme
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
			Name:       "test-resource",
			Namespace:  "default",
			Generation: 3,
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(resource).
		WithStatusSubresource(resource).
		Build()

	actionErr := errors.New("action failed")
	action := func(_ context.Context, _ *reconciler.Request, _ *reconciler.Response) error {
		return actionErr
	}

	p, err := NewPipeline(fakeClient,
		WithFieldOwner("test-controller"),
		WithActions(action),
	)
	g.Expect(err).ToNot(HaveOccurred())

	req := &reconciler.Request{
		Client: fakeClient,
		Object: resource,
	}

	resp, err := p.run(t.Context(), req)
	g.Expect(err).To(HaveOccurred())
	g.Expect(errors.Is(err, actionErr)).To(BeTrue())
	g.Expect(resp).ToNot(BeNil())

	// Verify status was updated with failure condition
	updatedResource := &TestResource{}
	_ = fakeClient.Get(t.Context(), client.ObjectKeyFromObject(resource), updatedResource)

	// Check ProvisioningFailed condition is False (failure)
	cond := conditions.Get(&updatedResource.Status, conditionTypeProvisioningFailed)
	g.Expect(cond).ToNot(BeNil())
	g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
	g.Expect(cond.Reason).To(Equal("ReconciliationFailed"))
	g.Expect(cond.Message).To(ContainSubstring("action failed"))
}

func TestReconcile_StatusUpdate_ObservedGeneration(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	// Register TestResource with the scheme
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
			Name:       "test-resource",
			Namespace:  "default",
			Generation: 42,
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(resource).
		WithStatusSubresource(resource).
		Build()

	action := func(_ context.Context, _ *reconciler.Request, _ *reconciler.Response) error {
		return nil
	}

	p, err := NewPipeline(fakeClient,
		WithFieldOwner("test-controller"),
		WithActions(action),
	)
	g.Expect(err).ToNot(HaveOccurred())

	req := &reconciler.Request{
		Client: fakeClient,
		Object: resource,
	}

	resp, err := p.run(t.Context(), req)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(resp).ToNot(BeNil())

	// Verify ObservedGeneration matches object generation
	updatedResource := &TestResource{}
	_ = fakeClient.Get(t.Context(), client.ObjectKeyFromObject(resource), updatedResource)
	g.Expect(updatedResource.Status.ObservedGeneration).To(Equal(int64(42)))

	// Also check condition has the same ObservedGeneration
	cond := conditions.Get(&updatedResource.Status, conditionTypeProvisioningFailed)
	g.Expect(cond).ToNot(BeNil())
	g.Expect(cond.ObservedGeneration).To(Equal(int64(42)))
}
