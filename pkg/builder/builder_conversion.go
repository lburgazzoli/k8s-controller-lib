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
//nolint:revive // flag-parameter: asPartial is a configuration flag, not control coupling
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
//nolint:revive // flag-parameter: needsConversion is a configuration flag, not control coupling
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
//nolint:revive // flag-parameter: needsConversion is a configuration flag, not control coupling
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

// newConvertingHandler creates a handler that converts objects once and evaluates predicates
// before delegating to the underlying handler. This avoids the double conversion that occurs
// when using separate predicate wrappers and handler wrappers.
//

func newConvertingHandler(
	scheme *runtime.Scheme,
	h handler.EventHandler,
	predicates []predicate.Predicate,
	controllerName string,
) handler.EventHandler {
	return &convertingHandler{
		converter:  &converter{scheme: scheme, controllerName: controllerName},
		handler:    h,
		predicates: predicates,
	}
}

// convertingHandler combines conversion, predicate evaluation, and handler invocation
// into a single pass. This eliminates the double conversion that occurs when predicates
// and handlers are wrapped separately.
type convertingHandler struct {
	converter  *converter
	handler    handler.EventHandler
	predicates []predicate.Predicate
}

func (ch *convertingHandler) Create(
	ctx context.Context,
	e event.TypedCreateEvent[client.Object],
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	typed, err := ch.converter.Convert(e.Object)
	if err != nil {
		return
	}

	typedEvent := event.TypedCreateEvent[client.Object]{Object: typed}

	for _, p := range ch.predicates {
		if !p.Create(typedEvent) {
			return
		}
	}

	ch.handler.Create(ctx, typedEvent, q)
}

func (ch *convertingHandler) Update(
	ctx context.Context,
	e event.TypedUpdateEvent[client.Object],
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	typedOld, err := ch.converter.Convert(e.ObjectOld)
	if err != nil {
		return
	}

	typedNew, err := ch.converter.Convert(e.ObjectNew)
	if err != nil {
		return
	}

	typedEvent := event.TypedUpdateEvent[client.Object]{
		ObjectOld: typedOld,
		ObjectNew: typedNew,
	}

	for _, p := range ch.predicates {
		if !p.Update(typedEvent) {
			return
		}
	}

	ch.handler.Update(ctx, typedEvent, q)
}

func (ch *convertingHandler) Delete(
	ctx context.Context,
	e event.TypedDeleteEvent[client.Object],
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	typed, err := ch.converter.Convert(e.Object)
	if err != nil {
		return
	}

	typedEvent := event.TypedDeleteEvent[client.Object]{
		Object:             typed,
		DeleteStateUnknown: e.DeleteStateUnknown,
	}

	for _, p := range ch.predicates {
		if !p.Delete(typedEvent) {
			return
		}
	}

	ch.handler.Delete(ctx, typedEvent, q)
}

func (ch *convertingHandler) Generic(
	ctx context.Context,
	e event.TypedGenericEvent[client.Object],
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	typed, err := ch.converter.Convert(e.Object)
	if err != nil {
		return
	}

	typedEvent := event.TypedGenericEvent[client.Object]{Object: typed}

	for _, p := range ch.predicates {
		if !p.Generic(typedEvent) {
			return
		}
	}

	ch.handler.Generic(ctx, typedEvent, q)
}

// MapperExpectation indicates what object type the mapper expects.
type MapperExpectation int

const (
	// MapperExpectsUnstructured means the mapper expects *unstructured.Unstructured.
	MapperExpectsUnstructured MapperExpectation = iota
	// MapperExpectsPartial means the mapper expects *metav1.PartialObjectMetadata.
	MapperExpectsPartial
	// MapperExpectsClientObject means the mapper expects client.Object (any type works).
	MapperExpectsClientObject
	// MapperExpectsTyped means the mapper expects a typed object (e.g., *corev1.Pod).
	MapperExpectsTyped
)

// MapperFactoryFunc is a function that creates a typed mapper handler.
// It is called during watch registration when scheme, conversion info, controller name, and predicates are available.
// Predicates are passed so the handler can evaluate them after conversion, avoiding double conversion.
type MapperFactoryFunc func(
	scheme *runtime.Scheme,
	needsConversion bool,
	controllerName string,
	predicates []predicate.Predicate,
) handler.EventHandler

// MapperFactory holds a mapper factory function and its type expectations.
type MapperFactory struct {
	// Expectation indicates what object type the mapper expects.
	Expectation MapperExpectation
	// Create creates the handler.
	Create MapperFactoryFunc
}

// CreateTypedMapperHandler creates a handler from a typed mapper function.
// This is called at registration time when the type O is known via generics,
// avoiding runtime reflection during event processing.
// Predicates are evaluated internally after conversion to avoid double conversion.
//
// The generic type O determines if conversion is needed:
//   - *unstructured.Unstructured: no conversion, requires watch without AsPartial()
//   - *metav1.PartialObjectMetadata: no conversion, requires watch with AsPartial()
//   - client.Object: no conversion, works with any watch type
//   - Typed objects (e.g., *corev1.Pod): conversion from unstructured to typed
//
// This allows using typed mappers with GVK-based watches:
//
//	b.Watches(gvks.Pod, WithMapper(func(ctx context.Context, pod *corev1.Pod) []reconcile.Request {
//	    // pod is automatically converted from unstructured
//	    return nil
//	}))
func CreateTypedMapperHandler[O client.Object](
	mapper func(context.Context, O) []reconcile.Request,
) MapperFactory {
	// Determine the mapper's expectation based on the generic type O
	expectation := getMapperExpectation[O]()

	// Determine if the mapper needs conversion (typed objects)
	mapperNeedsConversion := expectation == MapperExpectsTyped

	createFn := func(
		scheme *runtime.Scheme,
		needsConversion bool,
		controllerName string,
		predicates []predicate.Predicate,
	) handler.EventHandler {
		// Conversion is needed if either:
		// 1. The watched object needs conversion (typed object watched via unstructured)
		// 2. The mapper expects typed objects (detected from generic type O)
		actualNeedsConversion := needsConversion || mapperNeedsConversion

		var conv *converter
		if actualNeedsConversion {
			conv = &converter{scheme: scheme, controllerName: controllerName}
		}

		return &typedMapperHandler[O]{
			converter:  conv,
			mapper:     mapper,
			predicates: predicates,
		}
	}

	return MapperFactory{
		Expectation: expectation,
		Create:      createFn,
	}
}

// getMapperExpectation determines what object type the generic type O expects.
func getMapperExpectation[O client.Object]() MapperExpectation {
	oType := reflect.TypeFor[O]()

	unstructuredType := reflect.TypeFor[*unstructured.Unstructured]()
	partialType := reflect.TypeFor[*metav1.PartialObjectMetadata]()
	clientObjectType := reflect.TypeFor[client.Object]()

	switch oType {
	case unstructuredType:
		return MapperExpectsUnstructured
	case partialType:
		return MapperExpectsPartial
	case clientObjectType:
		return MapperExpectsClientObject
	default:
		return MapperExpectsTyped
	}
}

// typeRequiresConversion checks if the generic type O requires conversion from unstructured.
// Returns false for types that don't need conversion (unstructured, partial metadata, client.Object interface).
// Returns true for typed objects (e.g., *corev1.Pod) that need conversion.
func typeRequiresConversion[O client.Object]() bool {
	oType := reflect.TypeFor[O]()

	// Types that don't require conversion
	unstructuredType := reflect.TypeFor[*unstructured.Unstructured]()
	partialType := reflect.TypeFor[*metav1.PartialObjectMetadata]()
	clientObjectType := reflect.TypeFor[client.Object]()

	return oType != unstructuredType && oType != partialType && oType != clientObjectType
}

// TypedPredicateFactory is a function that creates a typed predicate.
// It is called during watch registration when scheme and controller name are available.
type TypedPredicateFactory func(
	scheme *runtime.Scheme,
	controllerName string,
) predicate.Predicate

// CreateTypedPredicate creates a predicate from a typed filter function.
// This is called at registration time when the type O is known via generics.
//
// The generic type O determines if conversion is needed:
//   - *unstructured.Unstructured, *metav1.PartialObjectMetadata, client.Object: no conversion
//   - Typed objects (e.g., *corev1.Pod): conversion from unstructured to typed
func CreateTypedPredicate[O client.Object](filter func(O) bool) TypedPredicateFactory {
	// Determine at registration time if the predicate expects a typed object
	predicateNeedsConversion := typeRequiresConversion[O]()

	return func(
		scheme *runtime.Scheme,
		controllerName string,
	) predicate.Predicate {
		var conv *converter
		if predicateNeedsConversion {
			conv = &converter{scheme: scheme, controllerName: controllerName}
		}

		return &typedPredicateWrapper[O]{
			converter: conv,
			filter:    filter,
		}
	}
}

// typedPredicateWrapper wraps a typed filter function as a predicate.
// The generic type O is captured at registration time via CreateTypedPredicate.
type typedPredicateWrapper[O client.Object] struct {
	converter *converter // nil if no conversion needed
	filter    func(O) bool
}

func (tpw *typedPredicateWrapper[O]) Create(e event.TypedCreateEvent[client.Object]) bool {
	obj, ok := tpw.getTypedObject(e.Object)
	if !ok {
		return false
	}

	return tpw.filter(obj)
}

func (tpw *typedPredicateWrapper[O]) Update(e event.TypedUpdateEvent[client.Object]) bool {
	// For update, we check the new object
	obj, ok := tpw.getTypedObject(e.ObjectNew)
	if !ok {
		return false
	}

	return tpw.filter(obj)
}

func (tpw *typedPredicateWrapper[O]) Delete(e event.TypedDeleteEvent[client.Object]) bool {
	obj, ok := tpw.getTypedObject(e.Object)
	if !ok {
		return false
	}

	return tpw.filter(obj)
}

func (tpw *typedPredicateWrapper[O]) Generic(e event.TypedGenericEvent[client.Object]) bool {
	obj, ok := tpw.getTypedObject(e.Object)
	if !ok {
		return false
	}

	return tpw.filter(obj)
}

// getTypedObject converts the object to the expected type O.
// If converter is set, it converts first. Returns the typed object and true on success.
func (tpw *typedPredicateWrapper[O]) getTypedObject(obj client.Object) (O, bool) {
	var zero O

	if tpw.converter != nil {
		typed, err := tpw.converter.Convert(obj)
		if err != nil {
			return zero, false
		}

		obj = typed
	}

	result, ok := obj.(O)

	return result, ok
}

// typedMapperHandler wraps a typed mapper function without reflection.
// The generic type O is captured at registration time via CreateTypedMapperHandler.
// Predicates are evaluated internally after conversion to avoid double conversion.
type typedMapperHandler[O client.Object] struct {
	converter  *converter // nil if no conversion needed
	mapper     func(context.Context, O) []reconcile.Request
	predicates []predicate.Predicate
}

func (tmh *typedMapperHandler[O]) Create(
	ctx context.Context,
	e event.TypedCreateEvent[client.Object],
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	typedObj, convertedEvent, ok := tmh.processCreateEvent(e)
	if !ok {
		return
	}

	for _, req := range tmh.mapper(ctx, typedObj) {
		q.Add(req)
	}

	// Mark event as used to avoid compiler warning
	_ = convertedEvent
}

func (tmh *typedMapperHandler[O]) Update(
	ctx context.Context,
	e event.TypedUpdateEvent[client.Object],
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	typedObj, convertedEvent, ok := tmh.processUpdateEvent(e)
	if !ok {
		return
	}

	for _, req := range tmh.mapper(ctx, typedObj) {
		q.Add(req)
	}

	// Mark event as used to avoid compiler warning
	_ = convertedEvent
}

func (tmh *typedMapperHandler[O]) Delete(
	ctx context.Context,
	e event.TypedDeleteEvent[client.Object],
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	typedObj, convertedEvent, ok := tmh.processDeleteEvent(e)
	if !ok {
		return
	}

	for _, req := range tmh.mapper(ctx, typedObj) {
		q.Add(req)
	}

	// Mark event as used to avoid compiler warning
	_ = convertedEvent
}

func (tmh *typedMapperHandler[O]) Generic(
	ctx context.Context,
	e event.TypedGenericEvent[client.Object],
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	typedObj, convertedEvent, ok := tmh.processGenericEvent(e)
	if !ok {
		return
	}

	for _, req := range tmh.mapper(ctx, typedObj) {
		q.Add(req)
	}

	// Mark event as used to avoid compiler warning
	_ = convertedEvent
}

// processCreateEvent converts the object, evaluates predicates, and returns the typed object.
// Returns (typedObject, convertedEvent, ok). If ok is false, the event should be dropped.
func (tmh *typedMapperHandler[O]) processCreateEvent(
	e event.TypedCreateEvent[client.Object],
) (O, event.TypedCreateEvent[client.Object], bool) {
	var zero O

	obj, ok := tmh.getTypedObject(e.Object)
	if !ok {
		return zero, event.TypedCreateEvent[client.Object]{}, false
	}

	convertedEvent := event.TypedCreateEvent[client.Object]{Object: obj}

	for _, p := range tmh.predicates {
		if !p.Create(convertedEvent) {
			return zero, event.TypedCreateEvent[client.Object]{}, false
		}
	}

	return obj, convertedEvent, true
}

// processUpdateEvent converts both objects, evaluates predicates, and returns the new typed object.
// Returns (typedObjectNew, convertedEvent, ok). If ok is false, the event should be dropped.
// ObjectOld may be nil in some scenarios; it's converted only if non-nil.
func (tmh *typedMapperHandler[O]) processUpdateEvent(
	e event.TypedUpdateEvent[client.Object],
) (O, event.TypedUpdateEvent[client.Object], bool) {
	var zero O

	// ObjectOld is optional; convert only if present
	var objOld client.Object
	if e.ObjectOld != nil {
		typed, ok := tmh.getTypedObject(e.ObjectOld)
		if !ok {
			return zero, event.TypedUpdateEvent[client.Object]{}, false
		}

		objOld = typed
	}

	// ObjectNew is required
	objNew, ok := tmh.getTypedObject(e.ObjectNew)
	if !ok {
		return zero, event.TypedUpdateEvent[client.Object]{}, false
	}

	convertedEvent := event.TypedUpdateEvent[client.Object]{
		ObjectOld: objOld,
		ObjectNew: objNew,
	}

	for _, p := range tmh.predicates {
		if !p.Update(convertedEvent) {
			return zero, event.TypedUpdateEvent[client.Object]{}, false
		}
	}

	return objNew, convertedEvent, true
}

// processDeleteEvent converts the object, evaluates predicates, and returns the typed object.
// Returns (typedObject, convertedEvent, ok). If ok is false, the event should be dropped.
func (tmh *typedMapperHandler[O]) processDeleteEvent(
	e event.TypedDeleteEvent[client.Object],
) (O, event.TypedDeleteEvent[client.Object], bool) {
	var zero O

	obj, ok := tmh.getTypedObject(e.Object)
	if !ok {
		return zero, event.TypedDeleteEvent[client.Object]{}, false
	}

	convertedEvent := event.TypedDeleteEvent[client.Object]{
		Object:             obj,
		DeleteStateUnknown: e.DeleteStateUnknown,
	}

	for _, p := range tmh.predicates {
		if !p.Delete(convertedEvent) {
			return zero, event.TypedDeleteEvent[client.Object]{}, false
		}
	}

	return obj, convertedEvent, true
}

// processGenericEvent converts the object, evaluates predicates, and returns the typed object.
// Returns (typedObject, convertedEvent, ok). If ok is false, the event should be dropped.
func (tmh *typedMapperHandler[O]) processGenericEvent(
	e event.TypedGenericEvent[client.Object],
) (O, event.TypedGenericEvent[client.Object], bool) {
	var zero O

	obj, ok := tmh.getTypedObject(e.Object)
	if !ok {
		return zero, event.TypedGenericEvent[client.Object]{}, false
	}

	convertedEvent := event.TypedGenericEvent[client.Object]{Object: obj}

	for _, p := range tmh.predicates {
		if !p.Generic(convertedEvent) {
			return zero, event.TypedGenericEvent[client.Object]{}, false
		}
	}

	return obj, convertedEvent, true
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
