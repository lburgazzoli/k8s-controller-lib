//nolint:testpackage // White-box tests verify lifecycle ordering internals.
package hierarchical

import (
	"context"
	"errors"
	"testing"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/manager"

	. "github.com/onsi/gomega"
)

func TestLifecycleStopsControllersBeforeCache(t *testing.T) {
	g := NewWithT(t)
	lifetime := newLifecycle()
	started := make(chan string, 2)
	stopped := make(chan string, 2)

	go func() {
		_ = lifetime.run(t.Context(), controllerGroup, blockingComponent("controller", started, stopped))
	}()
	go func() {
		_ = lifetime.run(t.Context(), cacheGroup, blockingComponent("cache", started, stopped))
	}()

	g.Eventually(started).Should(Receive())
	g.Eventually(started).Should(Receive())
	g.Expect(lifetime.stop(t.Context())).To(Succeed())
	g.Expect(stopped).To(Receive(Equal("controller")))
	g.Expect(stopped).To(Receive(Equal("cache")))
	g.Expect(lifetime.stop(t.Context())).To(Succeed())
}

func TestLifecycleStopTimeoutContinuesShutdown(t *testing.T) {
	g := NewWithT(t)
	lifetime := newLifecycle()
	started := make(chan struct{})
	release := make(chan struct{})

	go func() {
		_ = lifetime.run(t.Context(), controllerGroup, func(ctx context.Context) error {
			close(started)
			<-ctx.Done()
			<-release

			return nil
		})
	}()
	g.Eventually(started).Should(BeClosed())

	stopCtx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	t.Cleanup(cancel)
	err := lifetime.stop(stopCtx)
	g.Expect(err).To(MatchError(ContainSubstring("stop hierarchical cluster")))
	g.Expect(errors.Is(err, context.DeadlineExceeded)).To(BeTrue())
	close(release)
	g.Expect(lifetime.stop(t.Context())).To(Succeed())
}

func TestLifecycleRunnablePreservesLeaderElectionAndWarmup(t *testing.T) {
	g := NewWithT(t)
	runnable := &warmupRunnable{warmup: make(chan struct{}, 1)}
	wrapper := &lifecycleRunnable{Runnable: runnable, lifecycle: newLifecycle()}

	g.Expect(wrapper.NeedLeaderElection()).To(BeFalse())
	g.Expect(wrapper.Warmup(t.Context())).To(Succeed())
	g.Expect(runnable.warmup).To(Receive())
}

func TestLifecycleCacheIsReadyAfterStop(t *testing.T) {
	g := NewWithT(t)
	lifetime := newLifecycle()
	delegate := &recordingCache{}
	c := &lifecycleCache{Cache: delegate, lifecycle: lifetime}

	g.Expect(lifetime.stop(t.Context())).To(Succeed())
	g.Expect(c.WaitForCacheSync(t.Context())).To(BeTrue())
	g.Expect(delegate.calls).To(BeEmpty())
}

func TestLifecycleSuppressesCancellationErrors(t *testing.T) {
	g := NewWithT(t)
	lifetime := newLifecycle()
	started := make(chan struct{})
	result := make(chan error, 1)

	go func() {
		result <- lifetime.run(t.Context(), controllerGroup, func(ctx context.Context) error {
			close(started)
			<-ctx.Done()

			return context.Canceled
		})
	}()
	g.Eventually(started).Should(BeClosed())
	g.Expect(lifetime.stop(t.Context())).To(Succeed())
	g.Expect(result).To(Receive(Succeed()))
}

func blockingComponent(
	name string,
	started chan<- string,
	stopped chan<- string,
) func(context.Context) error {
	return func(ctx context.Context) error {
		started <- name
		<-ctx.Done()
		stopped <- name

		return nil
	}
}

type warmupRunnable struct {
	warmup chan struct{}
}

func (r *warmupRunnable) Start(context.Context) error {
	return nil
}

func (r *warmupRunnable) NeedLeaderElection() bool {
	return false
}

func (r *warmupRunnable) Warmup(context.Context) error {
	r.warmup <- struct{}{}

	return nil
}

var (
	_ manager.Runnable               = (*warmupRunnable)(nil)
	_ manager.LeaderElectionRunnable = (*warmupRunnable)(nil)
)
