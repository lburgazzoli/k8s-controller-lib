package status_test

import (
	"testing"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/status"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

type TestResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Status status.Status `json:"status"`
}

func TestAccessor_GetStatus(t *testing.T) {
	g := NewWithT(t)

	resource := &TestResource{
		Status: status.Status{
			ObservedGeneration: 5,
		},
	}

	accessor, err := status.NewAccessor(resource)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(accessor).ToNot(BeNil())

	// Test GetStatus
	s := accessor.GetStatus()
	g.Expect(s).ToNot(BeNil())
	g.Expect(s.ObservedGeneration).To(Equal(int64(5)))
}

func TestAccessor_SetStatus(t *testing.T) {
	g := NewWithT(t)

	resource := &TestResource{}

	accessor, err := status.NewAccessor(resource)
	g.Expect(err).ToNot(HaveOccurred())

	// Test SetStatus
	newStatus := &status.Status{
		ObservedGeneration: 10,
		Conditions: []metav1.Condition{
			{Type: "Ready", Status: metav1.ConditionTrue},
		},
	}
	accessor.SetStatus(newStatus)

	g.Expect(resource.Status.ObservedGeneration).To(Equal(int64(10)))
	g.Expect(resource.Status.Conditions).To(HaveLen(1))
	g.Expect(resource.Status.Conditions[0]).To(MatchFields(IgnoreExtras, Fields{
		"Type":   Equal("Ready"),
		"Status": Equal(metav1.ConditionTrue),
	}))
}

func TestAccessor_GetAndModify(t *testing.T) {
	g := NewWithT(t)

	resource := &TestResource{}

	accessor, err := status.NewAccessor(resource)
	g.Expect(err).ToNot(HaveOccurred())

	// Get and modify
	s := accessor.GetStatus()
	s.ObservedGeneration = 20
	s.Conditions = []metav1.Condition{
		{Type: "Available", Status: metav1.ConditionFalse},
	}

	// Verify changes are reflected in the resource
	g.Expect(resource.Status.ObservedGeneration).To(Equal(int64(20)))
	g.Expect(resource.Status.Conditions).To(HaveLen(1))
}

func TestAccessor_NotAPointer(t *testing.T) {
	g := NewWithT(t)

	resource := TestResource{}
	_, err := status.NewAccessor(resource)

	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("expected pointer"))
}

func TestAccessor_NoStatusField(t *testing.T) {
	g := NewWithT(t)

	type NoStatus struct {
		Name string
	}

	resource := &NoStatus{}
	_, err := status.NewAccessor(resource)

	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("does not have a Status field"))
}

func TestAccessor_WrongStatusType(t *testing.T) {
	g := NewWithT(t)

	type CustomStatus struct {
		Ready bool
	}

	type WrongStatus struct {
		Status CustomStatus
	}

	resource := &WrongStatus{}
	_, err := status.NewAccessor(resource)

	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("not of type status.Status"))
}
