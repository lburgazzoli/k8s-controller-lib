package conditions_test

import (
	"testing"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/conditions"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

func TestMarkConditions(t *testing.T) {
	tests := []struct {
		name           string
		markFunc       func(conditions.Accessor, string, ...conditions.ConditionOption)
		expectedStatus metav1.ConditionStatus
	}{
		{
			name:           "MarkTrue sets status to True",
			markFunc:       conditions.MarkTrue,
			expectedStatus: metav1.ConditionTrue,
		},
		{
			name:           "MarkFalse sets status to False",
			markFunc:       conditions.MarkFalse,
			expectedStatus: metav1.ConditionFalse,
		},
		{
			name:           "MarkUnknown sets status to Unknown",
			markFunc:       conditions.MarkUnknown,
			expectedStatus: metav1.ConditionUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			conditionsList := make([]metav1.Condition, 0)
			accessor := conditions.NewAccessor(&conditionsList)

			tt.markFunc(accessor, "TestCondition",
				conditions.WithReason("TestReason"),
				conditions.WithMessage("TestMessage"))

			g.Expect(conditionsList).To(HaveLen(1))
			g.Expect(conditionsList[0]).To(MatchFields(IgnoreExtras, Fields{
				"Status":  Equal(tt.expectedStatus),
				"Reason":  Equal("TestReason"),
				"Message": Equal("TestMessage"),
			}))
		})
	}
}

func TestConditionOptions(t *testing.T) {
	tests := []struct {
		name           string
		opts           []conditions.ConditionOption
		expectedFields Fields
	}{
		{
			name: "WithReason sets reason",
			opts: []conditions.ConditionOption{
				conditions.WithReason("TestReason"),
			},
			expectedFields: Fields{
				"Reason": Equal("TestReason"),
			},
		},
		{
			name: "WithMessage sets message",
			opts: []conditions.ConditionOption{
				conditions.WithMessage("TestMessage"),
			},
			expectedFields: Fields{
				"Message": Equal("TestMessage"),
			},
		},
		{
			name: "WithObservedGeneration sets observed generation",
			opts: []conditions.ConditionOption{
				conditions.WithObservedGeneration(5),
			},
			expectedFields: Fields{
				"ObservedGeneration": Equal(int64(5)),
			},
		},
		{
			name: "Multiple options combined",
			opts: []conditions.ConditionOption{
				conditions.WithReason("Combined"),
				conditions.WithMessage("Message"),
				conditions.WithObservedGeneration(10),
			},
			expectedFields: Fields{
				"Reason":             Equal("Combined"),
				"Message":            Equal("Message"),
				"ObservedGeneration": Equal(int64(10)),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			conditionsList := make([]metav1.Condition, 0)
			accessor := conditions.NewAccessor(&conditionsList)

			conditions.Set(accessor, "TestCondition", metav1.ConditionTrue, tt.opts...)

			g.Expect(conditionsList).To(
				HaveLen(1),
			)
			g.Expect(conditionsList[0]).To(
				MatchFields(IgnoreExtras, tt.expectedFields),
			)
		})
	}
}

func TestGet(t *testing.T) {
	g := NewWithT(t)

	conditionsList := []metav1.Condition{
		{Type: "Ready", Status: metav1.ConditionTrue},
		{Type: "Available", Status: metav1.ConditionFalse},
	}
	accessor := conditions.NewAccessor(&conditionsList)

	ready := conditions.Get(accessor, "Ready")
	g.Expect(ready).ToNot(BeNil())
	g.Expect(ready).To(PointTo(MatchFields(IgnoreExtras, Fields{
		"Type":   Equal("Ready"),
		"Status": Equal(metav1.ConditionTrue),
	})))

	notFound := conditions.Get(accessor, "NotExists")
	g.Expect(notFound).To(BeNil())
}

func TestHas(t *testing.T) {
	g := NewWithT(t)

	conditionsList := []metav1.Condition{
		{Type: "Ready", Status: metav1.ConditionTrue},
	}
	accessor := conditions.NewAccessor(&conditionsList)

	g.Expect(conditions.Has(accessor, "Ready")).To(BeTrue())
	g.Expect(conditions.Has(accessor, "NotExists")).To(BeFalse())
}

func TestIsTrue(t *testing.T) {
	g := NewWithT(t)

	conditionsList := []metav1.Condition{
		{Type: "Ready", Status: metav1.ConditionTrue},
		{Type: "Available", Status: metav1.ConditionFalse},
	}
	accessor := conditions.NewAccessor(&conditionsList)

	g.Expect(conditions.IsTrue(accessor, "Ready")).To(BeTrue())
	g.Expect(conditions.IsTrue(accessor, "Available")).To(BeFalse())
	g.Expect(conditions.IsTrue(accessor, "NotExists")).To(BeFalse())
}

func TestIsFalse(t *testing.T) {
	g := NewWithT(t)

	conditionsList := []metav1.Condition{
		{Type: "Ready", Status: metav1.ConditionTrue},
		{Type: "Available", Status: metav1.ConditionFalse},
	}
	accessor := conditions.NewAccessor(&conditionsList)

	g.Expect(conditions.IsFalse(accessor, "Available")).To(BeTrue())
	g.Expect(conditions.IsFalse(accessor, "Ready")).To(BeFalse())
	g.Expect(conditions.IsFalse(accessor, "NotExists")).To(BeFalse())
}

func TestRemove(t *testing.T) {
	g := NewWithT(t)

	conditionsList := []metav1.Condition{
		{Type: "Ready", Status: metav1.ConditionTrue},
		{Type: "Available", Status: metav1.ConditionFalse},
	}
	accessor := conditions.NewAccessor(&conditionsList)

	conditions.Remove(accessor, "Ready")

	g.Expect(conditionsList).To(HaveLen(1))
	g.Expect(conditionsList[0].Type).To(Equal("Available"))
}

func TestFirstFalse(t *testing.T) {
	g := NewWithT(t)

	conditionsList := []metav1.Condition{
		{Type: "DatabaseReady", Status: metav1.ConditionTrue},
		{Type: "CacheReady", Status: metav1.ConditionFalse, Reason: "Unavailable"},
		{Type: "APIReady", Status: metav1.ConditionFalse, Reason: "Error"},
	}
	accessor := conditions.NewAccessor(&conditionsList)

	result := conditions.FirstFalse(accessor, []string{"DatabaseReady", "CacheReady", "APIReady"})

	g.Expect(result).ToNot(BeNil())
	g.Expect(result).To(PointTo(MatchFields(IgnoreExtras, Fields{
		"Type":   Equal("CacheReady"),
		"Reason": Equal("Unavailable"),
	})))
}

func TestFirstFalse_NotFound(t *testing.T) {
	g := NewWithT(t)

	conditionsList := []metav1.Condition{
		{Type: "Ready", Status: metav1.ConditionTrue},
	}
	accessor := conditions.NewAccessor(&conditionsList)

	result := conditions.FirstFalse(accessor, []string{"Ready"})
	g.Expect(result).To(BeNil())
}

func TestFirstUnknown(t *testing.T) {
	g := NewWithT(t)

	conditionsList := []metav1.Condition{
		{Type: "DatabaseReady", Status: metav1.ConditionTrue},
		{Type: "CacheReady", Status: metav1.ConditionUnknown, Reason: "Initializing"},
	}
	accessor := conditions.NewAccessor(&conditionsList)

	result := conditions.FirstUnknown(accessor, []string{"DatabaseReady", "CacheReady"})

	g.Expect(result).ToNot(BeNil())
	g.Expect(result).To(PointTo(MatchFields(IgnoreExtras, Fields{
		"Type":   Equal("CacheReady"),
		"Reason": Equal("Initializing"),
	})))
}

func TestFirstUnknown_MissingCondition(t *testing.T) {
	g := NewWithT(t)

	conditionsList := []metav1.Condition{
		{Type: "DatabaseReady", Status: metav1.ConditionTrue},
	}
	accessor := conditions.NewAccessor(&conditionsList)

	result := conditions.FirstUnknown(accessor, []string{"DatabaseReady", "MissingCondition"})

	g.Expect(result).To(BeNil())
}

func TestFirstUnknown_NotFound(t *testing.T) {
	g := NewWithT(t)

	conditionsList := []metav1.Condition{
		{Type: "Ready", Status: metav1.ConditionTrue},
	}
	accessor := conditions.NewAccessor(&conditionsList)

	result := conditions.FirstUnknown(accessor, []string{"Ready"})
	g.Expect(result).To(BeNil())
}
