package conditions

import (
	"testing"

	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAggregate_AllTrue(t *testing.T) {
	g := NewWithT(t)

	conditions := []metav1.Condition{
		{Type: "DatabaseReady", Status: metav1.ConditionTrue, Reason: "Connected"},
		{Type: "CacheReady", Status: metav1.ConditionTrue, Reason: "Connected"},
		{Type: "APIReady", Status: metav1.ConditionTrue, Reason: "Serving"},
	}
	accessor := NewAccessor(&conditions)

	Aggregate(accessor, "Ready", []string{"DatabaseReady", "CacheReady", "APIReady"})

	ready := Get(accessor, "Ready")
	g.Expect(ready).ToNot(BeNil())
	g.Expect(ready).To(PointTo(MatchFields(IgnoreExtras, Fields{
		"Status":  Equal(metav1.ConditionTrue),
		"Reason":  Equal("Ready"),
		"Message": Equal("Ready"),
	})))
}

func TestAggregate_OneFalse(t *testing.T) {
	g := NewWithT(t)

	conditions := []metav1.Condition{
		{Type: "DatabaseReady", Status: metav1.ConditionTrue, Reason: "Connected"},
		{Type: "CacheReady", Status: metav1.ConditionFalse, Reason: "Unavailable", Message: "Cache is down"},
		{Type: "APIReady", Status: metav1.ConditionTrue, Reason: "Serving"},
	}
	accessor := NewAccessor(&conditions)

	Aggregate(accessor, "Ready", []string{"DatabaseReady", "CacheReady", "APIReady"})

	ready := Get(accessor, "Ready")
	g.Expect(ready).ToNot(BeNil())
	g.Expect(ready).To(PointTo(MatchFields(IgnoreExtras, Fields{
		"Status":  Equal(metav1.ConditionFalse),
		"Reason":  Equal("Unavailable"),
		"Message": Equal("Cache is down"),
	})))
}

func TestAggregate_MultipleFalse_UsesFirst(t *testing.T) {
	g := NewWithT(t)

	conditions := []metav1.Condition{
		{Type: "DatabaseReady", Status: metav1.ConditionFalse, Reason: "DBError", Message: "Database error"},
		{Type: "CacheReady", Status: metav1.ConditionFalse, Reason: "CacheError", Message: "Cache error"},
	}
	accessor := NewAccessor(&conditions)

	Aggregate(accessor, "Ready", []string{"DatabaseReady", "CacheReady"})

	ready := Get(accessor, "Ready")
	g.Expect(ready).ToNot(BeNil())
	g.Expect(ready).To(PointTo(MatchFields(IgnoreExtras, Fields{
		"Status":  Equal(metav1.ConditionFalse),
		"Reason":  Equal("DBError"),
		"Message": Equal("Database error"),
	})))
}

func TestAggregate_OneUnknown(t *testing.T) {
	g := NewWithT(t)

	conditions := []metav1.Condition{
		{Type: "DatabaseReady", Status: metav1.ConditionTrue, Reason: "Connected"},
		{Type: "CacheReady", Status: metav1.ConditionUnknown, Reason: "Initializing", Message: "Cache is starting"},
	}
	accessor := NewAccessor(&conditions)

	Aggregate(accessor, "Ready", []string{"DatabaseReady", "CacheReady"})

	ready := Get(accessor, "Ready")
	g.Expect(ready).ToNot(BeNil())
	g.Expect(ready).To(PointTo(MatchFields(IgnoreExtras, Fields{
		"Status":  Equal(metav1.ConditionUnknown),
		"Reason":  Equal("Initializing"),
		"Message": Equal("Cache is starting"),
	})))
}

func TestAggregate_MissingCondition(t *testing.T) {
	g := NewWithT(t)

	conditions := []metav1.Condition{
		{Type: "DatabaseReady", Status: metav1.ConditionTrue, Reason: "Connected"},
	}
	accessor := NewAccessor(&conditions)

	Aggregate(accessor, "Ready", []string{"DatabaseReady", "MissingCondition"})

	ready := Get(accessor, "Ready")
	g.Expect(ready).ToNot(BeNil())
	g.Expect(ready).To(PointTo(MatchFields(IgnoreExtras, Fields{
		"Status":  Equal(metav1.ConditionTrue),
		"Reason":  Equal("Ready"),
		"Message": Equal("Ready"),
	})))
}

func TestAggregate_EmptyContributing(t *testing.T) {
	g := NewWithT(t)

	conditions := []metav1.Condition{}
	accessor := NewAccessor(&conditions)

	Aggregate(accessor, "Ready", []string{})

	ready := Get(accessor, "Ready")
	g.Expect(ready).ToNot(BeNil())
	g.Expect(ready).To(PointTo(MatchFields(IgnoreExtras, Fields{
		"Status":  Equal(metav1.ConditionTrue),
		"Reason":  Equal("Ready"),
		"Message": Equal("Ready"),
	})))
}

func TestAggregate_WithDefaultReason(t *testing.T) {
	g := NewWithT(t)

	conditions := []metav1.Condition{
		{Type: "DatabaseReady", Status: metav1.ConditionTrue, Reason: "Connected"},
		{Type: "CacheReady", Status: metav1.ConditionTrue, Reason: "Connected"},
	}
	accessor := NewAccessor(&conditions)

	Aggregate(accessor, "Ready", []string{"DatabaseReady", "CacheReady"},
		WithDefaultReason("AllHealthy"))

	ready := Get(accessor, "Ready")
	g.Expect(ready).ToNot(BeNil())
	g.Expect(ready).To(PointTo(MatchFields(IgnoreExtras, Fields{
		"Status":  Equal(metav1.ConditionTrue),
		"Reason":  Equal("AllHealthy"),
		"Message": Equal("Ready"),
	})))
}

func TestAggregate_WithDefaultMessage(t *testing.T) {
	g := NewWithT(t)

	conditions := []metav1.Condition{
		{Type: "DatabaseReady", Status: metav1.ConditionTrue, Reason: "Connected"},
	}
	accessor := NewAccessor(&conditions)

	Aggregate(accessor, "Ready", []string{"DatabaseReady"},
		WithDefaultMessage("All systems operational"))

	ready := Get(accessor, "Ready")
	g.Expect(ready).ToNot(BeNil())
	g.Expect(ready).To(PointTo(MatchFields(IgnoreExtras, Fields{
		"Status":  Equal(metav1.ConditionTrue),
		"Reason":  Equal("Ready"),
		"Message": Equal("All systems operational"),
	})))
}

func TestAggregate_WithDefaultReasonAndMessage(t *testing.T) {
	g := NewWithT(t)

	conditions := []metav1.Condition{}
	accessor := NewAccessor(&conditions)

	Aggregate(accessor, "Ready", []string{},
		WithDefaultReason("NoComponents"),
		WithDefaultMessage("No components configured"))

	ready := Get(accessor, "Ready")
	g.Expect(ready).ToNot(BeNil())
	g.Expect(ready).To(PointTo(MatchFields(IgnoreExtras, Fields{
		"Status":  Equal(metav1.ConditionTrue),
		"Reason":  Equal("NoComponents"),
		"Message": Equal("No components configured"),
	})))
}

func TestAggregate_FalseTakesPrecedenceOverUnknown(t *testing.T) {
	g := NewWithT(t)

	conditions := []metav1.Condition{
		{Type: "DatabaseReady", Status: metav1.ConditionUnknown, Reason: "Initializing"},
		{Type: "CacheReady", Status: metav1.ConditionFalse, Reason: "Unavailable", Message: "Cache is down"},
	}
	accessor := NewAccessor(&conditions)

	Aggregate(accessor, "Ready", []string{"DatabaseReady", "CacheReady"})

	ready := Get(accessor, "Ready")
	g.Expect(ready).ToNot(BeNil())
	g.Expect(ready).To(PointTo(MatchFields(IgnoreExtras, Fields{
		"Status":  Equal(metav1.ConditionFalse),
		"Reason":  Equal("Unavailable"),
		"Message": Equal("Cache is down"),
	})))
}

func TestAggregate_UnknownWithEmptyReasonUsesDefault(t *testing.T) {
	g := NewWithT(t)

	conditions := []metav1.Condition{
		{Type: "DatabaseReady", Status: metav1.ConditionTrue},
		{Type: "CacheReady", Status: metav1.ConditionUnknown, Reason: "", Message: ""},
	}
	accessor := NewAccessor(&conditions)

	Aggregate(accessor, "Ready", []string{"DatabaseReady", "CacheReady"})

	ready := Get(accessor, "Ready")
	g.Expect(ready).ToNot(BeNil())
	g.Expect(ready).To(PointTo(MatchFields(IgnoreExtras, Fields{
		"Status":  Equal(metav1.ConditionUnknown),
		"Reason":  Equal("Ready"),
		"Message": Equal("Ready"),
	})))
}

func TestAggregate_UpdatesExistingCondition(t *testing.T) {
	g := NewWithT(t)

	conditions := []metav1.Condition{
		{Type: "Ready", Status: metav1.ConditionFalse, Reason: "OldReason"},
		{Type: "DatabaseReady", Status: metav1.ConditionTrue, Reason: "Connected"},
	}
	accessor := NewAccessor(&conditions)

	Aggregate(accessor, "Ready", []string{"DatabaseReady"})

	ready := Get(accessor, "Ready")
	g.Expect(ready).ToNot(BeNil())
	g.Expect(ready).To(PointTo(MatchFields(IgnoreExtras, Fields{
		"Status": Equal(metav1.ConditionTrue),
		"Reason": Equal("Ready"),
	})))
}
