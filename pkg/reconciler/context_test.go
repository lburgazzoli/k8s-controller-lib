package reconciler_test

import (
	"context"
	"testing"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/reconciler"

	. "github.com/onsi/gomega"
)

func TestWithControllerName(t *testing.T) {
	g := NewWithT(t)

	ctx := context.Background()
	ctx = reconciler.WithControllerName(ctx, "test-controller")

	name, ok := reconciler.ControllerNameFromContext(ctx)
	g.Expect(ok).To(BeTrue())
	g.Expect(name).To(Equal("test-controller"))
}

func TestControllerNameFromContext_NotFound(t *testing.T) {
	g := NewWithT(t)

	ctx := context.Background()

	name, ok := reconciler.ControllerNameFromContext(ctx)
	g.Expect(ok).To(BeFalse())
	g.Expect(name).To(BeEmpty())
}

func TestControllerNameFromContext_OverwriteValue(t *testing.T) {
	g := NewWithT(t)

	ctx := context.Background()
	ctx = reconciler.WithControllerName(ctx, "controller-1")
	ctx = reconciler.WithControllerName(ctx, "controller-2")

	name, ok := reconciler.ControllerNameFromContext(ctx)
	g.Expect(ok).To(BeTrue())
	g.Expect(name).To(Equal("controller-2"))
}

func TestControllerNameFromContext_NilContext(t *testing.T) {
	g := NewWithT(t)

	//nolint:staticcheck
	name, ok := reconciler.ControllerNameFromContext(nil)
	g.Expect(ok).To(BeFalse())
	g.Expect(name).To(BeEmpty())
}
