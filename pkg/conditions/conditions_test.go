package conditions

import (
	"testing"

	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMarkConditions(t *testing.T) {
	tests := []struct {
		name           string
		markFunc       func(Accessor, string, ...ConditionOption)
		expectedStatus metav1.ConditionStatus
	}{
		{
			name:           "MarkTrue sets status to True",
			markFunc:       MarkTrue,
			expectedStatus: metav1.ConditionTrue,
		},
		{
			name:           "MarkFalse sets status to False",
			markFunc:       MarkFalse,
			expectedStatus: metav1.ConditionFalse,
		},
		{
			name:           "MarkUnknown sets status to Unknown",
			markFunc:       MarkUnknown,
			expectedStatus: metav1.ConditionUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			conditions := make([]metav1.Condition, 0)
			accessor := NewAccessor(&conditions)

			tt.markFunc(accessor, "TestCondition",
				WithReason("TestReason"),
				WithMessage("TestMessage"))

			g.Expect(conditions).To(HaveLen(1))
			g.Expect(conditions[0]).To(MatchFields(IgnoreExtras, Fields{
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
		opts           []ConditionOption
		expectedFields Fields
	}{
		{
			name: "WithReason sets reason",
			opts: []ConditionOption{
				WithReason("TestReason"),
			},
			expectedFields: Fields{
				"Reason": Equal("TestReason"),
			},
		},
		{
			name: "WithMessage sets message",
			opts: []ConditionOption{
				WithMessage("TestMessage"),
			},
			expectedFields: Fields{
				"Message": Equal("TestMessage"),
			},
		},
		{
			name: "WithObservedGeneration sets observed generation",
			opts: []ConditionOption{
				WithObservedGeneration(5),
			},
			expectedFields: Fields{
				"ObservedGeneration": Equal(int64(5)),
			},
		},
		{
			name: "Multiple options combined",
			opts: []ConditionOption{
				WithReason("Combined"),
				WithMessage("Message"),
				WithObservedGeneration(10),
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

			conditions := make([]metav1.Condition, 0)
			accessor := NewAccessor(&conditions)

			Set(accessor, "TestCondition", metav1.ConditionTrue, tt.opts...)

			g.Expect(conditions).To(
				HaveLen(1),
			)
			g.Expect(conditions[0]).To(
				MatchFields(IgnoreExtras, tt.expectedFields),
			)
		})
	}
}

func TestGet(t *testing.T) {
	g := NewWithT(t)

	conditions := []metav1.Condition{
		{Type: "Ready", Status: metav1.ConditionTrue},
		{Type: "Available", Status: metav1.ConditionFalse},
	}
	accessor := NewAccessor(&conditions)

	ready := Get(accessor, "Ready")
	g.Expect(ready).ToNot(BeNil())
	g.Expect(ready).To(PointTo(MatchFields(IgnoreExtras, Fields{
		"Type":   Equal("Ready"),
		"Status": Equal(metav1.ConditionTrue),
	})))

	notFound := Get(accessor, "NotExists")
	g.Expect(notFound).To(BeNil())
}

func TestHas(t *testing.T) {
	g := NewWithT(t)

	conditions := []metav1.Condition{
		{Type: "Ready", Status: metav1.ConditionTrue},
	}
	accessor := NewAccessor(&conditions)

	g.Expect(Has(accessor, "Ready")).To(BeTrue())
	g.Expect(Has(accessor, "NotExists")).To(BeFalse())
}

func TestIsTrue(t *testing.T) {
	g := NewWithT(t)

	conditions := []metav1.Condition{
		{Type: "Ready", Status: metav1.ConditionTrue},
		{Type: "Available", Status: metav1.ConditionFalse},
	}
	accessor := NewAccessor(&conditions)

	g.Expect(IsTrue(accessor, "Ready")).To(BeTrue())
	g.Expect(IsTrue(accessor, "Available")).To(BeFalse())
	g.Expect(IsTrue(accessor, "NotExists")).To(BeFalse())
}

func TestIsFalse(t *testing.T) {
	g := NewWithT(t)

	conditions := []metav1.Condition{
		{Type: "Ready", Status: metav1.ConditionTrue},
		{Type: "Available", Status: metav1.ConditionFalse},
	}
	accessor := NewAccessor(&conditions)

	g.Expect(IsFalse(accessor, "Available")).To(BeTrue())
	g.Expect(IsFalse(accessor, "Ready")).To(BeFalse())
	g.Expect(IsFalse(accessor, "NotExists")).To(BeFalse())
}

func TestRemove(t *testing.T) {
	g := NewWithT(t)

	conditions := []metav1.Condition{
		{Type: "Ready", Status: metav1.ConditionTrue},
		{Type: "Available", Status: metav1.ConditionFalse},
	}
	accessor := NewAccessor(&conditions)

	Remove(accessor, "Ready")

	g.Expect(conditions).To(HaveLen(1))
	g.Expect(conditions[0].Type).To(Equal("Available"))
}

func TestFirstFalse(t *testing.T) {
	g := NewWithT(t)

	conditions := []metav1.Condition{
		{Type: "DatabaseReady", Status: metav1.ConditionTrue},
		{Type: "CacheReady", Status: metav1.ConditionFalse, Reason: "Unavailable"},
		{Type: "APIReady", Status: metav1.ConditionFalse, Reason: "Error"},
	}
	accessor := NewAccessor(&conditions)

	result := FirstFalse(accessor, []string{"DatabaseReady", "CacheReady", "APIReady"})

	g.Expect(result).ToNot(BeNil())
	g.Expect(result).To(PointTo(MatchFields(IgnoreExtras, Fields{
		"Type":   Equal("CacheReady"),
		"Reason": Equal("Unavailable"),
	})))
}

func TestFirstFalse_NotFound(t *testing.T) {
	g := NewWithT(t)

	conditions := []metav1.Condition{
		{Type: "Ready", Status: metav1.ConditionTrue},
	}
	accessor := NewAccessor(&conditions)

	result := FirstFalse(accessor, []string{"Ready"})
	g.Expect(result).To(BeNil())
}

func TestFirstUnknown(t *testing.T) {
	g := NewWithT(t)

	conditions := []metav1.Condition{
		{Type: "DatabaseReady", Status: metav1.ConditionTrue},
		{Type: "CacheReady", Status: metav1.ConditionUnknown, Reason: "Initializing"},
	}
	accessor := NewAccessor(&conditions)

	result := FirstUnknown(accessor, []string{"DatabaseReady", "CacheReady"})

	g.Expect(result).ToNot(BeNil())
	g.Expect(result).To(PointTo(MatchFields(IgnoreExtras, Fields{
		"Type":   Equal("CacheReady"),
		"Reason": Equal("Initializing"),
	})))
}

func TestFirstUnknown_MissingCondition(t *testing.T) {
	g := NewWithT(t)

	conditions := []metav1.Condition{
		{Type: "DatabaseReady", Status: metav1.ConditionTrue},
	}
	accessor := NewAccessor(&conditions)

	result := FirstUnknown(accessor, []string{"DatabaseReady", "MissingCondition"})

	g.Expect(result).To(BeNil())
}

func TestFirstUnknown_NotFound(t *testing.T) {
	g := NewWithT(t)

	conditions := []metav1.Condition{
		{Type: "Ready", Status: metav1.ConditionTrue},
	}
	accessor := NewAccessor(&conditions)

	result := FirstUnknown(accessor, []string{"Ready"})
	g.Expect(result).To(BeNil())
}
