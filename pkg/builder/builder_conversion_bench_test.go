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

//nolint:testpackage // Testing internal functions requires same package
package builder

import (
	"context"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/workqueue"
)

// setupBenchScheme creates a scheme with corev1 registered.
func setupBenchScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	return scheme
}

// createUnstructuredConfigMap creates an unstructured ConfigMap for benchmarking.
func createUnstructuredConfigMap(name string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))
	u.SetName(name)
	u.SetNamespace("default")
	u.Object["data"] = map[string]any{
		"key1": "value1",
		"key2": "value2",
	}

	return u
}

// noopHandler is a handler that does nothing (for benchmarking).
//
//nolint:gochecknoglobals // Benchmark fixture shared across tests
var noopHandler = handler.TypedFuncs[client.Object, reconcile.Request]{
	CreateFunc: func(
		_ context.Context,
		_ event.TypedCreateEvent[client.Object],
		_ workqueue.TypedRateLimitingInterface[reconcile.Request],
	) {
	},
	UpdateFunc: func(
		_ context.Context,
		_ event.TypedUpdateEvent[client.Object],
		_ workqueue.TypedRateLimitingInterface[reconcile.Request],
	) {
	},
	DeleteFunc: func(
		_ context.Context,
		_ event.TypedDeleteEvent[client.Object],
		_ workqueue.TypedRateLimitingInterface[reconcile.Request],
	) {
	},
	GenericFunc: func(
		_ context.Context,
		_ event.TypedGenericEvent[client.Object],
		_ workqueue.TypedRateLimitingInterface[reconcile.Request],
	) {
	},
}

// acceptAllPredicate is a predicate that accepts all events.
//
//nolint:gochecknoglobals // Benchmark fixture shared across tests
var acceptAllPredicate = predicate.Funcs{
	CreateFunc:  func(_ event.TypedCreateEvent[client.Object]) bool { return true },
	UpdateFunc:  func(_ event.TypedUpdateEvent[client.Object]) bool { return true },
	DeleteFunc:  func(_ event.TypedDeleteEvent[client.Object]) bool { return true },
	GenericFunc: func(_ event.TypedGenericEvent[client.Object]) bool { return true },
}

// BenchmarkConversion_SeparateWrappers benchmarks the old approach where predicates
// and handlers are wrapped separately, causing double conversion.
func BenchmarkConversion_SeparateWrappers_Create(b *testing.B) {
	scheme := setupBenchScheme()
	predicates := []predicate.Predicate{acceptAllPredicate}

	// Wrap predicates and handler separately (old approach)
	wrappedPreds := wrapPredicates(scheme, predicates, true, "bench-controller")
	wrappedHandler := wrapHandler(scheme, noopHandler, true, "bench-controller")

	u := createUnstructuredConfigMap("test")
	e := event.TypedCreateEvent[client.Object]{Object: u}
	q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
	ctx := context.Background()

	b.ReportAllocs()

	for b.Loop() {
		// Simulate event flow: predicate first, then handler
		if wrappedPreds[0].Create(e) {
			wrappedHandler.Create(ctx, e, q)
		}
	}
}

// BenchmarkConversion_CombinedHandler benchmarks the new approach where conversion
// happens once in a combined handler that evaluates predicates internally.
func BenchmarkConversion_CombinedHandler_Create(b *testing.B) {
	scheme := setupBenchScheme()
	predicates := []predicate.Predicate{acceptAllPredicate}

	// Combined handler (new approach)
	combined := newConvertingHandler(scheme, noopHandler, predicates, "bench-controller")

	u := createUnstructuredConfigMap("test")
	e := event.TypedCreateEvent[client.Object]{Object: u}
	q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
	ctx := context.Background()

	b.ReportAllocs()

	for b.Loop() {
		combined.Create(ctx, e, q)
	}
}

// BenchmarkConversion_SeparateWrappers_Update benchmarks Update events with the old approach.
// Update events convert 2 objects (old and new), so double conversion = 4 conversions total.
func BenchmarkConversion_SeparateWrappers_Update(b *testing.B) {
	scheme := setupBenchScheme()
	predicates := []predicate.Predicate{acceptAllPredicate}

	wrappedPreds := wrapPredicates(scheme, predicates, true, "bench-controller")
	wrappedHandler := wrapHandler(scheme, noopHandler, true, "bench-controller")

	oldU := createUnstructuredConfigMap("old")
	newU := createUnstructuredConfigMap("new")
	e := event.TypedUpdateEvent[client.Object]{ObjectOld: oldU, ObjectNew: newU}
	q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
	ctx := context.Background()

	b.ReportAllocs()

	for b.Loop() {
		if wrappedPreds[0].Update(e) {
			wrappedHandler.Update(ctx, e, q)
		}
	}
}

// BenchmarkConversion_CombinedHandler_Update benchmarks Update events with the new approach.
// Update events convert 2 objects (old and new), but only once = 2 conversions total.
func BenchmarkConversion_CombinedHandler_Update(b *testing.B) {
	scheme := setupBenchScheme()
	predicates := []predicate.Predicate{acceptAllPredicate}

	combined := newConvertingHandler(scheme, noopHandler, predicates, "bench-controller")

	oldU := createUnstructuredConfigMap("old")
	newU := createUnstructuredConfigMap("new")
	e := event.TypedUpdateEvent[client.Object]{ObjectOld: oldU, ObjectNew: newU}
	q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
	ctx := context.Background()

	b.ReportAllocs()

	for b.Loop() {
		combined.Update(ctx, e, q)
	}
}

// BenchmarkConversion_SeparateWrappers_MultiplePredicates benchmarks with multiple predicates.
func BenchmarkConversion_SeparateWrappers_MultiplePredicates(b *testing.B) {
	scheme := setupBenchScheme()
	predicates := []predicate.Predicate{
		acceptAllPredicate,
		acceptAllPredicate,
		acceptAllPredicate,
	}

	wrappedPreds := wrapPredicates(scheme, predicates, true, "bench-controller")
	wrappedHandler := wrapHandler(scheme, noopHandler, true, "bench-controller")

	u := createUnstructuredConfigMap("test")
	e := event.TypedCreateEvent[client.Object]{Object: u}
	q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
	ctx := context.Background()

	b.ReportAllocs()

	for b.Loop() {
		pass := true
		for _, p := range wrappedPreds {
			if !p.Create(e) {
				pass = false

				break
			}
		}

		if pass {
			wrappedHandler.Create(ctx, e, q)
		}
	}
}

// BenchmarkConversion_CombinedHandler_MultiplePredicates benchmarks combined handler with multiple predicates.
func BenchmarkConversion_CombinedHandler_MultiplePredicates(b *testing.B) {
	scheme := setupBenchScheme()
	predicates := []predicate.Predicate{
		acceptAllPredicate,
		acceptAllPredicate,
		acceptAllPredicate,
	}

	combined := newConvertingHandler(scheme, noopHandler, predicates, "bench-controller")

	u := createUnstructuredConfigMap("test")
	e := event.TypedCreateEvent[client.Object]{Object: u}
	q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
	ctx := context.Background()

	b.ReportAllocs()

	for b.Loop() {
		combined.Create(ctx, e, q)
	}
}

// BenchmarkTypedMapper_SeparateWrappers benchmarks typed mapper with separate predicate wrapping.
func BenchmarkTypedMapper_SeparateWrappers_Create(b *testing.B) {
	scheme := setupBenchScheme()
	predicates := []predicate.Predicate{acceptAllPredicate}

	mapper := func(_ context.Context, _ *corev1.ConfigMap) []reconcile.Request {
		return nil
	}

	// Old approach: wrap predicates separately, pass nil predicates to mapper
	wrappedPreds := wrapPredicates(scheme, predicates, true, "bench-controller")
	factory := CreateTypedMapperHandler(mapper)
	h := factory.Create(scheme, true, "bench-controller", nil)

	u := createUnstructuredConfigMap("test")
	e := event.TypedCreateEvent[client.Object]{Object: u}
	q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
	ctx := context.Background()

	b.ReportAllocs()

	for b.Loop() {
		if wrappedPreds[0].Create(e) {
			h.Create(ctx, e, q)
		}
	}
}

// BenchmarkTypedMapper_IntegratedPredicates benchmarks typed mapper with integrated predicates.
func BenchmarkTypedMapper_IntegratedPredicates_Create(b *testing.B) {
	scheme := setupBenchScheme()
	predicates := []predicate.Predicate{acceptAllPredicate}

	mapper := func(_ context.Context, _ *corev1.ConfigMap) []reconcile.Request {
		return nil
	}

	// New approach: pass predicates to mapper factory
	factory := CreateTypedMapperHandler(mapper)
	h := factory.Create(scheme, true, "bench-controller", predicates)

	u := createUnstructuredConfigMap("test")
	e := event.TypedCreateEvent[client.Object]{Object: u}
	q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
	ctx := context.Background()

	b.ReportAllocs()

	for b.Loop() {
		h.Create(ctx, e, q)
	}
}
