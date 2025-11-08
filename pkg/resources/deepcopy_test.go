package resources_test

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/resources"

	. "github.com/onsi/gomega"
)

func TestDeepCopySlice(t *testing.T) {
	g := NewWithT(t)

	original := []metav1.ManagedFieldsEntry{
		{
			Manager:   "test-manager",
			Operation: metav1.ManagedFieldsOperationApply,
		},
		{
			Manager:   "another-manager",
			Operation: metav1.ManagedFieldsOperationUpdate,
		},
	}

	copied := resources.DeepCopySlice(original)
	g.Expect(copied).ToNot(BeNil())
	g.Expect(copied).To(HaveLen(2))

	// Verify deep copy - values are equal
	g.Expect(copied[0].Manager).To(Equal("test-manager"))
	g.Expect(copied[0].Operation).To(Equal(metav1.ManagedFieldsOperationApply))
	g.Expect(copied[1].Manager).To(Equal("another-manager"))
	g.Expect(copied[1].Operation).To(Equal(metav1.ManagedFieldsOperationUpdate))

	// Verify deep copy - modifying copy doesn't affect original
	copied[0].Manager = "modified"
	g.Expect(original[0].Manager).To(Equal("test-manager"))
}

func TestDeepCopySliceNil(t *testing.T) {
	g := NewWithT(t)

	var original []metav1.ManagedFieldsEntry
	copied := resources.DeepCopySlice(original)
	g.Expect(copied).To(BeNil())
}

func TestDeepCopySliceEmpty(t *testing.T) {
	g := NewWithT(t)

	original := []metav1.ManagedFieldsEntry{}
	copied := resources.DeepCopySlice(original)
	g.Expect(copied).ToNot(BeNil())
	g.Expect(copied).To(BeEmpty())
}
