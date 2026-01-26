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
	h := factory(scheme, false, "test-controller")

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
	h := factory(scheme, true, "test-controller")

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
	h := factory(scheme, false, "test-controller")

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
	h := factory(scheme, false, "test-controller")
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
