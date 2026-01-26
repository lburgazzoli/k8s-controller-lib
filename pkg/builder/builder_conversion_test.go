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

func TestConvertObject_UnstructuredToTyped_Success(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	// Create unstructured ConfigMap
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))
	u.SetName("test")
	u.SetNamespace("default")
	_ = unstructured.SetNestedMap(u.Object, map[string]any{
		"key": "value",
	}, "data")

	// Convert to typed
	result, err := convertObject(scheme, u)

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result).To(BeAssignableToTypeOf(&corev1.ConfigMap{}))

	cm := result.(*corev1.ConfigMap)
	g.Expect(cm.Name).To(Equal("test"))
	g.Expect(cm.Namespace).To(Equal("default"))
	g.Expect(cm.Data).To(HaveKey("key"))
}

func TestConvertObject_UnstructuredToTyped_Failure(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	// Don't register ConfigMap - will fail

	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))

	// Convert should fail
	_, err := convertObject(scheme, u)

	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("failed to create target type"))
}

func TestConvertObject_PartialMetadata(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()

	p := &metav1.PartialObjectMetadata{}
	p.SetName("test")

	// Partial should return unchanged
	result, err := convertObject(scheme, p)

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result).To(Equal(p))
}

func TestConvertObject_AlreadyTyped(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()

	cm := &corev1.ConfigMap{}
	cm.SetName("test")

	// Already typed should return unchanged
	result, err := convertObject(scheme, cm)

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
	wrapped := wrapPredicates(scheme, predicates, false)

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
	wrapped := wrapPredicates(scheme, predicates, true)

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

	wrapped := wrapPredicates(scheme, []predicate.Predicate{userPred}, true)

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

	wrapped := wrapPredicates(scheme, []predicate.Predicate{userPred}, true)

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
	wrapped := wrapHandler(scheme, h, false)

	g.Expect(wrapped).To(Equal(h), "should return original handler")
}

func TestWrapHandler_WithConversion(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	h := handler.TypedFuncs[client.Object, reconcile.Request]{}

	// With conversion
	wrapped := wrapHandler(scheme, h, true)

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

	wrapped := wrapHandler(scheme, userHandler, true)

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

	wrapped := wrapHandler(scheme, userHandler, true)

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

func TestUntypedMapperHandler_TypeAssertionSuccess(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	called := false
	mapper := func(_ context.Context, _ client.Object) []reconcile.Request {
		called = true

		return []reconcile.Request{
			{NamespacedName: client.ObjectKey{Name: "test", Namespace: "default"}},
		}
	}

	handler := createUntypedMapperHandler(scheme, mapper, false)

	cm := &corev1.ConfigMap{}
	cm.SetName("test")

	e := event.TypedCreateEvent[client.Object]{Object: cm}
	q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())

	handler.Create(context.Background(), e, q)

	g.Expect(called).To(BeTrue(), "mapper should be called")
	g.Expect(q.Len()).To(Equal(1), "request should be enqueued")
}

func TestUntypedMapperHandler_TypeAssertionFailure(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()

	// Wrong mapper signature
	wrongMapper := "not a function"

	handler := createUntypedMapperHandler(scheme, wrongMapper, false)

	cm := &corev1.ConfigMap{}
	e := event.TypedCreateEvent[client.Object]{Object: cm}
	q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())

	// Should not panic, just drop event
	handler.Create(context.Background(), e, q)

	g.Expect(q.Len()).To(Equal(0), "queue should be empty on type mismatch")
}

func TestUntypedMapperHandler_AllEventTypes(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	createCalled := false
	updateCalled := false
	deleteCalled := false
	genericCalled := false

	mapper := func(_ context.Context, obj client.Object) []reconcile.Request {
		switch obj.GetName() {
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

	handler := createUntypedMapperHandler(scheme, mapper, false)
	q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())

	// Test Create
	cm1 := &corev1.ConfigMap{}
	cm1.SetName("create")
	handler.Create(context.Background(), event.TypedCreateEvent[client.Object]{Object: cm1}, q)

	// Test Update
	cm2 := &corev1.ConfigMap{}
	cm2.SetName("update")
	handler.Update(context.Background(), event.TypedUpdateEvent[client.Object]{ObjectNew: cm2}, q)

	// Test Delete
	cm3 := &corev1.ConfigMap{}
	cm3.SetName("delete")
	handler.Delete(context.Background(), event.TypedDeleteEvent[client.Object]{Object: cm3}, q)

	// Test Generic
	cm4 := &corev1.ConfigMap{}
	cm4.SetName("generic")
	handler.Generic(context.Background(), event.TypedGenericEvent[client.Object]{Object: cm4}, q)

	g.Expect(createCalled).To(BeTrue())
	g.Expect(updateCalled).To(BeTrue())
	g.Expect(deleteCalled).To(BeTrue())
	g.Expect(genericCalled).To(BeTrue())
}
