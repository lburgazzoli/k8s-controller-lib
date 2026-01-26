//nolint:testpackage // Testing internal functions requires same package
package builder

import (
	"context"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/workqueue"

	. "github.com/onsi/gomega"
)

func TestProcessObject_TypedObject(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	cm := &corev1.ConfigMap{}
	cm.SetName("test")
	cm.SetNamespace("default")

	// Test typed object without AsPartial
	watchObj, needsConversion, err := processObject(cm, scheme, false)

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(needsConversion).To(BeTrue(), "typed object should need conversion")
	g.Expect(watchObj).To(BeAssignableToTypeOf(&unstructured.Unstructured{}))

	u := watchObj.(*unstructured.Unstructured)
	gvk := u.GroupVersionKind()
	g.Expect(gvk.Version).To(Equal("v1"))
	g.Expect(gvk.Kind).To(Equal("ConfigMap"))
}

func TestProcessObject_TypedObjectWithAsPartial(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	cm := &corev1.ConfigMap{}

	// Test typed object with AsPartial
	watchObj, needsConversion, err := processObject(cm, scheme, true)

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(needsConversion).To(BeTrue(), "typed object should need conversion")
	g.Expect(watchObj).To(BeAssignableToTypeOf(&metav1.PartialObjectMetadata{}))

	p := watchObj.(*metav1.PartialObjectMetadata)
	gvk := p.GroupVersionKind()
	g.Expect(gvk.Version).To(Equal("v1"))
	g.Expect(gvk.Kind).To(Equal("ConfigMap"))
}

func TestProcessObject_Unstructured(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))

	// Test unstructured object
	watchObj, needsConversion, err := processObject(u, scheme, false)

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(needsConversion).To(BeFalse(), "unstructured should not need conversion")
	g.Expect(watchObj).To(BeAssignableToTypeOf(&unstructured.Unstructured{}))

	result := watchObj.(*unstructured.Unstructured)
	gvk := result.GroupVersionKind()
	g.Expect(gvk.Version).To(Equal("v1"))
	g.Expect(gvk.Kind).To(Equal("ConfigMap"))
}

func TestProcessObject_PartialMetadata(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()

	p := &metav1.PartialObjectMetadata{}
	p.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))

	// Test partial metadata object
	watchObj, needsConversion, err := processObject(p, scheme, false)

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(needsConversion).To(BeFalse(), "partial should not need conversion")
	g.Expect(watchObj).To(BeAssignableToTypeOf(&metav1.PartialObjectMetadata{}))

	result := watchObj.(*metav1.PartialObjectMetadata)
	gvk := result.GroupVersionKind()
	g.Expect(gvk.Version).To(Equal("v1"))
	g.Expect(gvk.Kind).To(Equal("ConfigMap"))
}

func TestProcessObject_UnknownType(t *testing.T) {
	g := NewWithT(t)

	// Empty scheme - ConfigMap type not registered
	scheme := runtime.NewScheme()

	// ConfigMap will fail GVK lookup because scheme is empty
	cm := &corev1.ConfigMap{}

	// Should return error when GVK lookup fails
	_, _, err := processObject(cm, scheme, false)

	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("failed to get GVK"))
	g.Expect(err.Error()).To(ContainSubstring("ConfigMap"))
}

func TestConverter_UnstructuredToTyped_Success(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	conv := &converter{scheme: scheme, controllerName: "test-controller"}

	// Create unstructured ConfigMap
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))
	u.SetName("test")
	u.SetNamespace("default")
	_ = unstructured.SetNestedMap(u.Object, map[string]any{
		"key": "value",
	}, "data")

	// Convert to typed
	result, err := conv.Convert(u)

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result).To(BeAssignableToTypeOf(&corev1.ConfigMap{}))

	cm := result.(*corev1.ConfigMap)
	g.Expect(cm.Name).To(Equal("test"))
	g.Expect(cm.Namespace).To(Equal("default"))
	g.Expect(cm.Data).To(HaveKey("key"))
}

func TestConverter_UnstructuredToTyped_Failure(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	// Don't register ConfigMap - will fail

	conv := &converter{scheme: scheme, controllerName: "test-controller"}

	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))

	// Convert should fail
	_, err := conv.Convert(u)

	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("failed to create target type"))
}

func TestConverter_PartialMetadata(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	conv := &converter{scheme: scheme, controllerName: "test-controller"}

	p := &metav1.PartialObjectMetadata{}
	p.SetName("test")

	// Partial should return unchanged
	result, err := conv.Convert(p)

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result).To(Equal(p))
}

func TestConverter_AlreadyTyped(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	conv := &converter{scheme: scheme, controllerName: "test-controller"}

	cm := &corev1.ConfigMap{}
	cm.SetName("test")

	// Already typed should return unchanged
	result, err := conv.Convert(cm)

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result).To(Equal(cm))
}

func TestWrapPredicates_NoConversion(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	pred1 := predicate.GenerationChangedPredicate{}
	pred2 := predicate.LabelChangedPredicate{}

	predicates := []predicate.Predicate{pred1, pred2}

	// No conversion needed
	wrapped := wrapPredicates(scheme, predicates, false, "test-controller")

	g.Expect(wrapped).To(HaveLen(2))
	g.Expect(wrapped).To(Equal(predicates), "should return original predicates")
}

func TestWrapPredicates_WithConversion(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	pred := predicate.GenerationChangedPredicate{}
	predicates := []predicate.Predicate{pred}

	// With conversion
	wrapped := wrapPredicates(scheme, predicates, true, "test-controller")

	g.Expect(wrapped).To(HaveLen(1))
	g.Expect(wrapped[0]).ToNot(Equal(pred), "should wrap predicate")
}

func TestPredicateWrapper_ConversionSuccess(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	// User predicate that checks typed object
	called := false
	userPred := predicate.TypedFuncs[client.Object]{
		CreateFunc: func(_ event.TypedCreateEvent[client.Object]) bool {
			called = true

			return true
		},
	}

	wrapped := wrapPredicates(scheme, []predicate.Predicate{userPred}, true, "test-controller")

	// Create unstructured event
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))
	u.SetName("test")

	e := event.TypedCreateEvent[client.Object]{Object: u}

	// Call wrapped predicate
	result := wrapped[0].Create(e)

	g.Expect(called).To(BeTrue(), "user predicate should be called")
	g.Expect(result).To(BeTrue())
}

func TestPredicateWrapper_ConversionFailure(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	// Don't register ConfigMap

	called := false
	userPred := predicate.TypedFuncs[client.Object]{
		CreateFunc: func(_ event.TypedCreateEvent[client.Object]) bool {
			called = true

			return true
		},
	}

	wrapped := wrapPredicates(scheme, []predicate.Predicate{userPred}, true, "test-controller")

	// Create unstructured event for unregistered type
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))

	e := event.TypedCreateEvent[client.Object]{Object: u}

	// Call wrapped predicate - should return false due to conversion failure
	result := wrapped[0].Create(e)

	g.Expect(called).To(BeFalse(), "user predicate should not be called on conversion failure")
	g.Expect(result).To(BeFalse(), "should return false to drop event")
}

func TestWrapHandler_NoConversion(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	h := handler.TypedFuncs[client.Object, reconcile.Request]{}

	// No conversion needed
	wrapped := wrapHandler(scheme, h, false, "test-controller")

	g.Expect(wrapped).To(Equal(h), "should return original handler")
}

func TestWrapHandler_WithConversion(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	h := handler.TypedFuncs[client.Object, reconcile.Request]{}

	// With conversion
	wrapped := wrapHandler(scheme, h, true, "test-controller")

	g.Expect(wrapped).ToNot(Equal(h), "should wrap handler")
}

func TestHandlerWrapper_ConversionSuccess(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	called := false
	userHandler := handler.TypedFuncs[client.Object, reconcile.Request]{
		CreateFunc: func(
			_ context.Context,
			_ event.TypedCreateEvent[client.Object],
			_ workqueue.TypedRateLimitingInterface[reconcile.Request],
		) {
			called = true
		},
	}

	wrapped := wrapHandler(scheme, userHandler, true, "test-controller")

	// Create unstructured event
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))

	e := event.TypedCreateEvent[client.Object]{Object: u}
	q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())

	// Call wrapped handler
	wrapped.Create(context.Background(), e, q)

	g.Expect(called).To(BeTrue(), "user handler should be called")
}

func TestHandlerWrapper_ConversionFailure(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	// Don't register ConfigMap

	called := false
	userHandler := handler.TypedFuncs[client.Object, reconcile.Request]{
		CreateFunc: func(
			_ context.Context,
			_ event.TypedCreateEvent[client.Object],
			_ workqueue.TypedRateLimitingInterface[reconcile.Request],
		) {
			called = true
		},
	}

	wrapped := wrapHandler(scheme, userHandler, true, "test-controller")

	// Create unstructured event for unregistered type
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))

	e := event.TypedCreateEvent[client.Object]{Object: u}
	q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())

	// Call wrapped handler - should not enqueue due to conversion failure
	wrapped.Create(context.Background(), e, q)

	g.Expect(called).To(BeFalse(), "user handler should not be called on conversion failure")
	g.Expect(q.Len()).To(Equal(0), "queue should be empty")
}

// Tests for convertingHandler - combined predicate+handler wrapper

func TestConvertingHandler_ConvertsOnce(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	// Track calls to verify single conversion path
	predicateCalled := false
	handlerCalled := false

	userPred := predicate.Funcs{
		CreateFunc: func(e event.TypedCreateEvent[client.Object]) bool {
			predicateCalled = true
			// Verify we receive typed object, not unstructured
			_, ok := e.Object.(*corev1.ConfigMap)
			g.Expect(ok).To(BeTrue(), "predicate should receive typed ConfigMap")

			return true
		},
	}

	userHandler := handler.TypedFuncs[client.Object, reconcile.Request]{
		CreateFunc: func(
			_ context.Context,
			e event.TypedCreateEvent[client.Object],
			_ workqueue.TypedRateLimitingInterface[reconcile.Request],
		) {
			handlerCalled = true
			// Verify we receive typed object, not unstructured
			_, ok := e.Object.(*corev1.ConfigMap)
			g.Expect(ok).To(BeTrue(), "handler should receive typed ConfigMap")
		},
	}

	combined := newConvertingHandler(scheme, userHandler, []predicate.Predicate{userPred}, "test-controller")

	// Create unstructured event (needs conversion)
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))
	u.SetName("test")
	u.SetNamespace("default")

	e := event.TypedCreateEvent[client.Object]{Object: u}
	q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())

	combined.Create(context.Background(), e, q)

	g.Expect(predicateCalled).To(BeTrue(), "predicate should be called")
	g.Expect(handlerCalled).To(BeTrue(), "handler should be called")
}

func TestConvertingHandler_PredicateFilters(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	handlerCalled := false

	// Predicate that rejects all events
	rejectPred := predicate.Funcs{
		CreateFunc: func(_ event.TypedCreateEvent[client.Object]) bool {
			return false
		},
	}

	userHandler := handler.TypedFuncs[client.Object, reconcile.Request]{
		CreateFunc: func(
			_ context.Context,
			_ event.TypedCreateEvent[client.Object],
			_ workqueue.TypedRateLimitingInterface[reconcile.Request],
		) {
			handlerCalled = true
		},
	}

	combined := newConvertingHandler(scheme, userHandler, []predicate.Predicate{rejectPred}, "test-controller")

	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))
	u.SetName("test")

	e := event.TypedCreateEvent[client.Object]{Object: u}
	q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())

	combined.Create(context.Background(), e, q)

	g.Expect(handlerCalled).To(BeFalse(), "handler should not be called when predicate rejects")
}

func TestConvertingHandler_MultiplePredicates(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	pred1Called := false
	pred2Called := false
	handlerCalled := false

	pred1 := predicate.Funcs{
		CreateFunc: func(_ event.TypedCreateEvent[client.Object]) bool {
			pred1Called = true

			return true
		},
	}

	pred2 := predicate.Funcs{
		CreateFunc: func(_ event.TypedCreateEvent[client.Object]) bool {
			pred2Called = true

			return true
		},
	}

	userHandler := handler.TypedFuncs[client.Object, reconcile.Request]{
		CreateFunc: func(
			_ context.Context,
			_ event.TypedCreateEvent[client.Object],
			_ workqueue.TypedRateLimitingInterface[reconcile.Request],
		) {
			handlerCalled = true
		},
	}

	combined := newConvertingHandler(scheme, userHandler, []predicate.Predicate{pred1, pred2}, "test-controller")

	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))
	u.SetName("test")

	e := event.TypedCreateEvent[client.Object]{Object: u}
	q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())

	combined.Create(context.Background(), e, q)

	g.Expect(pred1Called).To(BeTrue(), "first predicate should be called")
	g.Expect(pred2Called).To(BeTrue(), "second predicate should be called")
	g.Expect(handlerCalled).To(BeTrue(), "handler should be called when all predicates pass")
}

func TestConvertingHandler_ConversionFailure(t *testing.T) {
	g := NewWithT(t)

	// Empty scheme - ConfigMap not registered
	scheme := runtime.NewScheme()

	predicateCalled := false
	handlerCalled := false

	userPred := predicate.Funcs{
		CreateFunc: func(_ event.TypedCreateEvent[client.Object]) bool {
			predicateCalled = true

			return true
		},
	}

	userHandler := handler.TypedFuncs[client.Object, reconcile.Request]{
		CreateFunc: func(
			_ context.Context,
			_ event.TypedCreateEvent[client.Object],
			_ workqueue.TypedRateLimitingInterface[reconcile.Request],
		) {
			handlerCalled = true
		},
	}

	combined := newConvertingHandler(scheme, userHandler, []predicate.Predicate{userPred}, "test-controller")

	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))
	u.SetName("test")

	e := event.TypedCreateEvent[client.Object]{Object: u}
	q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())

	combined.Create(context.Background(), e, q)

	g.Expect(predicateCalled).To(BeFalse(), "predicate should not be called on conversion failure")
	g.Expect(handlerCalled).To(BeFalse(), "handler should not be called on conversion failure")
}

func TestConvertingHandler_UpdateEvent(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	var receivedOldName, receivedNewName string

	userPred := predicate.Funcs{
		UpdateFunc: func(e event.TypedUpdateEvent[client.Object]) bool {
			if e.ObjectOld != nil {
				receivedOldName = e.ObjectOld.GetName()
			}
			receivedNewName = e.ObjectNew.GetName()

			return true
		},
	}

	userHandler := handler.TypedFuncs[client.Object, reconcile.Request]{
		UpdateFunc: func(
			_ context.Context,
			_ event.TypedUpdateEvent[client.Object],
			_ workqueue.TypedRateLimitingInterface[reconcile.Request],
		) {
		},
	}

	combined := newConvertingHandler(scheme, userHandler, []predicate.Predicate{userPred}, "test-controller")

	oldU := &unstructured.Unstructured{}
	oldU.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))
	oldU.SetName("old-name")

	newU := &unstructured.Unstructured{}
	newU.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))
	newU.SetName("new-name")

	e := event.TypedUpdateEvent[client.Object]{ObjectOld: oldU, ObjectNew: newU}
	q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())

	combined.Update(context.Background(), e, q)

	g.Expect(receivedOldName).To(Equal("old-name"))
	g.Expect(receivedNewName).To(Equal("new-name"))
}

func TestTypedMapperHandler_Success(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	called := false
	mapper := func(_ context.Context, cm *corev1.ConfigMap) []reconcile.Request {
		called = true
		g.Expect(cm.GetName()).To(Equal("test"))

		return []reconcile.Request{
			{NamespacedName: client.ObjectKey{Name: "test", Namespace: "default"}},
		}
	}

	// Create handler factory and invoke it
	factory := CreateTypedMapperHandler(mapper)
	h := factory.Create(scheme, false, "test-controller", nil)

	cm := &corev1.ConfigMap{}
	cm.SetName("test")

	e := event.TypedCreateEvent[client.Object]{Object: cm}
	q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())

	h.Create(context.Background(), e, q)

	g.Expect(called).To(BeTrue(), "mapper should be called")
	g.Expect(q.Len()).To(Equal(1), "request should be enqueued")
}

func TestTypedMapperHandler_WithConversion(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	called := false
	mapper := func(_ context.Context, cm *corev1.ConfigMap) []reconcile.Request {
		called = true
		g.Expect(cm.GetName()).To(Equal("test"))

		return []reconcile.Request{
			{NamespacedName: client.ObjectKey{Name: "test", Namespace: "default"}},
		}
	}

	// Create handler with conversion enabled
	factory := CreateTypedMapperHandler(mapper)
	h := factory.Create(scheme, true, "test-controller", nil)

	// Create unstructured event (needs conversion)
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))
	u.SetName("test")

	e := event.TypedCreateEvent[client.Object]{Object: u}
	q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())

	h.Create(context.Background(), e, q)

	g.Expect(called).To(BeTrue(), "mapper should be called after conversion")
	g.Expect(q.Len()).To(Equal(1), "request should be enqueued")
}

func TestTypedMapperHandler_TypeMismatch(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	called := false
	// Mapper expects Secret but we'll pass ConfigMap
	mapper := func(_ context.Context, _ *corev1.Secret) []reconcile.Request {
		called = true

		return nil
	}

	factory := CreateTypedMapperHandler(mapper)
	h := factory.Create(scheme, false, "test-controller", nil)

	// Pass ConfigMap instead of Secret
	cm := &corev1.ConfigMap{}
	cm.SetName("test")

	e := event.TypedCreateEvent[client.Object]{Object: cm}
	q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())

	// Should not panic, just drop event due to type mismatch
	h.Create(context.Background(), e, q)

	g.Expect(called).To(BeFalse(), "mapper should not be called on type mismatch")
	g.Expect(q.Len()).To(Equal(0), "queue should be empty")
}

func TestTypedMapperHandler_AllEventTypes(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	createCalled := false
	updateCalled := false
	deleteCalled := false
	genericCalled := false

	mapper := func(_ context.Context, cm *corev1.ConfigMap) []reconcile.Request {
		switch cm.GetName() {
		case "create":
			createCalled = true
		case "update":
			updateCalled = true
		case "delete":
			deleteCalled = true
		case "generic":
			genericCalled = true
		}

		return nil
	}

	factory := CreateTypedMapperHandler(mapper)
	h := factory.Create(scheme, false, "test-controller", nil)
	q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())

	// Test Create
	cm1 := &corev1.ConfigMap{}
	cm1.SetName("create")
	h.Create(context.Background(), event.TypedCreateEvent[client.Object]{Object: cm1}, q)

	// Test Update
	cm2 := &corev1.ConfigMap{}
	cm2.SetName("update")
	h.Update(context.Background(), event.TypedUpdateEvent[client.Object]{ObjectNew: cm2}, q)

	// Test Delete
	cm3 := &corev1.ConfigMap{}
	cm3.SetName("delete")
	h.Delete(context.Background(), event.TypedDeleteEvent[client.Object]{Object: cm3}, q)

	// Test Generic
	cm4 := &corev1.ConfigMap{}
	cm4.SetName("generic")
	h.Generic(context.Background(), event.TypedGenericEvent[client.Object]{Object: cm4}, q)

	g.Expect(createCalled).To(BeTrue())
	g.Expect(updateCalled).To(BeTrue())
	g.Expect(deleteCalled).To(BeTrue())
	g.Expect(genericCalled).To(BeTrue())
}

func TestTypedMapperHandler_WithPredicates(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	mapperCalled := false
	predicateCalled := false

	mapper := func(_ context.Context, cm *corev1.ConfigMap) []reconcile.Request {
		mapperCalled = true
		// Verify we receive typed object
		g.Expect(cm.GetName()).To(Equal("test"))

		return []reconcile.Request{
			{NamespacedName: client.ObjectKey{Name: "test", Namespace: "default"}},
		}
	}

	pred := predicate.Funcs{
		CreateFunc: func(e event.TypedCreateEvent[client.Object]) bool {
			predicateCalled = true
			// Verify predicate receives typed object (conversion happened before predicate)
			_, ok := e.Object.(*corev1.ConfigMap)
			g.Expect(ok).To(BeTrue(), "predicate should receive typed ConfigMap")

			return true
		},
	}

	// Create handler with conversion enabled and predicates
	factory := CreateTypedMapperHandler(mapper)
	h := factory.Create(scheme, true, "test-controller", []predicate.Predicate{pred})

	// Create unstructured event (needs conversion)
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))
	u.SetName("test")

	e := event.TypedCreateEvent[client.Object]{Object: u}
	q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())

	h.Create(context.Background(), e, q)

	g.Expect(predicateCalled).To(BeTrue(), "predicate should be called")
	g.Expect(mapperCalled).To(BeTrue(), "mapper should be called after predicate passes")
	g.Expect(q.Len()).To(Equal(1), "request should be enqueued")
}

func TestTypedMapperHandler_PredicateRejects(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	mapperCalled := false

	mapper := func(_ context.Context, _ *corev1.ConfigMap) []reconcile.Request {
		mapperCalled = true

		return nil
	}

	// Predicate that rejects all events
	pred := predicate.Funcs{
		CreateFunc: func(_ event.TypedCreateEvent[client.Object]) bool {
			return false
		},
	}

	factory := CreateTypedMapperHandler(mapper)
	h := factory.Create(scheme, true, "test-controller", []predicate.Predicate{pred})

	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))
	u.SetName("test")

	e := event.TypedCreateEvent[client.Object]{Object: u}
	q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())

	h.Create(context.Background(), e, q)

	g.Expect(mapperCalled).To(BeFalse(), "mapper should not be called when predicate rejects")
	g.Expect(q.Len()).To(Equal(0), "queue should be empty")
}

// Zero-conversion tests - verify that using unstructured directly avoids conversion overhead

func TestUnstructuredWatch_NoConversionNeeded(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	// When watching *unstructured.Unstructured, processObject should return needsConversion=false
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Pod"))

	watchObj, needsConversion, err := processObject(u, scheme, false)

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(needsConversion).To(BeFalse(), "unstructured watch should NOT need conversion")
	g.Expect(watchObj).To(BeAssignableToTypeOf(&unstructured.Unstructured{}))
}

func TestTypedMapperHandler_UnstructuredNoConversion(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	// Note: We don't need to register corev1 because we're not converting

	mapperCalled := false
	var receivedObj *unstructured.Unstructured

	// Mapper that expects *unstructured.Unstructured - no conversion needed
	mapper := func(_ context.Context, u *unstructured.Unstructured) []reconcile.Request {
		mapperCalled = true
		receivedObj = u

		return []reconcile.Request{
			{NamespacedName: client.ObjectKey{Name: u.GetName(), Namespace: u.GetNamespace()}},
		}
	}

	// Create handler with conversion=false (simulating unstructured watch)
	factory := CreateTypedMapperHandler(mapper)
	h := factory.Create(scheme, false, "test-controller", nil) // needsConversion=false!

	// Create unstructured event (same type as watch)
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Pod"))
	u.SetName("my-pod")
	u.SetNamespace("default")
	u.Object["spec"] = map[string]any{
		"containers": []any{
			map[string]any{"name": "nginx", "image": "nginx:latest"},
		},
	}

	e := event.TypedCreateEvent[client.Object]{Object: u}
	q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())

	h.Create(context.Background(), e, q)

	g.Expect(mapperCalled).To(BeTrue(), "mapper should be called")
	g.Expect(receivedObj).ToNot(BeNil())
	g.Expect(receivedObj.GetName()).To(Equal("my-pod"))
	g.Expect(receivedObj.GetNamespace()).To(Equal("default"))
	// Verify we received the SAME object (no conversion)
	g.Expect(receivedObj).To(BeIdenticalTo(u), "should receive same object without conversion")
	g.Expect(q.Len()).To(Equal(1))
}

func TestTypedMapperHandler_UnstructuredWithPredicates_NoConversion(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()

	predicateCalled := false
	mapperCalled := false
	var predicateReceivedObj client.Object
	var mapperReceivedObj *unstructured.Unstructured

	// Predicate that works with client.Object (generic)
	pred := predicate.Funcs{
		CreateFunc: func(e event.TypedCreateEvent[client.Object]) bool {
			predicateCalled = true
			predicateReceivedObj = e.Object

			return true
		},
	}

	// Mapper that expects *unstructured.Unstructured
	mapper := func(_ context.Context, u *unstructured.Unstructured) []reconcile.Request {
		mapperCalled = true
		mapperReceivedObj = u

		return nil
	}

	// Create handler with conversion=false and predicates
	factory := CreateTypedMapperHandler(mapper)
	h := factory.Create(scheme, false, "test-controller", []predicate.Predicate{pred})

	// Create unstructured event
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Pod"))
	u.SetName("my-pod")
	u.SetNamespace("default")

	e := event.TypedCreateEvent[client.Object]{Object: u}
	q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())

	h.Create(context.Background(), e, q)

	g.Expect(predicateCalled).To(BeTrue(), "predicate should be called")
	g.Expect(mapperCalled).To(BeTrue(), "mapper should be called")

	// Both predicate and mapper should receive the SAME unstructured object (no conversion)
	g.Expect(predicateReceivedObj).To(BeIdenticalTo(u), "predicate should receive same object")
	g.Expect(mapperReceivedObj).To(BeIdenticalTo(u), "mapper should receive same object")
}

func TestWrapPredicates_UnstructuredNoConversion(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()

	predicateCalled := false
	var receivedObj client.Object

	pred := predicate.Funcs{
		CreateFunc: func(e event.TypedCreateEvent[client.Object]) bool {
			predicateCalled = true
			receivedObj = e.Object

			return true
		},
	}

	// Create original slice
	originalSlice := []predicate.Predicate{pred}

	// When needsConversion=false, wrapPredicates returns original predicates unchanged
	wrapped := wrapPredicates(scheme, originalSlice, false, "test-controller")

	g.Expect(wrapped).To(HaveLen(1))
	// Verify it's the exact same slice (not a copy or wrapped version)
	// We can check this by verifying the slice header points to the same backing array
	g.Expect(&wrapped[0]).To(BeIdenticalTo(&originalSlice[0]), "should return same slice when no conversion needed")

	// Create unstructured event
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Pod"))
	u.SetName("my-pod")

	e := event.TypedCreateEvent[client.Object]{Object: u}

	result := wrapped[0].Create(e)

	g.Expect(result).To(BeTrue())
	g.Expect(predicateCalled).To(BeTrue())
	// Predicate receives the SAME unstructured object (no conversion)
	g.Expect(receivedObj).To(BeIdenticalTo(u), "predicate should receive same object")
}

func TestWrapHandler_UnstructuredNoConversion(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()

	handlerCalled := false
	var receivedObj client.Object

	userHandler := handler.TypedFuncs[client.Object, reconcile.Request]{
		CreateFunc: func(
			_ context.Context,
			e event.TypedCreateEvent[client.Object],
			_ workqueue.TypedRateLimitingInterface[reconcile.Request],
		) {
			handlerCalled = true
			receivedObj = e.Object
		},
	}

	// When needsConversion=false, wrapHandler returns original handler unchanged
	wrapped := wrapHandler(scheme, userHandler, false, "test-controller")

	// Verify it's not a wrapper type (original handler returned)
	_, isWrapper := wrapped.(*handlerWrapper)
	g.Expect(isWrapper).To(BeFalse(), "should return original handler, not wrapped")

	// Create unstructured event
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Pod"))
	u.SetName("my-pod")

	e := event.TypedCreateEvent[client.Object]{Object: u}
	q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())

	wrapped.Create(context.Background(), e, q)

	g.Expect(handlerCalled).To(BeTrue())
	// Handler receives the SAME unstructured object (no conversion)
	g.Expect(receivedObj).To(BeIdenticalTo(u), "handler should receive same object")
}

func TestZeroConversionPath_EndToEnd(t *testing.T) {
	// This test documents the zero-conversion optimization path:
	// With the GVK-based API, Watches() and Owns() always use unstructured (or partial metadata)
	// internally, so predicates and handlers receive unstructured objects directly without
	// any conversion overhead.
	//
	// Usage pattern:
	//   b.Watches(gvks.Pod, WithMapper(func(ctx, *unstructured.Unstructured) []reconcile.Request {...}))
	//   // or with AsPartial():
	//   b.Watches(gvks.Pod, WithMapper(func(ctx, *metav1.PartialObjectMetadata) []reconcile.Request {...}), AsPartial())
	//
	// This avoids the conversion overhead entirely because:
	// 1. GVK-based watches always create unstructured/partial objects internally
	// 2. processObject() returns needsConversion=false for unstructured/partial
	// 3. wrapPredicates() returns original predicates unchanged
	// 4. wrapHandler() returns original handler unchanged
	// 5. CreateTypedMapperHandler() with needsConversion=false creates no converter

	g := NewWithT(t)

	scheme := runtime.NewScheme()

	// Step 1: Verify processObject returns needsConversion=false
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Pod"))

	_, needsConversion, err := processObject(u, scheme, false)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(needsConversion).To(BeFalse(), "unstructured should not need conversion")

	// Step 2: Verify wrapPredicates passes through unchanged
	pred := predicate.GenerationChangedPredicate{}
	wrappedPreds := wrapPredicates(scheme, []predicate.Predicate{pred}, false, "test")
	g.Expect(wrappedPreds[0]).To(Equal(pred), "predicate should pass through unchanged")

	// Step 3: Verify wrapHandler passes through unchanged
	h := handler.TypedFuncs[client.Object, reconcile.Request]{}
	wrappedHandler := wrapHandler(scheme, h, false, "test")
	g.Expect(wrappedHandler).To(Equal(h), "handler should pass through unchanged")

	// Step 4: Verify typedMapperHandler with needsConversion=false has no converter
	mapper := func(_ context.Context, u *unstructured.Unstructured) []reconcile.Request { return nil }
	factory := CreateTypedMapperHandler(mapper)
	mapperHandler := factory.Create(scheme, false, "test", nil)

	// The handler should work with unstructured directly
	testU := &unstructured.Unstructured{}
	testU.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Pod"))
	testU.SetName("test-pod")

	e := event.TypedCreateEvent[client.Object]{Object: testU}
	q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())

	// Should not panic or fail - works directly with unstructured
	mapperHandler.Create(context.Background(), e, q)
}

// Metric tests

func TestConverter_Metrics_TypeNotInScheme(t *testing.T) {
	g := NewWithT(t)

	// Use unique controller name to avoid interference with other tests
	controllerName := "test-metrics-type-not-in-scheme"

	// Get initial metric value
	initialValue := testutil.ToFloat64(ConversionErrorsTotal.WithLabelValues(
		controllerName,
		"v1",
		"ConfigMap",
		ReasonTypeNotInScheme,
	))

	// Empty scheme - ConfigMap not registered
	scheme := runtime.NewScheme()
	conv := &converter{scheme: scheme, controllerName: controllerName}

	// Create unstructured ConfigMap
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))
	u.SetName("test")

	// Convert should fail and increment metric
	_, err := conv.Convert(u)
	g.Expect(err).To(HaveOccurred())

	// Verify metric incremented
	newValue := testutil.ToFloat64(ConversionErrorsTotal.WithLabelValues(
		controllerName,
		"v1",
		"ConfigMap",
		ReasonTypeNotInScheme,
	))
	g.Expect(newValue).To(Equal(initialValue + 1))
}

func TestConverter_Metrics_CorrectLabels(t *testing.T) {
	g := NewWithT(t)

	// Use unique controller name
	controllerName := "test-metrics-labels"

	// Get initial values for different label combinations
	initialConfigMap := testutil.ToFloat64(ConversionErrorsTotal.WithLabelValues(
		controllerName,
		"v1",
		"ConfigMap",
		ReasonTypeNotInScheme,
	))
	initialSecret := testutil.ToFloat64(ConversionErrorsTotal.WithLabelValues(
		controllerName,
		"v1",
		"Secret",
		ReasonTypeNotInScheme,
	))

	// Empty scheme
	scheme := runtime.NewScheme()
	conv := &converter{scheme: scheme, controllerName: controllerName}

	// Trigger error for ConfigMap
	cmUnstructured := &unstructured.Unstructured{}
	cmUnstructured.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))
	_, _ = conv.Convert(cmUnstructured)

	// Trigger error for Secret
	secretUnstructured := &unstructured.Unstructured{}
	secretUnstructured.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Secret"))
	_, _ = conv.Convert(secretUnstructured)

	// Verify ConfigMap metric incremented
	newConfigMap := testutil.ToFloat64(ConversionErrorsTotal.WithLabelValues(
		controllerName,
		"v1",
		"ConfigMap",
		ReasonTypeNotInScheme,
	))
	g.Expect(newConfigMap).To(Equal(initialConfigMap + 1))

	// Verify Secret metric incremented separately
	newSecret := testutil.ToFloat64(ConversionErrorsTotal.WithLabelValues(
		controllerName,
		"v1",
		"Secret",
		ReasonTypeNotInScheme,
	))
	g.Expect(newSecret).To(Equal(initialSecret + 1))
}

func TestConverter_Metrics_DifferentControllers(t *testing.T) {
	g := NewWithT(t)

	// Use unique controller names
	controller1 := "test-metrics-controller-1"
	controller2 := "test-metrics-controller-2"

	// Get initial values
	initial1 := testutil.ToFloat64(ConversionErrorsTotal.WithLabelValues(
		controller1,
		"v1",
		"ConfigMap",
		ReasonTypeNotInScheme,
	))
	initial2 := testutil.ToFloat64(ConversionErrorsTotal.WithLabelValues(
		controller2,
		"v1",
		"ConfigMap",
		ReasonTypeNotInScheme,
	))

	// Empty scheme
	scheme := runtime.NewScheme()
	conv1 := &converter{scheme: scheme, controllerName: controller1}
	conv2 := &converter{scheme: scheme, controllerName: controller2}

	// Trigger errors from different controllers
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))

	_, _ = conv1.Convert(u)
	_, _ = conv1.Convert(u) // controller1 gets 2 errors
	_, _ = conv2.Convert(u) // controller2 gets 1 error

	// Verify metrics are tracked separately per controller
	new1 := testutil.ToFloat64(ConversionErrorsTotal.WithLabelValues(
		controller1,
		"v1",
		"ConfigMap",
		ReasonTypeNotInScheme,
	))
	new2 := testutil.ToFloat64(ConversionErrorsTotal.WithLabelValues(
		controller2,
		"v1",
		"ConfigMap",
		ReasonTypeNotInScheme,
	))

	g.Expect(new1).To(Equal(initial1 + 2))
	g.Expect(new2).To(Equal(initial2 + 1))
}

// Tests for automatic conversion detection based on mapper's generic type

func TestTypeRequiresConversion_Unstructured(t *testing.T) {
	g := NewWithT(t)

	// *unstructured.Unstructured should not require conversion
	result := typeRequiresConversion[*unstructured.Unstructured]()
	g.Expect(result).To(BeFalse(), "*unstructured.Unstructured should not require conversion")
}

func TestTypeRequiresConversion_PartialMetadata(t *testing.T) {
	g := NewWithT(t)

	// *metav1.PartialObjectMetadata should not require conversion
	result := typeRequiresConversion[*metav1.PartialObjectMetadata]()
	g.Expect(result).To(BeFalse(), "*metav1.PartialObjectMetadata should not require conversion")
}

func TestTypeRequiresConversion_ClientObject(t *testing.T) {
	g := NewWithT(t)

	// client.Object interface should not require conversion
	result := typeRequiresConversion[client.Object]()
	g.Expect(result).To(BeFalse(), "client.Object should not require conversion")
}

func TestTypeRequiresConversion_TypedObject(t *testing.T) {
	g := NewWithT(t)

	// Typed objects should require conversion
	result := typeRequiresConversion[*corev1.ConfigMap]()
	g.Expect(result).To(BeTrue(), "*corev1.ConfigMap should require conversion")

	result = typeRequiresConversion[*corev1.Pod]()
	g.Expect(result).To(BeTrue(), "*corev1.Pod should require conversion")

	result = typeRequiresConversion[*corev1.Secret]()
	g.Expect(result).To(BeTrue(), "*corev1.Secret should require conversion")
}

func TestTypedMapperHandler_AutoConversion(t *testing.T) {
	// This test verifies that when using a typed mapper with a GVK-based watch,
	// conversion is automatically enabled based on the mapper's generic type.
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	mapperCalled := false
	var receivedName string
	var receivedData map[string]string

	// Mapper that expects *corev1.ConfigMap - should trigger automatic conversion
	mapper := func(_ context.Context, cm *corev1.ConfigMap) []reconcile.Request {
		mapperCalled = true
		receivedName = cm.GetName()
		receivedData = cm.Data

		return []reconcile.Request{
			{NamespacedName: client.ObjectKey{Name: cm.GetName(), Namespace: cm.GetNamespace()}},
		}
	}

	// Create handler with needsConversion=false (simulating GVK-based watch)
	// The handler should automatically enable conversion because the mapper expects *corev1.ConfigMap
	factory := CreateTypedMapperHandler(mapper)
	h := factory.Create(scheme, false, "test-controller", nil) // needsConversion=false from caller

	// Create unstructured event (what we'd receive from a GVK-based watch)
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))
	u.SetName("my-configmap")
	u.SetNamespace("default")
	u.Object["data"] = map[string]any{
		"key1": "value1",
		"key2": "value2",
	}

	e := event.TypedCreateEvent[client.Object]{Object: u}
	q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())

	h.Create(context.Background(), e, q)

	g.Expect(mapperCalled).To(BeTrue(), "mapper should be called")
	g.Expect(receivedName).To(Equal("my-configmap"), "should receive converted ConfigMap with correct name")
	g.Expect(receivedData).To(HaveKeyWithValue("key1", "value1"), "should receive converted ConfigMap with data")
	g.Expect(q.Len()).To(Equal(1), "should enqueue request")
}

func TestTypedMapperHandler_NoAutoConversionForUnstructured(t *testing.T) {
	// Verify that unstructured mappers don't get unnecessary conversion
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	// Note: Not registering corev1 - if conversion was attempted, it would fail

	mapperCalled := false
	var receivedObj *unstructured.Unstructured

	// Mapper that expects *unstructured.Unstructured - should NOT trigger conversion
	mapper := func(_ context.Context, u *unstructured.Unstructured) []reconcile.Request {
		mapperCalled = true
		receivedObj = u

		return nil
	}

	// Create handler with needsConversion=false
	// Should stay false because mapper expects unstructured
	factory := CreateTypedMapperHandler(mapper)
	h := factory.Create(scheme, false, "test-controller", nil)

	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))
	u.SetName("test")

	e := event.TypedCreateEvent[client.Object]{Object: u}
	q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())

	h.Create(context.Background(), e, q)

	g.Expect(mapperCalled).To(BeTrue(), "mapper should be called")
	// Verify we received the SAME object (no conversion)
	g.Expect(receivedObj).To(BeIdenticalTo(u), "should receive same unstructured object without conversion")
}

func TestTypedMapperHandler_NoAutoConversionForClientObject(t *testing.T) {
	// Verify that client.Object mappers don't get unnecessary conversion
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	// Note: Not registering corev1 - if conversion was attempted, it would fail

	mapperCalled := false
	var receivedObj client.Object

	// Mapper that expects client.Object interface - should NOT trigger conversion
	mapper := func(_ context.Context, obj client.Object) []reconcile.Request {
		mapperCalled = true
		receivedObj = obj

		return nil
	}

	// Create handler with needsConversion=false
	// Should stay false because mapper expects client.Object interface
	factory := CreateTypedMapperHandler(mapper)
	h := factory.Create(scheme, false, "test-controller", nil)

	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Secret"))
	u.SetName("my-secret")

	e := event.TypedCreateEvent[client.Object]{Object: u}
	q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())

	h.Create(context.Background(), e, q)

	g.Expect(mapperCalled).To(BeTrue(), "mapper should be called")
	// Verify we received the SAME object (no conversion)
	g.Expect(receivedObj).To(BeIdenticalTo(u), "should receive same object without conversion")
}

// Tests for MapperExpectation

func TestMapperExpectation_Unstructured(t *testing.T) {
	g := NewWithT(t)

	mapper := func(_ context.Context, _ *unstructured.Unstructured) []reconcile.Request { return nil }
	factory := CreateTypedMapperHandler(mapper)

	g.Expect(factory.Expectation).To(Equal(MapperExpectsUnstructured))
}

func TestMapperExpectation_PartialMetadata(t *testing.T) {
	g := NewWithT(t)

	mapper := func(_ context.Context, _ *metav1.PartialObjectMetadata) []reconcile.Request { return nil }
	factory := CreateTypedMapperHandler(mapper)

	g.Expect(factory.Expectation).To(Equal(MapperExpectsPartial))
}

func TestMapperExpectation_ClientObject(t *testing.T) {
	g := NewWithT(t)

	mapper := func(_ context.Context, _ client.Object) []reconcile.Request { return nil }
	factory := CreateTypedMapperHandler(mapper)

	g.Expect(factory.Expectation).To(Equal(MapperExpectsClientObject))
}

func TestMapperExpectation_Typed(t *testing.T) {
	g := NewWithT(t)

	mapper := func(_ context.Context, _ *corev1.Pod) []reconcile.Request { return nil }
	factory := CreateTypedMapperHandler(mapper)

	g.Expect(factory.Expectation).To(Equal(MapperExpectsTyped))
}

// Tests for TypedPredicate

func TestTypedPredicate_Unstructured(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()

	filterCalled := false
	var receivedObj *unstructured.Unstructured

	// Filter that expects *unstructured.Unstructured - no conversion needed
	filter := func(u *unstructured.Unstructured) bool {
		filterCalled = true
		receivedObj = u

		return true
	}

	factory := CreateTypedPredicate(filter)
	pred := factory(scheme, "test-controller")

	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Pod"))
	u.SetName("my-pod")

	e := event.TypedCreateEvent[client.Object]{Object: u}
	result := pred.Create(e)

	g.Expect(result).To(BeTrue())
	g.Expect(filterCalled).To(BeTrue())
	// Verify we received the SAME object (no conversion)
	g.Expect(receivedObj).To(BeIdenticalTo(u), "should receive same unstructured object")
}

func TestTypedPredicate_ClientObject(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	// Note: Not registering corev1 - if conversion was attempted, it would fail

	filterCalled := false
	var receivedObj client.Object

	// Filter that expects client.Object interface - no conversion needed
	filter := func(obj client.Object) bool {
		filterCalled = true
		receivedObj = obj

		return true
	}

	factory := CreateTypedPredicate(filter)
	pred := factory(scheme, "test-controller")

	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Secret"))
	u.SetName("my-secret")

	e := event.TypedCreateEvent[client.Object]{Object: u}
	result := pred.Create(e)

	g.Expect(result).To(BeTrue())
	g.Expect(filterCalled).To(BeTrue())
	// Verify we received the SAME object (no conversion)
	g.Expect(receivedObj).To(BeIdenticalTo(u), "should receive same object without conversion")
}

func TestTypedPredicate_TypedAutoConversion(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	filterCalled := false
	var receivedName string
	var receivedPhase corev1.PodPhase

	// Filter that expects *corev1.Pod - should trigger automatic conversion
	filter := func(pod *corev1.Pod) bool {
		filterCalled = true
		receivedName = pod.GetName()
		receivedPhase = pod.Status.Phase

		return pod.Status.Phase == corev1.PodRunning
	}

	factory := CreateTypedPredicate(filter)
	pred := factory(scheme, "test-controller")

	// Create unstructured Pod (what we'd receive from a GVK-based watch)
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Pod"))
	u.SetName("my-pod")
	u.SetNamespace("default")
	u.Object["status"] = map[string]any{
		"phase": "Running",
	}

	e := event.TypedCreateEvent[client.Object]{Object: u}
	result := pred.Create(e)

	g.Expect(result).To(BeTrue(), "predicate should return true for Running pod")
	g.Expect(filterCalled).To(BeTrue())
	g.Expect(receivedName).To(Equal("my-pod"))
	g.Expect(receivedPhase).To(Equal(corev1.PodRunning))
}

func TestTypedPredicate_TypedFilterRejects(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	// Filter that rejects non-Running pods
	filter := func(pod *corev1.Pod) bool {
		return pod.Status.Phase == corev1.PodRunning
	}

	factory := CreateTypedPredicate(filter)
	pred := factory(scheme, "test-controller")

	// Create unstructured Pod with Pending phase
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Pod"))
	u.SetName("pending-pod")
	u.Object["status"] = map[string]any{
		"phase": "Pending",
	}

	e := event.TypedCreateEvent[client.Object]{Object: u}
	result := pred.Create(e)

	g.Expect(result).To(BeFalse(), "predicate should return false for Pending pod")
}

func TestTypedPredicate_AllEventTypes(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	filter := func(_ *corev1.ConfigMap) bool {
		return true
	}

	factory := CreateTypedPredicate(filter)
	pred := factory(scheme, "test-controller")

	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))
	u.SetName("test")

	// Test Create
	g.Expect(pred.Create(event.TypedCreateEvent[client.Object]{Object: u})).To(BeTrue())

	// Test Update
	g.Expect(pred.Update(event.TypedUpdateEvent[client.Object]{ObjectNew: u})).To(BeTrue())

	// Test Delete
	g.Expect(pred.Delete(event.TypedDeleteEvent[client.Object]{Object: u})).To(BeTrue())

	// Test Generic
	g.Expect(pred.Generic(event.TypedGenericEvent[client.Object]{Object: u})).To(BeTrue())
}

func TestTypedPredicate_ConversionFailure(t *testing.T) {
	g := NewWithT(t)

	// Empty scheme - ConfigMap not registered
	scheme := runtime.NewScheme()

	filterCalled := false

	filter := func(_ *corev1.ConfigMap) bool {
		filterCalled = true

		return true
	}

	factory := CreateTypedPredicate(filter)
	pred := factory(scheme, "test-controller")

	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))
	u.SetName("test")

	e := event.TypedCreateEvent[client.Object]{Object: u}
	result := pred.Create(e)

	g.Expect(result).To(BeFalse(), "should return false when conversion fails")
	g.Expect(filterCalled).To(BeFalse(), "filter should not be called when conversion fails")
}
