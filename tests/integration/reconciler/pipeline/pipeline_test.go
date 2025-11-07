package pipeline_test

import (
	"context"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/status"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// TestResource is a custom resource type with status for testing status updates.
type TestResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   TestResourceSpec `json:"spec"`
	Status status.Status    `json:"status"`
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

// TestResourceList is a list of TestResource objects.
type TestResourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []TestResource `json:"items"`
}

func (r *TestResourceList) DeepCopyObject() runtime.Object {
	if r == nil {
		return nil
	}
	out := new(TestResourceList)
	r.DeepCopyInto(out)
	return out
}

func (r *TestResourceList) DeepCopyInto(out *TestResourceList) {
	*out = *r
	out.TypeMeta = r.TypeMeta
	r.ListMeta.DeepCopyInto(&out.ListMeta)
	if r.Items != nil {
		out.Items = make([]TestResource, len(r.Items))
		for i := range r.Items {
			r.Items[i].DeepCopyInto(&out.Items[i])
		}
	}
}

// dummyReconciler is a no-op reconciler for testing.
type dummyReconciler struct{}

func (r *dummyReconciler) Reconcile(
	ctx context.Context,
	req reconcile.Request,
) (reconcile.Result, error) {
	return reconcile.Result{}, nil
}
