package result_test

import (
	"testing"
	"time"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/reconciler/result"

	. "github.com/onsi/gomega"
)

func TestRequeueAfter(t *testing.T) {
	g := NewWithT(t)

	res := result.RequeueAfter(30 * time.Second)

	g.Expect(res.RequeueAfter).To(Equal(30 * time.Second))
}

func TestRequeueIf_ConditionTrue(t *testing.T) {
	g := NewWithT(t)

	res := result.RequeueIf(func() bool {
		return true
	})

	g.Expect(res.RequeueAfter).To(Equal(result.DefaultRequeueDelay))
}

func TestRequeueIf_ConditionFalse(t *testing.T) {
	g := NewWithT(t)

	res := result.RequeueIf(func() bool {
		return false
	})

	g.Expect(res.RequeueAfter).To(Equal(time.Duration(0)))
}

func TestRequeueIf_WithCustomDelay(t *testing.T) {
	g := NewWithT(t)

	res := result.RequeueIf(func() bool {
		return true
	}).After(10 * time.Second)

	g.Expect(res.RequeueAfter).To(Equal(10 * time.Second))
}

func TestRequeueAfter_WithAfter(t *testing.T) {
	g := NewWithT(t)

	res := result.RequeueAfter(5 * time.Second).After(15 * time.Second)

	g.Expect(res.RequeueAfter).To(Equal(15 * time.Second))
}

func TestSuccess(t *testing.T) {
	g := NewWithT(t)

	res := result.Success()

	g.Expect(res.Requeue).To(BeFalse())
	g.Expect(res.RequeueAfter).To(Equal(time.Duration(0)))
}
