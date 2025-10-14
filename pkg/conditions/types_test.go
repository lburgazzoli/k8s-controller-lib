package conditions

import (
	"testing"

	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewAccessor(t *testing.T) {
	g := NewWithT(t)

	conditions := make([]metav1.Condition, 0)
	accessor := NewAccessor(&conditions)

	g.Expect(accessor).ToNot(BeNil())
	g.Expect(accessor.GetConditions()).To(BeEmpty())
}

func TestAccessorGetConditions(t *testing.T) {
	g := NewWithT(t)

	conditions := []metav1.Condition{
		{
			Type:   "Ready",
			Status: metav1.ConditionTrue,
			Reason: "Initialized",
		},
	}

	accessor := NewAccessor(&conditions)
	retrieved := accessor.GetConditions()

	g.Expect(retrieved).To(HaveLen(1))
	g.Expect(retrieved[0]).To(MatchFields(IgnoreExtras, Fields{
		"Type":   Equal("Ready"),
		"Status": Equal(metav1.ConditionTrue),
		"Reason": Equal("Initialized"),
	}))
}

func TestAccessorSetConditions(t *testing.T) {
	g := NewWithT(t)

	conditions := make([]metav1.Condition, 0)
	accessor := NewAccessor(&conditions)

	newConditions := []metav1.Condition{
		{
			Type:   "Available",
			Status: metav1.ConditionFalse,
			Reason: "NotReady",
		},
	}

	accessor.SetConditions(newConditions)

	g.Expect(conditions).To(HaveLen(1))
	g.Expect(conditions[0]).To(MatchFields(IgnoreExtras, Fields{
		"Type":   Equal("Available"),
		"Status": Equal(metav1.ConditionFalse),
		"Reason": Equal("NotReady"),
	}))
}

func TestAccessorNilConditions(t *testing.T) {
	g := NewWithT(t)

	accessor := NewAccessor(nil)

	g.Expect(accessor.GetConditions()).To(BeNil())

	// SetConditions on nil should not panic
	accessor.SetConditions([]metav1.Condition{{Type: "Test"}})
	g.Expect(accessor.GetConditions()).To(BeNil())
}
