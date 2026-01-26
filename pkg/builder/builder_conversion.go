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
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/workqueue"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/resources"
)

// converter encapsulates scheme and controller name for object conversion.
// It provides a single method to convert unstructured objects to typed objects
// while tracking conversion errors in metrics.
type converter struct {
	scheme         *runtime.Scheme
	controllerName string
}

// Convert converts unstructured or partial objects to typed objects using the scheme.
// Returns error if conversion fails (type not registered or incompatible).
// Conversion failures are tracked via the builder_conversion_errors_total metric.
func (c *converter) Convert(obj client.Object) (client.Object, error) {
	// Handle unstructured conversion
	if u, ok := obj.(*unstructured.Unstructured); ok {
		gvk := u.GroupVersionKind()

		// Create zero value of target type from scheme
		target, err := c.scheme.New(gvk)
		if err != nil {
			ConversionErrorsTotal.WithLabelValues(
				c.controllerName,
				gvk.GroupVersion().String(),
				gvk.Kind,
				ReasonTypeNotInScheme,
			).Inc()

			return nil, fmt.Errorf("failed to create target type from scheme: %w", err)
		}

		// Type assert target to client.Object
		targetObj, ok := target.(client.Object)
		if !ok {
			ConversionErrorsTotal.WithLabelValues(
				c.controllerName,
				gvk.GroupVersion().String(),
				gvk.Kind,
				ReasonNotClientObject,
			).Inc()

			return nil, fmt.Errorf("target type %T does not implement client.Object", target)
		}

		// Convert using resources.FromUnstructured
		if err := resources.FromUnstructured(c.scheme, u, targetObj); err != nil {
			ConversionErrorsTotal.WithLabelValues(
				c.controllerName,
				gvk.GroupVersion().String(),
				gvk.Kind,
				ReasonConversionFailed,
			).Inc()

			return nil, fmt.Errorf("failed to convert from unstructured: %w", err)
		}

		return targetObj, nil
	}

	// Partial metadata cannot be converted to typed objects
	// It only contains metadata, no spec/status fields
	if _, ok := obj.(*metav1.PartialObjectMetadata); ok {
		return obj, nil
	}

	// Already typed
	return obj, nil
}

// processObject determines watch strategy based on object type and asPartial flag.
// Returns the watch object (with only GVK set), whether conversion is needed, and any error.
//
//nolint:revive // asPartial is a configuration flag, not control coupling
func processObject[O client.Object](
	obj O,
	scheme *runtime.Scheme,
	asPartial bool,
) (client.Object, bool, error) {
	switch v := any(obj).(type) {
	case *unstructured.Unstructured:
		// Use directly - copy GVK only
		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(v.GroupVersionKind())

		return u, false, nil

	case *metav1.PartialObjectMetadata:
		// Use directly - copy GVK only
		p := &metav1.PartialObjectMetadata{}
		p.SetGroupVersionKind(v.GroupVersionKind())

		return p, false, nil

	default:
		// Typed object - extract GVK
		gvk, err := resources.GetGroupVersionKindForObject(scheme, obj)
		if err != nil {
			return nil, false, fmt.Errorf("failed to get GVK for object type %T: %w", obj, err)
		}

		// Create watch object based on asPartial flag
		if asPartial {
			p := &metav1.PartialObjectMetadata{}
			p.SetGroupVersionKind(gvk)

			return p, true, nil
		}

		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(gvk)

		return u, true, nil
	}
}

// wrapPredicates wraps predicates to handle conversion if needed.
// When needsConversion is true, each predicate is wrapped to convert unstructured events to typed objects.
// Conversion failures cause the predicate to return false, dropping the event.
//
//nolint:revive,unparam // config flags; controllerName varies per controller
func wrapPredicates(
	scheme *runtime.Scheme,
	predicates []predicate.Predicate,
	needsConversion bool,
	controllerName string,
) []predicate.Predicate {
	if !needsConversion {
		return predicates
	}

	conv := &converter{scheme: scheme, controllerName: controllerName}
	wrapped := make([]predicate.Predicate, len(predicates))

	for i, pred := range predicates {
		wrapped[i] = &predicateWrapper{
			converter: conv,
			predicate: pred,
		}
	}

	return wrapped
}

// predicateWrapper wraps a user predicate with conversion logic.
type predicateWrapper struct {
	converter *converter
	predicate predicate.Predicate
}

func (pw *predicateWrapper) Create(e event.TypedCreateEvent[client.Object]) bool {
	typed, err := pw.converter.Convert(e.Object)
	if err != nil {
		return false
	}

	return pw.predicate.Create(event.TypedCreateEvent[client.Object]{Object: typed})
}

func (pw *predicateWrapper) Update(e event.TypedUpdateEvent[client.Object]) bool {
	typedOld, err := pw.converter.Convert(e.ObjectOld)
	if err != nil {
		return false
	}

	typedNew, err := pw.converter.Convert(e.ObjectNew)
	if err != nil {
		return false
	}

	return pw.predicate.Update(event.TypedUpdateEvent[client.Object]{
		ObjectOld: typedOld,
		ObjectNew: typedNew,
	})
}

func (pw *predicateWrapper) Delete(e event.TypedDeleteEvent[client.Object]) bool {
	typed, err := pw.converter.Convert(e.Object)
	if err != nil {
		return false
	}

	return pw.predicate.Delete(event.TypedDeleteEvent[client.Object]{
		Object:             typed,
		DeleteStateUnknown: e.DeleteStateUnknown,
	})
}

func (pw *predicateWrapper) Generic(e event.TypedGenericEvent[client.Object]) bool {
	typed, err := pw.converter.Convert(e.Object)
	if err != nil {
		return false
	}

	return pw.predicate.Generic(event.TypedGenericEvent[client.Object]{Object: typed})
}

// wrapHandler wraps a handler to handle conversion if needed.
// When needsConversion is true, the handler is wrapped to convert unstructured events to typed objects.
// Conversion failures cause the event to be dropped without enqueueing.
//
//nolint:revive,unparam // config flags; controllerName varies per controller
func wrapHandler(
	scheme *runtime.Scheme,
	h handler.EventHandler,
	needsConversion bool,
	controllerName string,
) handler.EventHandler {
	if !needsConversion {
		return h
	}

	return &handlerWrapper{
		converter: &converter{scheme: scheme, controllerName: controllerName},
		handler:   h,
	}
}

// handlerWrapper wraps a user handler with conversion logic.
type handlerWrapper struct {
	converter *converter
	handler   handler.EventHandler
}

func (hw *handlerWrapper) Create(
	ctx context.Context,
	e event.TypedCreateEvent[client.Object],
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	typed, err := hw.converter.Convert(e.Object)
	if err != nil {
		return
	}

	hw.handler.Create(ctx, event.TypedCreateEvent[client.Object]{Object: typed}, q)
}

func (hw *handlerWrapper) Update(
	ctx context.Context,
	e event.TypedUpdateEvent[client.Object],
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	typedOld, err := hw.converter.Convert(e.ObjectOld)
	if err != nil {
		return
	}

	typedNew, err := hw.converter.Convert(e.ObjectNew)
	if err != nil {
		return
	}

	hw.handler.Update(ctx, event.TypedUpdateEvent[client.Object]{
		ObjectOld: typedOld,
		ObjectNew: typedNew,
	}, q)
}

func (hw *handlerWrapper) Delete(
	ctx context.Context,
	e event.TypedDeleteEvent[client.Object],
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	typed, err := hw.converter.Convert(e.Object)
	if err != nil {
		return
	}

	hw.handler.Delete(ctx, event.TypedDeleteEvent[client.Object]{
		Object:             typed,
		DeleteStateUnknown: e.DeleteStateUnknown,
	}, q)
}

func (hw *handlerWrapper) Generic(
	ctx context.Context,
	e event.TypedGenericEvent[client.Object],
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	typed, err := hw.converter.Convert(e.Object)
	if err != nil {
		return
	}

	hw.handler.Generic(ctx, event.TypedGenericEvent[client.Object]{Object: typed}, q)
}

// MapperFactory is a function that creates a typed mapper handler.
// It is called during watch registration when scheme, conversion info, and controller name are available.
type MapperFactory func(scheme *runtime.Scheme, needsConversion bool, controllerName string) handler.EventHandler

// CreateTypedMapperHandler creates a handler from a typed mapper function.
// This is called at registration time when the type O is known via generics,
// avoiding runtime reflection during event processing.
func CreateTypedMapperHandler[O client.Object](
	mapper func(context.Context, O) []reconcile.Request,
) MapperFactory {
	return func(scheme *runtime.Scheme, needsConversion bool, controllerName string) handler.EventHandler {
		var conv *converter
		if needsConversion {
			conv = &converter{scheme: scheme, controllerName: controllerName}
		}

		return &typedMapperHandler[O]{
			converter: conv,
			mapper:    mapper,
		}
	}
}

// typedMapperHandler wraps a typed mapper function without reflection.
// The generic type O is captured at registration time via CreateTypedMapperHandler.
type typedMapperHandler[O client.Object] struct {
	converter *converter // nil if no conversion needed
	mapper    func(context.Context, O) []reconcile.Request
}

func (tmh *typedMapperHandler[O]) Create(
	ctx context.Context,
	e event.TypedCreateEvent[client.Object],
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	obj, ok := tmh.getTypedObject(e.Object)
	if !ok {
		return
	}

	for _, req := range tmh.mapper(ctx, obj) {
		q.Add(req)
	}
}

func (tmh *typedMapperHandler[O]) Update(
	ctx context.Context,
	e event.TypedUpdateEvent[client.Object],
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	obj, ok := tmh.getTypedObject(e.ObjectNew)
	if !ok {
		return
	}

	for _, req := range tmh.mapper(ctx, obj) {
		q.Add(req)
	}
}

func (tmh *typedMapperHandler[O]) Delete(
	ctx context.Context,
	e event.TypedDeleteEvent[client.Object],
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	obj, ok := tmh.getTypedObject(e.Object)
	if !ok {
		return
	}

	for _, req := range tmh.mapper(ctx, obj) {
		q.Add(req)
	}
}

func (tmh *typedMapperHandler[O]) Generic(
	ctx context.Context,
	e event.TypedGenericEvent[client.Object],
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	obj, ok := tmh.getTypedObject(e.Object)
	if !ok {
		return
	}

	for _, req := range tmh.mapper(ctx, obj) {
		q.Add(req)
	}
}

// getTypedObject converts the object to the expected type O.
// If converter is set (typed object watched via unstructured), it converts first.
// Returns the typed object and true on success, zero value and false on failure.
func (tmh *typedMapperHandler[O]) getTypedObject(obj client.Object) (O, bool) {
	var zero O

	if tmh.converter != nil {
		typed, err := tmh.converter.Convert(obj)
		if err != nil {
			return zero, false
		}

		obj = typed
	}

	result, ok := obj.(O)

	return result, ok
}
