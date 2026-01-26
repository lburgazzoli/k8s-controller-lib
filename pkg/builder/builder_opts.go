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

// WatchOptions holds configuration for watch setup.
// Both struct-based and function-based options are supported.
// When both are provided, function options override struct fields.
type WatchOptions struct {
	// Handler is a custom event handler.
	// For Owns(), defaults to handler.EnqueueRequestForOwner if not provided.
	// For Watches(), must provide either Handler or Mapper.
	Handler handler.EventHandler

	// Mapper is a typed mapping function (Watches only).
	// Mutually exclusive with Handler.
	// Stored as interface{} to support generic mapper functions.
	Mapper any

	// Predicates filter events before reconciliation.
	// Multiple WithPredicates() calls are additive.
	Predicates []predicate.Predicate

	// AsPartial uses partial metadata instead of full object for memory optimization.
	AsPartial bool
}

// ApplyTo implements Option interface for WatchOptions.
// Function options override struct fields when both are provided.
// Predicates are additive (concatenated).
func (o *WatchOptions) ApplyTo(target *WatchOptions) {
	// Handler overrides if non-nil
	if o.Handler != nil {
		target.Handler = o.Handler
	}

	// Mapper overrides if non-nil
	if o.Mapper != nil {
		target.Mapper = o.Mapper
	}

	// Predicates are additive (concatenate)
	target.Predicates = append(target.Predicates, o.Predicates...)

	// AsPartial always overrides
	if o.AsPartial {
		target.AsPartial = true
	}
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
// The mapper function receives typed objects and returns reconcile requests.
//
// Example:
//
//	builder.Watches(
//	    &corev1.Secret{},
//	    builder.WithMapper(
//	        func(ctx context.Context, secret *corev1.Secret) []reconcile.Request {
//	            if secret.Type == corev1.SecretTypeTLS {
//	                return []reconcile.Request{
//	                    {NamespacedName: types.NamespacedName{
//	                        Name: "app", Namespace: secret.Namespace}},
//	                }
//	            }
//	            return nil
//	        },
//	    ),
//	)
func WithMapper[O client.Object](mapper func(context.Context, O) []reconcile.Request) WatchOption {
	return util.FunctionalOption[WatchOptions](func(opts *WatchOptions) {
		opts.Mapper = mapper
	})
}

// WithPredicates adds predicates that filter events before reconciliation.
// Multiple calls are additive - predicates are concatenated in order.
func WithPredicates(predicates ...predicate.Predicate) WatchOption {
	return util.FunctionalOption[WatchOptions](func(opts *WatchOptions) {
		opts.Predicates = append(opts.Predicates, predicates...)
	})
}

// AsPartial configures the watch to use partial metadata instead of full objects.
// Reduces memory usage by ~70% for metadata-only watches.
// Only applies to typed objects; unstructured and partial objects are unaffected.
func AsPartial() WatchOption {
	return util.FunctionalOption[WatchOptions](func(opts *WatchOptions) {
		opts.AsPartial = true
	})
}
