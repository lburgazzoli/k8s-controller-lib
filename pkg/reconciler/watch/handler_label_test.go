package watch_test

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/reconciler/watch"

	. "github.com/onsi/gomega"
)

func TestByLabel_Found(t *testing.T) {
	g := NewWithT(t)

	mapper := watch.ByLabel("my-label")

	obj := &metav1.PartialObjectMetadata{}
	obj.SetLabels(map[string]string{"my-label": "owner-name"})

	requests := mapper(t.Context(), obj)

	g.Expect(requests).To(HaveLen(1))
	g.Expect(requests[0].Name).To(Equal("owner-name"))
}

func TestByLabel_NotFound(t *testing.T) {
	g := NewWithT(t)

	mapper := watch.ByLabel("my-label")

	obj := &metav1.PartialObjectMetadata{}
	obj.SetLabels(map[string]string{"other": "value"})

	requests := mapper(t.Context(), obj)

	g.Expect(requests).To(BeNil())
}

func TestByLabel_NoLabels(t *testing.T) {
	g := NewWithT(t)

	mapper := watch.ByLabel("my-label")

	obj := &metav1.PartialObjectMetadata{}

	requests := mapper(t.Context(), obj)

	g.Expect(requests).To(BeNil())
}
