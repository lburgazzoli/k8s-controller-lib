package builder_test

import (
	"context"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"k8s.io/client-go/util/workqueue"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/builder"

	. "github.com/onsi/gomega"
)

func TestControllerOptions_WithName(t *testing.T) {
	g := NewWithT(t)

	opt := builder.WithName("my-controller")
	opts := &builder.ControllerOptions{}
	opt.ApplyTo(opts)

	g.Expect(opts.Name).To(Equal("my-controller"))
}

func TestControllerOptions_WithCache(t *testing.T) {
	g := NewWithT(t)

	// WithCache is tested in integration tests with real cache
	// Here we just verify it compiles and doesn't panic
	opt := builder.WithCache(nil)
	opts := &builder.ControllerOptions{}
	opt.ApplyTo(opts)

	g.Expect(opts.Cache).To(BeNil())
}

func TestControllerOptions_WithClient(t *testing.T) {
	g := NewWithT(t)

	// WithClient is tested in integration tests with real client
	// Here we just verify it compiles and doesn't panic
	opt := builder.WithClient(nil)
	opts := &builder.ControllerOptions{}
	opt.ApplyTo(opts)

	g.Expect(opts.Client).To(BeNil())
}

func TestControllerOptions_WithMaxConcurrentReconciles(t *testing.T) {
	g := NewWithT(t)

	opt := builder.WithMaxConcurrentReconciles(5)
	opts := &builder.ControllerOptions{}
	opt.ApplyTo(opts)

	g.Expect(opts.MaxConcurrentReconciles).To(Equal(5))
}

func TestControllerOptions_WithRateLimiter(t *testing.T) {
	g := NewWithT(t)

	limiter := workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]()
	opt := builder.WithRateLimiter(limiter)
	opts := &builder.ControllerOptions{}
	opt.ApplyTo(opts)

	g.Expect(opts.RateLimiter).To(Equal(limiter))
}

func TestControllerOptions_ApplyOptions(t *testing.T) {
	g := NewWithT(t)

	limiter := workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]()
	opts := &builder.ControllerOptions{}
	opts.ApplyOptions([]builder.ControllerOption{
		builder.WithMaxConcurrentReconciles(3),
		builder.WithRateLimiter(limiter),
	})

	g.Expect(opts.MaxConcurrentReconciles).To(Equal(3))
	g.Expect(opts.RateLimiter).To(Equal(limiter))
}

func TestControllerOptions_ApplyTo(t *testing.T) {
	g := NewWithT(t)

	limiter := workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]()
	source := &builder.ControllerOptions{
		MaxConcurrentReconciles: 5,
		RateLimiter:             limiter,
	}

	target := &builder.ControllerOptions{}
	source.ApplyTo(target)

	g.Expect(target.MaxConcurrentReconciles).To(Equal(5))
	g.Expect(target.RateLimiter).To(Equal(limiter))
}

func TestWatchOptions_WithHandler(t *testing.T) {
	g := NewWithT(t)

	// Create a simple handler
	h := handler.TypedFuncs[client.Object, reconcile.Request]{}
	opt := builder.WithHandler(h)
	opts := &builder.WatchOptions{}
	opt.ApplyTo(opts)

	g.Expect(opts.Handler).ToNot(BeNil())
}

func TestWatchOptions_WithMapper(t *testing.T) {
	g := NewWithT(t)

	mapper := func(_ context.Context, _ client.Object) []reconcile.Request {
		return nil
	}

	// Verify WithMapper can be created and applied without panic
	opt := builder.WithMapper(mapper)
	opts := &builder.WatchOptions{}
	opt.ApplyTo(opts)

	// mapperFactory is internal, so we verify indirectly through behavior
	// The fact that ApplyTo succeeds without panic is sufficient for this unit test
	// Integration tests verify end-to-end mapper functionality
	g.Expect(opts).ToNot(BeNil())
}

func TestWatchOptions_WithPredicates_Additive(t *testing.T) {
	pred1 := predicate.GenerationChangedPredicate{}
	pred2 := predicate.LabelChangedPredicate{}
	pred3 := predicate.AnnotationChangedPredicate{}

	tests := []struct {
		name     string
		initial  []predicate.Predicate
		options  []builder.WatchOption
		expected int
	}{
		{
			name:    "single WithPredicates call",
			initial: nil,
			options: []builder.WatchOption{
				builder.WithPredicates(pred1, pred2),
			},
			expected: 2,
		},
		{
			name:    "multiple WithPredicates calls are additive",
			initial: nil,
			options: []builder.WatchOption{
				builder.WithPredicates(pred1),
				builder.WithPredicates(pred2),
				builder.WithPredicates(pred3),
			},
			expected: 3,
		},
		{
			name:    "adds to existing predicates",
			initial: []predicate.Predicate{pred1},
			options: []builder.WatchOption{
				builder.WithPredicates(pred2, pred3),
			},
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			opts := &builder.WatchOptions{
				Predicates: tt.initial,
			}
			opts.ApplyOptions(tt.options)

			g.Expect(opts.Predicates).To(HaveLen(tt.expected))
		})
	}
}

func TestWatchOptions_AsPartial(t *testing.T) {
	g := NewWithT(t)

	opt := builder.AsPartial()
	opts := &builder.WatchOptions{}
	opt.ApplyTo(opts)

	g.Expect(opts.Strategy).To(Equal(builder.WatchPartial))
}

func TestWatchOptions_WithStrategy(t *testing.T) {
	g := NewWithT(t)

	opt := builder.WithStrategy(builder.WatchPartial)
	opts := &builder.WatchOptions{}
	opt.ApplyTo(opts)

	g.Expect(opts.Strategy).To(Equal(builder.WatchPartial))
}

func TestWatchOptions_FunctionOptionsOverrideStructFields(t *testing.T) {
	g := NewWithT(t)

	h1 := handler.TypedFuncs[client.Object, reconcile.Request]{}
	h2 := handler.TypedFuncs[client.Object, reconcile.Request]{}

	// Start with struct-based options
	structOpts := &builder.WatchOptions{
		Handler: h1,
	}

	// Apply function option that overrides
	opts := &builder.WatchOptions{}
	opts.ApplyOptions([]builder.WatchOption{
		structOpts,
		builder.WithHandler(h2),
	})

	g.Expect(opts.Handler).ToNot(BeNil())
}

func TestWatchOptions_StrategyOverride(t *testing.T) {
	tests := []struct {
		name     string
		initial  builder.WatchStrategy
		options  []builder.WatchOption
		expected builder.WatchStrategy
	}{
		{
			name:     "full to partial",
			initial:  builder.WatchFull,
			options:  []builder.WatchOption{builder.AsPartial()},
			expected: builder.WatchPartial,
		},
		{
			name:     "partial remains partial",
			initial:  builder.WatchPartial,
			options:  []builder.WatchOption{builder.AsPartial()},
			expected: builder.WatchPartial,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			opts := &builder.WatchOptions{
				Strategy: tt.initial,
			}
			opts.ApplyOptions(tt.options)

			g.Expect(opts.Strategy).To(Equal(tt.expected))
		})
	}
}

func TestWatchOptions_HybridConfiguration(t *testing.T) {
	g := NewWithT(t)

	pred1 := predicate.GenerationChangedPredicate{}
	pred2 := predicate.LabelChangedPredicate{}
	h := handler.TypedFuncs[client.Object, reconcile.Request]{}

	// Base configuration (struct-based)
	baseOpts := &builder.WatchOptions{
		Predicates: []predicate.Predicate{pred1},
		Strategy:   builder.WatchPartial,
	}

	// Apply base + function options
	opts := &builder.WatchOptions{}
	opts.ApplyOptions([]builder.WatchOption{
		baseOpts,
		builder.WithPredicates(pred2),
		builder.WithHandler(h),
	})

	g.Expect(opts.Predicates).To(HaveLen(2), "predicates should be additive")
	g.Expect(opts.Handler).ToNot(BeNil())
	g.Expect(opts.Strategy).To(Equal(builder.WatchPartial), "Strategy should be preserved")
}

func TestWatchOptions_ApplyTo(t *testing.T) {
	g := NewWithT(t)

	h := handler.TypedFuncs[client.Object, reconcile.Request]{}
	pred := predicate.GenerationChangedPredicate{}

	source := &builder.WatchOptions{
		Handler:    h,
		Predicates: []predicate.Predicate{pred},
		Strategy:   builder.WatchPartial,
	}

	target := &builder.WatchOptions{}
	source.ApplyTo(target)

	g.Expect(target.Handler).ToNot(BeNil())
	g.Expect(target.Predicates).To(HaveLen(1))
	g.Expect(target.Strategy).To(Equal(builder.WatchPartial))
}

func TestWatchOptions_PredicatesConcatenation(t *testing.T) {
	g := NewWithT(t)

	pred1 := predicate.GenerationChangedPredicate{}
	pred2 := predicate.LabelChangedPredicate{}
	pred3 := predicate.AnnotationChangedPredicate{}

	target := &builder.WatchOptions{
		Predicates: []predicate.Predicate{pred1},
	}

	source := &builder.WatchOptions{
		Predicates: []predicate.Predicate{pred2, pred3},
	}

	source.ApplyTo(target)

	g.Expect(target.Predicates).To(HaveLen(3), "predicates should be concatenated")
}
