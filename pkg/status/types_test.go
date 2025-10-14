package status_test

import (
	"testing"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/conditions"
	"github.com/lburgazzoli/k8s-controller-lib/pkg/status"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

func TestStatusImplementsConditionsAccessor(t *testing.T) {
	g := NewWithT(t)

	s := &status.Status{}

	// Verify it implements conditions.NewAccessor
	var _ conditions.Accessor = s

	// Test GetConditions/SetConditions
	conds := []metav1.Condition{
		{Type: "Ready", Status: metav1.ConditionTrue},
	}
	s.SetConditions(conds)

	g.Expect(s.GetConditions()).To(HaveLen(1))
	g.Expect(s.GetConditions()[0]).To(MatchFields(IgnoreExtras, Fields{
		"Type":   Equal("Ready"),
		"Status": Equal(metav1.ConditionTrue),
	}))
}

func TestStatusObservedGeneration(t *testing.T) {
	g := NewWithT(t)

	s := &status.Status{}
	s.ObservedGeneration = 5

	g.Expect(s.ObservedGeneration).To(Equal(int64(5)))
}
