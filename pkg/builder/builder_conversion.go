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
	"errors"
	"fmt"
	"reflect"

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
//nolint:revive // needsConversion is a configuration flag, not control coupling
func wrapPredicates(
	scheme *runtime.Scheme,
	predicates []predicate.Predicate,
	needsConversion bool,
) []predicate.Predicate {
	if !needsConversion {
		return predicates
	}

	wrapped := make([]predicate.Predicate, len(predicates))
	for i, pred := range predicates {
		wrapped[i] = &predicateWrapper{
			scheme:    scheme,
			predicate: pred,
		}
	}

	return wrapped
}

// predicateWrapper wraps a user predicate with conversion logic.
type predicateWrapper struct {
	scheme    *runtime.Scheme
	predicate predicate.Predicate
}

func (pw *predicateWrapper) Create(e event.TypedCreateEvent[client.Object]) bool {
	typed, err := convertObject(pw.scheme, e.Object)
	if err != nil {
		// Drop event on conversion failure
		return false
	}

	return pw.predicate.Create(event.TypedCreateEvent[client.Object]{Object: typed})
}

func (pw *predicateWrapper) Update(e event.TypedUpdateEvent[client.Object]) bool {
	typedOld, err := convertObject(pw.scheme, e.ObjectOld)
	if err != nil {
		// Drop event on conversion failure
		return false
	}

	typedNew, err := convertObject(pw.scheme, e.ObjectNew)
	if err != nil {
		// Drop event on conversion failure
		return false
	}

	return pw.predicate.Update(event.TypedUpdateEvent[client.Object]{
		ObjectOld: typedOld,
		ObjectNew: typedNew,
	})
}

func (pw *predicateWrapper) Delete(e event.TypedDeleteEvent[client.Object]) bool {
	typed, err := convertObject(pw.scheme, e.Object)
	if err != nil {
		// Drop event on conversion failure
		return false
	}

	return pw.predicate.Delete(event.TypedDeleteEvent[client.Object]{
		Object:             typed,
		DeleteStateUnknown: e.DeleteStateUnknown,
	})
}

func (pw *predicateWrapper) Generic(e event.TypedGenericEvent[client.Object]) bool {
	typed, err := convertObject(pw.scheme, e.Object)
	if err != nil {
		// Drop event on conversion failure
		return false
	}

	return pw.predicate.Generic(event.TypedGenericEvent[client.Object]{Object: typed})
}

// wrapHandler wraps a handler to handle conversion if needed.
// When needsConversion is true, the handler is wrapped to convert unstructured events to typed objects.
// Conversion failures cause the event to be dropped without enqueueing.
//
//nolint:revive // needsConversion is a configuration flag, not control coupling
func wrapHandler(
	scheme *runtime.Scheme,
	h handler.EventHandler,
	needsConversion bool,
) handler.EventHandler {
	if !needsConversion {
		return h
	}

	return &handlerWrapper{
		scheme:  scheme,
		handler: h,
	}
}

// handlerWrapper wraps a user handler with conversion logic.
type handlerWrapper struct {
	scheme  *runtime.Scheme
	handler handler.EventHandler
}

func (hw *handlerWrapper) Create(
	ctx context.Context,
	e event.TypedCreateEvent[client.Object],
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	typed, err := convertObject(hw.scheme, e.Object)
	if err != nil {
		// Drop event on conversion failure
		return
	}

	hw.handler.Create(ctx, event.TypedCreateEvent[client.Object]{Object: typed}, q)
}

func (hw *handlerWrapper) Update(
	ctx context.Context,
	e event.TypedUpdateEvent[client.Object],
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	typedOld, err := convertObject(hw.scheme, e.ObjectOld)
	if err != nil {
		// Drop event on conversion failure
		return
	}

	typedNew, err := convertObject(hw.scheme, e.ObjectNew)
	if err != nil {
		// Drop event on conversion failure
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
	typed, err := convertObject(hw.scheme, e.Object)
	if err != nil {
		// Drop event on conversion failure
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
	typed, err := convertObject(hw.scheme, e.Object)
	if err != nil {
		// Drop event on conversion failure
		return
	}

	hw.handler.Generic(ctx, event.TypedGenericEvent[client.Object]{Object: typed}, q)
}

// convertObject converts unstructured or partial objects to typed objects using the scheme.
// Returns error if conversion fails (type not registered or incompatible).
func convertObject(
	scheme *runtime.Scheme,
	obj client.Object,
) (client.Object, error) {
	// Handle unstructured conversion
	if u, ok := obj.(*unstructured.Unstructured); ok {
		gvk := u.GroupVersionKind()

		// Create zero value of target type from scheme
		target, err := scheme.New(gvk)
		if err != nil {
			return nil, fmt.Errorf("failed to create target type from scheme: %w", err)
		}

		// Type assert target to client.Object
		targetObj, ok := target.(client.Object)
		if !ok {
			return nil, fmt.Errorf("target type %T does not implement client.Object", target)
		}

		// Convert using resources.FromUnstructured
		if err := resources.FromUnstructured(scheme, u, targetObj); err != nil {
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

// createUntypedMapperHandler creates a handler from a type-erased mapper using reflection.
// This is used when the generic type parameter cannot be determined at compile time.
func createUntypedMapperHandler(
	scheme *runtime.Scheme,
	mapper any,
	needsConversion bool,
) handler.EventHandler {
	return &untypedMapperHandler{
		scheme:          scheme,
		mapper:          mapper,
		needsConversion: needsConversion,
	}
}

// untypedMapperHandler wraps an untyped mapper function.
type untypedMapperHandler struct {
	scheme          *runtime.Scheme
	mapper          any
	needsConversion bool
}

func (umh *untypedMapperHandler) Create(
	ctx context.Context,
	e event.TypedCreateEvent[client.Object],
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	obj := e.Object
	if umh.needsConversion {
		typed, err := convertObject(umh.scheme, obj)
		if err != nil {
			return
		}
		obj = typed
	}

	if err := umh.invokeMapper(ctx, obj, q); err != nil {
		return
	}
}

func (umh *untypedMapperHandler) Update(
	ctx context.Context,
	e event.TypedUpdateEvent[client.Object],
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	obj := e.ObjectNew
	if umh.needsConversion {
		typed, err := convertObject(umh.scheme, obj)
		if err != nil {
			return
		}
		obj = typed
	}

	if err := umh.invokeMapper(ctx, obj, q); err != nil {
		return
	}
}

func (umh *untypedMapperHandler) Delete(
	ctx context.Context,
	e event.TypedDeleteEvent[client.Object],
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	obj := e.Object
	if umh.needsConversion {
		typed, err := convertObject(umh.scheme, obj)
		if err != nil {
			return
		}
		obj = typed
	}

	if err := umh.invokeMapper(ctx, obj, q); err != nil {
		return
	}
}

func (umh *untypedMapperHandler) Generic(
	ctx context.Context,
	e event.TypedGenericEvent[client.Object],
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	obj := e.Object
	if umh.needsConversion {
		typed, err := convertObject(umh.scheme, obj)
		if err != nil {
			return
		}
		obj = typed
	}

	if err := umh.invokeMapper(ctx, obj, q); err != nil {
		return
	}
}

// invokeMapper calls the mapper function using reflection and enqueues results.
func (umh *untypedMapperHandler) invokeMapper(
	ctx context.Context,
	obj client.Object,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) error {
	// Try direct type assertion for common mapper signature
	// func(context.Context, client.Object) []reconcile.Request
	if mapper, ok := umh.mapper.(func(context.Context, client.Object) []reconcile.Request); ok {
		for _, req := range mapper(ctx, obj) {
			q.Add(req)
		}

		return nil
	}

	// Use reflection for typed mappers
	// Mapper signature: func(context.Context, *T) []reconcile.Request
	mapperValue := reflect.ValueOf(umh.mapper)
	if mapperValue.Kind() != reflect.Func {
		return errors.New("mapper is not a function")
	}

	mapperType := mapperValue.Type()
	if mapperType.NumIn() != 2 || mapperType.NumOut() != 1 {
		return errors.New("mapper has wrong signature")
	}

	// Check if obj can be converted to the mapper's expected type
	expectedType := mapperType.In(1)
	objValue := reflect.ValueOf(obj)
	if !objValue.Type().AssignableTo(expectedType) {
		return fmt.Errorf("object type %T not assignable to mapper parameter type %v", obj, expectedType)
	}

	// Call mapper with reflection
	results := mapperValue.Call([]reflect.Value{
		reflect.ValueOf(ctx),
		objValue,
	})

	// Extract reconcile.Request slice from result
	if results[0].IsNil() {
		return nil
	}

	requests, ok := results[0].Interface().([]reconcile.Request)
	if !ok {
		return errors.New("mapper did not return []reconcile.Request")
	}

	for _, req := range requests {
		q.Add(req)
	}

	return nil
}
