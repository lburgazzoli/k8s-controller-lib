package conditions_test

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/conditions"

	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

func TestManagerComputeAllHealthy(t *testing.T) {
	g := NewWithT(t)

	var conds []metav1.Condition
	a := conditions.NewAccessor(&conds)

	m := conditions.NewManager(
		conditions.ConditionTypeReady,
		conditions.PositivePolarity(conditions.ConditionTypeAvailable),
		conditions.PositivePolarity("ProvisioningSucceeded"),
		conditions.NegativePolarity(conditions.ConditionTypeDegraded),
	)

	conditions.MarkTrue(a, conditions.ConditionTypeAvailable, conditions.WithReason("Reconciled"))
	conditions.MarkTrue(a, "ProvisioningSucceeded", conditions.WithReason("Reconciled"))
	conditions.MarkFalse(a, conditions.ConditionTypeDegraded, conditions.WithReason("NotDegraded"))

	m.Compute(a, 1)

	g.Expect(conditions.Get(a, conditions.ConditionTypeReady)).To(PointTo(MatchFields(IgnoreExtras, Fields{
		"Status":  Equal(metav1.ConditionTrue),
		"Reason":  Equal(conditions.ConditionTypeReady),
		"Message": Equal("All conditions met"),
	})))
}

func TestManagerComputePositiveFalse(t *testing.T) {
	g := NewWithT(t)

	var conds []metav1.Condition
	a := conditions.NewAccessor(&conds)

	m := conditions.NewManager(
		conditions.ConditionTypeReady,
		conditions.PositivePolarity(conditions.ConditionTypeAvailable),
		conditions.PositivePolarity("ProvisioningSucceeded"),
	)

	conditions.MarkTrue(a, conditions.ConditionTypeAvailable, conditions.WithReason("Reconciled"))
	conditions.MarkFalse(a, "ProvisioningSucceeded",
		conditions.WithReason("ApplyFailed"),
		conditions.WithMessage("boom"))

	m.Compute(a, 1)

	g.Expect(conditions.Get(a, conditions.ConditionTypeReady)).To(PointTo(MatchFields(IgnoreExtras, Fields{
		"Status":  Equal(metav1.ConditionFalse),
		"Reason":  Equal("ApplyFailed"),
		"Message": Equal("boom"),
	})))
}

func TestManagerComputeNegativeTrue(t *testing.T) {
	g := NewWithT(t)

	var conds []metav1.Condition
	a := conditions.NewAccessor(&conds)

	m := conditions.NewManager(
		conditions.ConditionTypeReady,
		conditions.PositivePolarity(conditions.ConditionTypeAvailable),
		conditions.NegativePolarity(conditions.ConditionTypeDegraded),
	)

	conditions.MarkTrue(a, conditions.ConditionTypeAvailable, conditions.WithReason("Reconciled"))
	conditions.MarkTrue(a, conditions.ConditionTypeDegraded,
		conditions.WithReason("SomethingWrong"),
		conditions.WithMessage("partial failure"))

	m.Compute(a, 1)

	g.Expect(conditions.Get(a, conditions.ConditionTypeReady)).To(PointTo(MatchFields(IgnoreExtras, Fields{
		"Status":  Equal(metav1.ConditionFalse),
		"Reason":  Equal("SomethingWrong"),
		"Message": Equal("partial failure"),
	})))
}

func TestManagerComputeNegativeFalse(t *testing.T) {
	g := NewWithT(t)

	var conds []metav1.Condition
	a := conditions.NewAccessor(&conds)

	m := conditions.NewManager(
		conditions.ConditionTypeReady,
		conditions.NegativePolarity(conditions.ConditionTypeDegraded),
	)

	conditions.MarkFalse(a, conditions.ConditionTypeDegraded, conditions.WithReason("NotDegraded"))

	m.Compute(a, 1)

	g.Expect(conditions.Get(a, conditions.ConditionTypeReady)).To(PointTo(MatchFields(IgnoreExtras, Fields{
		"Status": Equal(metav1.ConditionTrue),
		"Reason": Equal(conditions.ConditionTypeReady),
	})))
}

func TestManagerComputeMissing(t *testing.T) {
	g := NewWithT(t)

	var conds []metav1.Condition
	a := conditions.NewAccessor(&conds)

	m := conditions.NewManager(
		conditions.ConditionTypeReady,
		conditions.PositivePolarity(conditions.ConditionTypeAvailable),
	)

	m.Compute(a, 1)

	g.Expect(conditions.Get(a, conditions.ConditionTypeReady)).To(PointTo(MatchFields(IgnoreExtras, Fields{
		"Status":  Equal(metav1.ConditionUnknown),
		"Reason":  Equal(conditions.ReasonConditionMissing),
		"Message": Equal("Available not yet reported"),
	})))
}

func TestManagerComputeMissingTakesPrecedence(t *testing.T) {
	g := NewWithT(t)

	var conds []metav1.Condition
	a := conditions.NewAccessor(&conds)

	m := conditions.NewManager(
		conditions.ConditionTypeReady,
		conditions.PositivePolarity("MissingCondition"),
		conditions.PositivePolarity(conditions.ConditionTypeAvailable),
	)

	conditions.MarkFalse(a, conditions.ConditionTypeAvailable,
		conditions.WithReason("Unavailable"),
		conditions.WithMessage("down"))

	m.Compute(a, 1)

	g.Expect(conditions.Get(a, conditions.ConditionTypeReady)).To(PointTo(MatchFields(IgnoreExtras, Fields{
		"Status":  Equal(metav1.ConditionUnknown),
		"Reason":  Equal(conditions.ReasonConditionMissing),
		"Message": Equal("MissingCondition not yet reported"),
	})))
}

func TestManagerComputeEmpty(t *testing.T) {
	g := NewWithT(t)

	var conds []metav1.Condition
	a := conditions.NewAccessor(&conds)

	m := conditions.NewManager(conditions.ConditionTypeReady)

	m.Compute(a, 1)

	g.Expect(conditions.Get(a, conditions.ConditionTypeReady)).To(PointTo(MatchFields(IgnoreExtras, Fields{
		"Status":  Equal(metav1.ConditionTrue),
		"Reason":  Equal(conditions.ConditionTypeReady),
		"Message": Equal("All conditions met"),
	})))
}

func TestManagerComputeObservedGeneration(t *testing.T) {
	g := NewWithT(t)

	var conds []metav1.Condition
	a := conditions.NewAccessor(&conds)

	m := conditions.NewManager(
		conditions.ConditionTypeReady,
		conditions.PositivePolarity(conditions.ConditionTypeAvailable),
	)

	conditions.MarkTrue(a, conditions.ConditionTypeAvailable, conditions.WithReason("Reconciled"))

	m.Compute(a, 42)

	g.Expect(conditions.Get(a, conditions.ConditionTypeReady)).To(PointTo(MatchFields(IgnoreExtras, Fields{
		"Status":             Equal(metav1.ConditionTrue),
		"ObservedGeneration": Equal(int64(42)),
	})))
}

func TestManagerComputeMultipleUnhealthy(t *testing.T) {
	g := NewWithT(t)

	var conds []metav1.Condition
	a := conditions.NewAccessor(&conds)

	m := conditions.NewManager(
		conditions.ConditionTypeReady,
		conditions.PositivePolarity(conditions.ConditionTypeAvailable),
		conditions.PositivePolarity("ProvisioningSucceeded"),
	)

	conditions.MarkFalse(a, conditions.ConditionTypeAvailable,
		conditions.WithReason("FirstError"),
		conditions.WithMessage("first problem"))
	conditions.MarkFalse(a, "ProvisioningSucceeded",
		conditions.WithReason("SecondError"),
		conditions.WithMessage("second problem"))

	m.Compute(a, 1)

	g.Expect(conditions.Get(a, conditions.ConditionTypeReady)).To(PointTo(MatchFields(IgnoreExtras, Fields{
		"Status":  Equal(metav1.ConditionFalse),
		"Reason":  Equal("FirstError"),
		"Message": Equal("first problem"),
	})))
}

func TestManagerComputeMixed(t *testing.T) {
	g := NewWithT(t)

	var conds []metav1.Condition
	a := conditions.NewAccessor(&conds)

	m := conditions.NewManager(
		conditions.ConditionTypeReady,
		conditions.PositivePolarity(conditions.ConditionTypeAvailable),
		conditions.NegativePolarity(conditions.ConditionTypeDegraded),
		conditions.PositivePolarity("ProvisioningSucceeded"),
	)

	conditions.MarkTrue(a, conditions.ConditionTypeAvailable, conditions.WithReason("Reconciled"))
	conditions.MarkFalse(a, conditions.ConditionTypeDegraded, conditions.WithReason("NotDegraded"))
	conditions.MarkTrue(a, "ProvisioningSucceeded", conditions.WithReason("Reconciled"))

	m.Compute(a, 1)

	g.Expect(conditions.Get(a, conditions.ConditionTypeReady)).To(PointTo(MatchFields(IgnoreExtras, Fields{
		"Status": Equal(metav1.ConditionTrue),
		"Reason": Equal(conditions.ConditionTypeReady),
	})))
}

func TestManagerMarkTrue(t *testing.T) {
	g := NewWithT(t)

	var conds []metav1.Condition
	a := conditions.NewAccessor(&conds)

	m := conditions.NewManager(conditions.ConditionTypeReady)

	m.MarkTrue(a, conditions.ConditionTypeAvailable,
		conditions.WithReason("Reconciled"),
		conditions.WithMessage("ok"),
		conditions.WithObservedGeneration(5))

	g.Expect(conditions.Get(a, conditions.ConditionTypeAvailable)).To(PointTo(MatchFields(IgnoreExtras, Fields{
		"Status":             Equal(metav1.ConditionTrue),
		"Reason":             Equal("Reconciled"),
		"Message":            Equal("ok"),
		"ObservedGeneration": Equal(int64(5)),
	})))
}

func TestManagerMarkFalse(t *testing.T) {
	g := NewWithT(t)

	var conds []metav1.Condition
	a := conditions.NewAccessor(&conds)

	m := conditions.NewManager(conditions.ConditionTypeReady)

	m.MarkFalse(a, conditions.ConditionTypeAvailable,
		conditions.WithReason("Blocked"),
		conditions.WithMessage("waiting"))

	g.Expect(conditions.Get(a, conditions.ConditionTypeAvailable)).To(PointTo(MatchFields(IgnoreExtras, Fields{
		"Status":  Equal(metav1.ConditionFalse),
		"Reason":  Equal("Blocked"),
		"Message": Equal("waiting"),
	})))
}

func TestManagerComputeUpdatesExisting(t *testing.T) {
	g := NewWithT(t)

	conds := []metav1.Condition{
		{Type: conditions.ConditionTypeReady, Status: metav1.ConditionFalse, Reason: "OldReason"},
	}
	a := conditions.NewAccessor(&conds)

	m := conditions.NewManager(conditions.ConditionTypeReady)

	m.Compute(a, 1)

	g.Expect(conditions.Get(a, conditions.ConditionTypeReady)).To(PointTo(MatchFields(IgnoreExtras, Fields{
		"Status": Equal(metav1.ConditionTrue),
		"Reason": Equal(conditions.ConditionTypeReady),
	})))
}
