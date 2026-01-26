/*
Copyright 2025 The k8s-controller-lib Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package builder

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"k8s.io/client-go/util/workqueue"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/util"
)

// ControllerOption configures a Controller during construction.
type ControllerOption = util.Option[ControllerOptions]

// ControllerOptions holds configuration for controller construction.
type ControllerOptions struct {
	Name                    string
	Cache                   cache.Cache
	Client                  client.Client
	MaxConcurrentReconciles int
	RateLimiter             workqueue.TypedRateLimiter[reconcile.Request]
}

// ApplyTo implements Option interface for ControllerOptions.
func (o *ControllerOptions) ApplyTo(target *ControllerOptions) {
	if o.Name != "" {
		target.Name = o.Name
	}

	if o.Cache != nil {
		target.Cache = o.Cache
	}

	if o.Client != nil {
		target.Client = o.Client
	}

	if o.MaxConcurrentReconciles > 0 {
		target.MaxConcurrentReconciles = o.MaxConcurrentReconciles
	}

	if o.RateLimiter != nil {
		target.RateLimiter = o.RateLimiter
	}
}

// ApplyOptions applies all given options to this ControllerOptions.
func (o *ControllerOptions) ApplyOptions(opts []ControllerOption) *ControllerOptions {
	for _, opt := range opts {
		opt.ApplyTo(o)
	}

	return o
}

// WithName sets the controller name for metrics and logging.
// If not provided, the name is automatically derived from the For() type.
func WithName(name string) ControllerOption {
	return util.FunctionalOption[ControllerOptions](func(opts *ControllerOptions) {
		opts.Name = name
	})
}

// WithCache sets a custom cache for the controller.
// If not provided, the manager's cache is used.
func WithCache(c cache.Cache) ControllerOption {
	return util.FunctionalOption[ControllerOptions](func(opts *ControllerOptions) {
		opts.Cache = c
	})
}

// WithClient sets a custom client for the controller.
// If not provided, the manager's client is used.
func WithClient(c client.Client) ControllerOption {
	return util.FunctionalOption[ControllerOptions](func(opts *ControllerOptions) {
		opts.Client = c
	})
}

// WithMaxConcurrentReconciles sets the maximum number of concurrent reconciles.
func WithMaxConcurrentReconciles(n int) ControllerOption {
	return util.FunctionalOption[ControllerOptions](func(opts *ControllerOptions) {
		opts.MaxConcurrentReconciles = n
	})
}

// WithRateLimiter sets the rate limiter for the controller.
func WithRateLimiter(limiter workqueue.TypedRateLimiter[reconcile.Request]) ControllerOption {
	return util.FunctionalOption[ControllerOptions](func(opts *ControllerOptions) {
		opts.RateLimiter = limiter
	})
}

// WatchOption configures a watch during For/Owns/Watches calls.
type WatchOption = util.Option[WatchOptions]

// WatchStrategy determines how objects are watched.
type WatchStrategy int

const (
	// WatchFull watches objects as *unstructured.Unstructured (default).
	// Provides full object data with zero conversion overhead.
	WatchFull WatchStrategy = iota

	// WatchPartial watches objects as *metav1.PartialObjectMetadata.
	// Lower memory usage but only metadata is available.
	WatchPartial
)

// WatchOptions holds configuration for watch setup.
type WatchOptions struct {
	// Handler is a custom event handler.
	// For Owns(), defaults to handler.EnqueueRequestForOwner if not provided.
	// For Watches(), must provide either Handler or use WithMapper.
	Handler handler.EventHandler

	// mapperFactory creates a typed mapper handler when scheme info is available.
	// Set by WithMapper[O]() - not directly accessible.
	// Mutually exclusive with Handler.
	mapperFactory MapperFactory

	// Predicates filter events before reconciliation.
	// Multiple WithPredicates() calls are additive.
	Predicates []predicate.Predicate

	// typedPredicateFactories create typed predicates when scheme info is available.
	// Set by TypedPredicate[O]() - not directly accessible.
	// These are resolved at registration time and added to Predicates.
	typedPredicateFactories []TypedPredicateFactory

	// Strategy determines how objects are watched (WatchFull or WatchPartial).
	// Default is WatchFull (*unstructured.Unstructured).
	Strategy WatchStrategy
}

// ApplyTo implements Option interface for WatchOptions.
// Predicates are additive (concatenated).
func (o *WatchOptions) ApplyTo(target *WatchOptions) {
	// Handler overrides if non-nil
	if o.Handler != nil {
		target.Handler = o.Handler
	}

	// mapperFactory overrides if set
	if o.hasMapper() {
		target.mapperFactory = o.mapperFactory
	}

	// Predicates are additive (concatenate)
	target.Predicates = append(target.Predicates, o.Predicates...)

	// TypedPredicateFactories are additive (concatenate)
	target.typedPredicateFactories = append(target.typedPredicateFactories, o.typedPredicateFactories...)

	// Strategy overrides if non-default (WatchPartial)
	if o.Strategy == WatchPartial {
		target.Strategy = WatchPartial
	}
}

// hasMapper returns true if a mapper factory is set.
func (o *WatchOptions) hasMapper() bool {
	return o.mapperFactory.Create != nil
}

// isPartial returns true if the watch strategy is WatchPartial.
func (o *WatchOptions) isPartial() bool {
	return o.Strategy == WatchPartial
}

// ApplyOptions applies all given options to this WatchOptions.
func (o *WatchOptions) ApplyOptions(opts []WatchOption) *WatchOptions {
	for _, opt := range opts {
		opt.ApplyTo(o)
	}

	return o
}

// WithHandler sets a custom event handler.
// Replaces the default handler for Owns() or any previously set handler.
// Mutually exclusive with WithMapper for Watches().
func WithHandler(h handler.EventHandler) WatchOption {
	return util.FunctionalOption[WatchOptions](func(opts *WatchOptions) {
		opts.Handler = h
	})
}

// WithMapper sets a typed mapping function that converts events to reconcile requests.
// Only valid for Watches(). Mutually exclusive with WithHandler.
// The mapper function receives objects and returns reconcile requests.
//
// The generic type O determines what type the mapper receives:
//   - *unstructured.Unstructured: receives unstructured directly (zero overhead)
//   - *metav1.PartialObjectMetadata: receives partial metadata directly (zero overhead)
//   - Typed objects (e.g., *corev1.Pod): automatically converted from unstructured
//
// Examples:
//
//	// Zero-conversion path - mapper receives unstructured directly
//	builder.Watches(gvks.Secret,
//	    builder.WithMapper(func(ctx context.Context, u *unstructured.Unstructured) []reconcile.Request {
//	        secretType, _, _ := unstructured.NestedString(u.Object, "type")
//	        if secretType == string(corev1.SecretTypeTLS) {
//	            return []reconcile.Request{{NamespacedName: types.NamespacedName{
//	                Name: "app", Namespace: u.GetNamespace()}}}
//	        }
//	        return nil
//	    }),
//	)
//
//	// Typed mapper - automatic conversion from unstructured to *corev1.Secret
//	builder.Watches(gvks.Secret,
//	    builder.WithMapper(func(ctx context.Context, secret *corev1.Secret) []reconcile.Request {
//	        if secret.Type == corev1.SecretTypeTLS {
//	            return []reconcile.Request{{NamespacedName: types.NamespacedName{
//	                Name: "app", Namespace: secret.Namespace}}}
//	        }
//	        return nil
//	    }),
//	)
func WithMapper[O client.Object](mapper func(context.Context, O) []reconcile.Request) WatchOption {
	return util.FunctionalOption[WatchOptions](func(opts *WatchOptions) {
		opts.mapperFactory = CreateTypedMapperHandler(mapper)
	})
}

// WithPredicates adds predicates that filter events before reconciliation.
// Multiple calls are additive - predicates are concatenated in order.
func WithPredicates(predicates ...predicate.Predicate) WatchOption {
	return util.FunctionalOption[WatchOptions](func(opts *WatchOptions) {
		opts.Predicates = append(opts.Predicates, predicates...)
	})
}

// TypedPredicate creates a typed predicate that receives objects of type O.
// Multiple calls are additive - predicates are concatenated in order.
//
// The generic type O determines what type the predicate receives:
//   - *unstructured.Unstructured: receives unstructured directly (zero overhead)
//   - *metav1.PartialObjectMetadata: receives partial metadata directly (zero overhead)
//   - Typed objects (e.g., *corev1.Pod): automatically converted from unstructured
//
// Examples:
//
//	// Zero-conversion path - predicate receives unstructured directly
//	builder.Watches(gvks.Pod,
//	    builder.TypedPredicate(func(u *unstructured.Unstructured) bool {
//	        phase, _, _ := unstructured.NestedString(u.Object, "status", "phase")
//	        return phase == "Running"
//	    }),
//	    builder.WithMapper(...),
//	)
//
//	// Typed predicate - automatic conversion from unstructured to *corev1.Pod
//	builder.Watches(gvks.Pod,
//	    builder.TypedPredicate(func(pod *corev1.Pod) bool {
//	        return pod.Status.Phase == corev1.PodRunning
//	    }),
//	    builder.WithMapper(...),
//	)
func TypedPredicate[O client.Object](filter func(O) bool) WatchOption {
	return util.FunctionalOption[WatchOptions](func(opts *WatchOptions) {
		opts.typedPredicateFactories = append(opts.typedPredicateFactories, CreateTypedPredicate(filter))
	})
}

// AsPartial configures the watch to use partial metadata instead of full objects.
// Reduces memory usage by ~70% for metadata-only watches.
// This is equivalent to WithStrategy(WatchPartial).
//
// Only applies to typed objects; unstructured and partial objects are unaffected.
func AsPartial() WatchOption {
	return WithStrategy(WatchPartial)
}

// WithStrategy sets the watch strategy (WatchFull or WatchPartial).
//
//   - WatchFull (default): Uses *unstructured.Unstructured with full object data
//   - WatchPartial: Uses *metav1.PartialObjectMetadata with lower memory usage
func WithStrategy(strategy WatchStrategy) WatchOption {
	return util.FunctionalOption[WatchOptions](func(opts *WatchOptions) {
		opts.Strategy = strategy
	})
}
