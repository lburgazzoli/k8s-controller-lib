package conditions_test

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/conditions"

	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

func TestNewAccessor(t *testing.T) {
	g := NewWithT(t)

	conditionsList := make([]metav1.Condition, 0)
	accessor := conditions.NewAccessor(&conditionsList)

	g.Expect(accessor).ToNot(BeNil())
	g.Expect(accessor.GetConditions()).To(BeEmpty())
}

func TestAccessorGetConditions(t *testing.T) {
	g := NewWithT(t)

	conditionsList := []metav1.Condition{
		{
			Type:   "Ready",
			Status: metav1.ConditionTrue,
			Reason: "Initialized",
		},
	}

	accessor := conditions.NewAccessor(&conditionsList)
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

	conditionsList := make([]metav1.Condition, 0)
	accessor := conditions.NewAccessor(&conditionsList)

	newConditions := []metav1.Condition{
		{
			Type:   "Available",
			Status: metav1.ConditionFalse,
			Reason: "NotReady",
		},
	}

	accessor.SetConditions(newConditions)

	g.Expect(conditionsList).To(HaveLen(1))
	g.Expect(conditionsList[0]).To(MatchFields(IgnoreExtras, Fields{
		"Type":   Equal("Available"),
		"Status": Equal(metav1.ConditionFalse),
		"Reason": Equal("NotReady"),
	}))
}

func TestAccessorNilConditions(t *testing.T) {
	g := NewWithT(t)

	accessor := conditions.NewAccessor(nil)

	g.Expect(accessor.GetConditions()).To(BeNil())

	// SetConditions on nil should not panic
	accessor.SetConditions([]metav1.Condition{{Type: "Test"}})
	g.Expect(accessor.GetConditions()).To(BeNil())
}
