package conditions_test

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/conditions"

	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

func TestMarkAvailable(t *testing.T) {
	g := NewWithT(t)

	conditionsList := make([]metav1.Condition, 0)
	accessor := conditions.NewAccessor(&conditionsList)

	conditions.MarkAvailable(accessor, "AllPodsReady")

	g.Expect(conditionsList).To(HaveLen(1))
	g.Expect(conditionsList[0]).To(MatchFields(IgnoreExtras, Fields{
		"Type":   Equal(conditions.ConditionTypeAvailable),
		"Status": Equal(metav1.ConditionTrue),
		"Reason": Equal("AllPodsReady"),
	}))
}

func TestMarkProgressing(t *testing.T) {
	g := NewWithT(t)

	conditionsList := make([]metav1.Condition, 0)
	accessor := conditions.NewAccessor(&conditionsList)

	conditions.MarkProgressing(accessor, "DeployingPods", "Deploying 3 of 5 pods")

	g.Expect(conditionsList).To(HaveLen(1))
	g.Expect(conditionsList[0]).To(MatchFields(IgnoreExtras, Fields{
		"Type":    Equal(conditions.ConditionTypeProgressing),
		"Status":  Equal(metav1.ConditionTrue),
		"Reason":  Equal("DeployingPods"),
		"Message": Equal("Deploying 3 of 5 pods"),
	}))
}

func TestMarkDegraded(t *testing.T) {
	g := NewWithT(t)

	conditionsList := make([]metav1.Condition, 0)
	accessor := conditions.NewAccessor(&conditionsList)

	conditions.MarkDegraded(accessor, "InsufficientReplicas", "Only 2 of 5 pods are running")

	g.Expect(conditionsList).To(HaveLen(1))
	g.Expect(conditionsList[0]).To(MatchFields(IgnoreExtras, Fields{
		"Type":    Equal(conditions.ConditionTypeDegraded),
		"Status":  Equal(metav1.ConditionTrue),
		"Reason":  Equal("InsufficientReplicas"),
		"Message": Equal("Only 2 of 5 pods are running"),
	}))
}
