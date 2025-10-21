//nolint:testpackage // Need access to private execute/cleanup methods
package pipeline

import (
	"context"
	"errors"
	"testing"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/reconciler"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	. "github.com/onsi/gomega"
)

// newMinimalFakeClient creates a basic fake client for tests that don't actually use it.
func newMinimalFakeClient() client.Client {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	return fake.NewClientBuilder().WithScheme(scheme).Build()
}

func TestPipeline_EmptyPipeline(t *testing.T) {
	g := NewWithT(t)

	p, err := NewPipeline(newMinimalFakeClient(),
		WithFieldOwner("test-controller"),
	)
	g.Expect(err).ToNot(HaveOccurred())

	req := &reconciler.Request{
		Object: &TestResource{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"}},
	}
	resp := reconciler.NewResponse()

	err = p.execute(t.Context(), req, resp)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(resp.GetObjects()).To(BeEmpty())
}

func TestPipeline_SequentialExecution(t *testing.T) {
	g := NewWithT(t)

	var executionOrder []int

	action1 := func(_ context.Context, _ *reconciler.Request, _ *reconciler.Response) error {
		executionOrder = append(executionOrder, 1)
		return nil
	}

	action2 := func(_ context.Context, _ *reconciler.Request, _ *reconciler.Response) error {
		executionOrder = append(executionOrder, 2)
		return nil
	}

	action3 := func(_ context.Context, _ *reconciler.Request, _ *reconciler.Response) error {
		executionOrder = append(executionOrder, 3)
		return nil
	}

	p, err := NewPipeline(newMinimalFakeClient(),
		WithActions(action1, action2, action3),
		WithFieldOwner("test-controller"),
	)
	g.Expect(err).ToNot(HaveOccurred())

	req := &reconciler.Request{
		Object: &TestResource{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"}},
	}
	resp := reconciler.NewResponse()

	err = p.execute(t.Context(), req, resp)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(executionOrder).To(Equal([]int{1, 2, 3}))
}

func TestPipeline_ErrorAccumulation(t *testing.T) {
	g := NewWithT(t)

	err1 := errors.New("error 1")
	err2 := errors.New("error 2")
	err3 := errors.New("error 3")

	action1 := func(_ context.Context, _ *reconciler.Request, _ *reconciler.Response) error {
		return err1
	}

	action2 := func(_ context.Context, _ *reconciler.Request, _ *reconciler.Response) error {
		return err2
	}

	action3 := func(_ context.Context, _ *reconciler.Request, _ *reconciler.Response) error {
		return err3
	}

	p, err := NewPipeline(newMinimalFakeClient(),
		WithActions(action1, action2, action3),
		WithFieldOwner("test-controller"),
	)
	g.Expect(err).ToNot(HaveOccurred())

	req := &reconciler.Request{
		Object: &TestResource{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"}},
	}
	resp := reconciler.NewResponse()

	err = p.execute(t.Context(), req, resp)
	g.Expect(err).To(HaveOccurred())

	// Verify it's an aggregate error containing all three errors
	agg := utilerrors.Flatten(utilerrors.NewAggregate([]error{err}))
	g.Expect(agg.Errors()).To(HaveLen(3))
	g.Expect(agg.Errors()).To(ConsistOf(err1, err2, err3))
}

func TestPipeline_StopError(t *testing.T) {
	g := NewWithT(t)

	var executionOrder []int

	action1 := func(_ context.Context, _ *reconciler.Request, _ *reconciler.Response) error {
		executionOrder = append(executionOrder, 1)
		return nil
	}

	action2 := func(_ context.Context, _ *reconciler.Request, _ *reconciler.Response) error {
		executionOrder = append(executionOrder, 2)
		return Stop(errors.New("stop here"))
	}

	action3 := func(_ context.Context, _ *reconciler.Request, _ *reconciler.Response) error {
		executionOrder = append(executionOrder, 3)
		return nil
	}

	p, err := NewPipeline(newMinimalFakeClient(),
		WithActions(action1, action2, action3),
		WithFieldOwner("test-controller"),
	)
	g.Expect(err).ToNot(HaveOccurred())

	req := &reconciler.Request{
		Object: &TestResource{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"}},
	}
	resp := reconciler.NewResponse()

	err = p.execute(t.Context(), req, resp)
	g.Expect(err).To(HaveOccurred())
	g.Expect(IsStopError(err)).To(BeTrue())

	// Only actions 1 and 2 should have executed
	g.Expect(executionOrder).To(Equal([]int{1, 2}))
}

func TestPipeline_CleanupReverseOrder(t *testing.T) {
	g := NewWithT(t)

	var cleanupOrder []int

	cleanup1 := func(_ context.Context, _ *reconciler.Request) error {
		cleanupOrder = append(cleanupOrder, 1)
		return nil
	}

	cleanup2 := func(_ context.Context, _ *reconciler.Request) error {
		cleanupOrder = append(cleanupOrder, 2)
		return nil
	}

	cleanup3 := func(_ context.Context, _ *reconciler.Request) error {
		cleanupOrder = append(cleanupOrder, 3)
		return nil
	}

	resource := &TestResource{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "test.example.com/v1",
			Kind:       "TestResource",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-resource",
			Namespace:  "default",
			Finalizers: []string{defaultFinalizer},
		},
	}

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	schemeBuilder := runtime.NewSchemeBuilder(func(s *runtime.Scheme) error {
		s.AddKnownTypes(schema.GroupVersion{Group: "test.example.com", Version: "v1"}, &TestResource{})
		return nil
	})
	_ = schemeBuilder.AddToScheme(scheme)
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(resource).WithStatusSubresource(resource).
		Build()

	p, err := NewPipeline(fakeClient,
		WithCleanupActions(cleanup1, cleanup2, cleanup3),
		WithFieldOwner("test-controller"),
	)
	g.Expect(err).ToNot(HaveOccurred())

	req := &reconciler.Request{
		Client: fakeClient,
		Object: resource,
	}

	err = p.cleanup(t.Context(), req)
	g.Expect(err).ToNot(HaveOccurred())

	// Cleanup should execute in reverse order: 3, 2, 1
	g.Expect(cleanupOrder).To(Equal([]int{3, 2, 1}))
}

func TestPipeline_CleanupIndependent(t *testing.T) {
	g := NewWithT(t)

	var cleanupExecuted bool

	cleanup := func(_ context.Context, _ *reconciler.Request) error {
		cleanupExecuted = true
		return nil
	}

	resource := &TestResource{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "test.example.com/v1",
			Kind:       "TestResource",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-resource",
			Namespace:  "default",
			Finalizers: []string{defaultFinalizer},
		},
	}

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	schemeBuilder := runtime.NewSchemeBuilder(func(s *runtime.Scheme) error {
		s.AddKnownTypes(schema.GroupVersion{Group: "test.example.com", Version: "v1"}, &TestResource{})
		return nil
	})
	_ = schemeBuilder.AddToScheme(scheme)
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(resource).WithStatusSubresource(resource).
		Build()

	p, err := NewPipeline(fakeClient,
		WithCleanupActions(cleanup),
		WithFieldOwner("test-controller"),
	)
	g.Expect(err).ToNot(HaveOccurred())

	req := &reconciler.Request{
		Client: fakeClient,
		Object: resource,
	}

	// Cleanup is independent of Execute
	err = p.cleanup(t.Context(), req)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(cleanupExecuted).To(BeTrue())
}

func TestPipeline_ResponseAccumulation(t *testing.T) {
	g := NewWithT(t)

	cm1 := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cm1",
			Namespace: "default",
		},
	}
	cm2 := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cm2",
			Namespace: "default",
		},
	}

	action1 := func(_ context.Context, _ *reconciler.Request, resp *reconciler.Response) error {
		resp.Objects(cm1)
		return nil
	}

	action2 := func(_ context.Context, _ *reconciler.Request, resp *reconciler.Response) error {
		resp.Objects(cm2)
		return nil
	}

	resource := &TestResource{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "test.example.com/v1",
			Kind:       "TestResource",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-resource",
			Namespace: "default",
		},
	}

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	schemeBuilder := runtime.NewSchemeBuilder(func(s *runtime.Scheme) error {
		s.AddKnownTypes(schema.GroupVersion{Group: "test.example.com", Version: "v1"}, &TestResource{})
		return nil
	})
	_ = schemeBuilder.AddToScheme(scheme)
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(resource).WithStatusSubresource(resource).
		Build()

	p, err := NewPipeline(fakeClient,
		WithActions(action1, action2),
		WithFieldOwner("test-controller"),
	)
	g.Expect(err).ToNot(HaveOccurred())

	req := &reconciler.Request{
		Client: fakeClient,
		Object: resource,
	}
	resp := reconciler.NewResponse()

	err = p.execute(t.Context(), req, resp)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(resp.GetObjects()).To(ConsistOf(cm1, cm2))
}

func TestPipeline_CleanupErrorAccumulation(t *testing.T) {
	g := NewWithT(t)

	cleanupErr1 := errors.New("cleanup error 1")
	cleanupErr2 := errors.New("cleanup error 2")

	cleanup1 := func(_ context.Context, _ *reconciler.Request) error {
		return cleanupErr1
	}

	cleanup2 := func(_ context.Context, _ *reconciler.Request) error {
		return cleanupErr2
	}

	p, err := NewPipeline(newMinimalFakeClient(),
		WithCleanupActions(cleanup1, cleanup2),
		WithFieldOwner("test-controller"),
	)
	g.Expect(err).ToNot(HaveOccurred())

	resource := &TestResource{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "test.example.com/v1",
			Kind:       "TestResource",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-resource",
			Namespace:  "default",
			Finalizers: []string{defaultFinalizer},
		},
	}

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	schemeBuilder := runtime.NewSchemeBuilder(func(s *runtime.Scheme) error {
		s.AddKnownTypes(schema.GroupVersion{Group: "test.example.com", Version: "v1"}, &TestResource{})
		return nil
	})
	_ = schemeBuilder.AddToScheme(scheme)
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(resource).WithStatusSubresource(resource).
		Build()

	req := &reconciler.Request{
		Client: fakeClient,
		Object: resource,
	}

	err = p.cleanup(t.Context(), req)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("cleanup failed"))
	g.Expect(errors.Is(err, cleanupErr1)).To(BeTrue())
	g.Expect(errors.Is(err, cleanupErr2)).To(BeTrue())
}

func TestPipeline_ExecuteDoesNotRunCleanup(t *testing.T) {
	g := NewWithT(t)

	var executionOrder []string

	action1 := func(_ context.Context, _ *reconciler.Request, _ *reconciler.Response) error {
		executionOrder = append(executionOrder, "action1")
		return nil
	}

	action2 := func(_ context.Context, _ *reconciler.Request, _ *reconciler.Response) error {
		executionOrder = append(executionOrder, "action2")
		return nil
	}

	cleanup1 := func(_ context.Context, _ *reconciler.Request) error {
		executionOrder = append(executionOrder, "cleanup1")
		return nil
	}

	cleanup2 := func(_ context.Context, _ *reconciler.Request) error {
		executionOrder = append(executionOrder, "cleanup2")
		return nil
	}

	resource := &TestResource{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "test.example.com/v1",
			Kind:       "TestResource",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-resource",
			Namespace:  "default",
			Finalizers: []string{defaultFinalizer},
		},
	}

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	schemeBuilder := runtime.NewSchemeBuilder(func(s *runtime.Scheme) error {
		s.AddKnownTypes(schema.GroupVersion{Group: "test.example.com", Version: "v1"}, &TestResource{})
		return nil
	})
	_ = schemeBuilder.AddToScheme(scheme)
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(resource).WithStatusSubresource(resource).
		Build()

	p, err := NewPipeline(fakeClient,
		WithActions(action1, action2),
		WithCleanupActions(cleanup1, cleanup2),
		WithFieldOwner("test-controller"),
	)
	g.Expect(err).ToNot(HaveOccurred())

	req := &reconciler.Request{
		Client: fakeClient,
		Object: resource,
	}
	resp := reconciler.NewResponse()

	// Execute should only run actions, not cleanup
	err = p.execute(t.Context(), req, resp)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(executionOrder).To(Equal([]string{"action1", "action2"}))

	// Cleanup runs separately
	err = p.cleanup(t.Context(), req)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(executionOrder).To(Equal([]string{"action1", "action2", "cleanup2", "cleanup1"}))
}

func TestStopError_Unwrap(t *testing.T) {
	g := NewWithT(t)

	cause := errors.New("underlying cause")
	stopErr := Stop(cause)

	g.Expect(errors.Unwrap(stopErr)).To(Equal(cause))
	g.Expect(errors.Is(stopErr, cause)).To(BeTrue())
}

func TestStopError_ErrorMessage(t *testing.T) {
	g := NewWithT(t)

	cause := errors.New("test error")
	stopErr := Stop(cause)

	g.Expect(stopErr.Error()).To(Equal("pipeline stopped: test error"))
}

func TestPipeline_RequeueControl(t *testing.T) {
	g := NewWithT(t)

	action := func(_ context.Context, _ *reconciler.Request, resp *reconciler.Response) error {
		resp.Requeue()
		return nil
	}

	p, err := NewPipeline(newMinimalFakeClient(),
		WithActions(action),
		WithFieldOwner("test-controller"),
	)
	g.Expect(err).ToNot(HaveOccurred())

	req := &reconciler.Request{
		Object: &TestResource{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"}},
	}
	resp := reconciler.NewResponse()

	err = p.execute(t.Context(), req, resp)
	g.Expect(err).ToNot(HaveOccurred())

	requeue := resp.ShouldRequeue()
	g.Expect(requeue).ShouldNot(BeZero())
}

func TestPipeline_ContextPropagation(t *testing.T) {
	g := NewWithT(t)

	var receivedCtx context.Context

	action := func(ctx context.Context, _ *reconciler.Request, _ *reconciler.Response) error {
		receivedCtx = ctx //nolint:fatcontext // Test intentionally captures context for validation
		return nil
	}

	p, err := NewPipeline(newMinimalFakeClient(),
		WithActions(action),
		WithFieldOwner("test-controller"),
	)
	g.Expect(err).ToNot(HaveOccurred())

	req := &reconciler.Request{
		Object: &TestResource{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"}},
	}
	resp := reconciler.NewResponse()

	testCtx := t.Context()
	err = p.execute(testCtx, req, resp)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(receivedCtx).To(Equal(testCtx))
}

func TestPipeline_MultipleOptions(t *testing.T) {
	g := NewWithT(t)

	action1 := func(_ context.Context, _ *reconciler.Request, _ *reconciler.Response) error {
		return nil
	}

	action2 := func(_ context.Context, _ *reconciler.Request, _ *reconciler.Response) error {
		return nil
	}

	cleanup := func(_ context.Context, _ *reconciler.Request) error {
		return nil
	}

	// Test that multiple WithActions calls accumulate
	p, err := NewPipeline(newMinimalFakeClient(),
		WithActions(action1),
		WithActions(action2),
		WithCleanupActions(cleanup),
		WithFieldOwner("test-controller"),
	)
	g.Expect(err).ToNot(HaveOccurred())

	req := &reconciler.Request{
		Object: &TestResource{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"}},
	}
	resp := reconciler.NewResponse()

	err = p.execute(t.Context(), req, resp)
	g.Expect(err).ToNot(HaveOccurred())
}

func TestPipeline_UsingOptionsStruct(t *testing.T) {
	g := NewWithT(t)

	var executionOrder []string

	// Non-typed actions
	action1 := func(_ context.Context, _ *reconciler.Request, _ *reconciler.Response) error {
		executionOrder = append(executionOrder, "action1")
		return nil
	}

	action2 := func(_ context.Context, _ *reconciler.Request, _ *reconciler.Response) error {
		executionOrder = append(executionOrder, "action2")
		return nil
	}

	// Typed action
	typedAction := reconciler.TypedActionFunc[*TestResource](
		func(_ context.Context, req *reconciler.TypedRequest[*TestResource], _ *reconciler.Response) error {
			executionOrder = append(executionOrder, "typed-action")
			return nil
		},
	)

	// Non-typed cleanup
	cleanup := func(_ context.Context, _ *reconciler.Request) error {
		executionOrder = append(executionOrder, "cleanup")
		return nil
	}

	// Typed cleanup
	typedCleanup := reconciler.TypedCleanupFunc[*TestResource](
		func(_ context.Context, req *reconciler.TypedRequest[*TestResource]) error {
			executionOrder = append(executionOrder, "typed-cleanup")
			return nil
		},
	)

	// Build options programmatically using the struct directly
	// Mix typed and non-typed actions by manually converting typed ones
	opts := &Options{
		Actions: []reconciler.ActionFunc{
			action1,
			action2,
			reconciler.ToActionFunc(typedAction), // Manual conversion
		},
		CleanupActions: []reconciler.CleanupFunc{
			cleanup,
			reconciler.ToCleanupFunc(typedCleanup), // Manual conversion
		},
		FieldOwner: "test-controller",
	}

	// Options implements Option, so it can be passed directly
	p, err := NewPipeline(newMinimalFakeClient(), opts)
	g.Expect(err).ToNot(HaveOccurred())

	req := &reconciler.Request{
		Object: &TestResource{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"}},
	}
	resp := reconciler.NewResponse()

	err = p.execute(t.Context(), req, resp)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(executionOrder).To(Equal([]string{"action1", "action2", "typed-action"}))
}

func TestNewPipeline_RequiresFieldOwner(t *testing.T) {
	g := NewWithT(t)

	// Creating pipeline without field owner should return error
	p, err := NewPipeline(newMinimalFakeClient(),
		WithActions(func(_ context.Context, _ *reconciler.Request, _ *reconciler.Response) error {
			return nil
		}),
	)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(Equal("field owner is required"))
	g.Expect(p).To(BeNil())
}

func TestNewPipeline_WithFieldOwner(t *testing.T) {
	g := NewWithT(t)

	// Creating pipeline with field owner should succeed
	p, err := NewPipeline(newMinimalFakeClient(),
		WithFieldOwner("test-controller"),
		WithActions(func(_ context.Context, _ *reconciler.Request, _ *reconciler.Response) error {
			return nil
		}),
	)
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(p).ToNot(BeNil())
	g.Expect(p.opts.FieldOwner).To(Equal("test-controller"))
}

func TestWithTypedActions_TypeSafeAccess(t *testing.T) {
	g := NewWithT(t)

	var capturedField string
	var executionCount int

	// Create a type-safe action that accesses TestResource-specific fields
	typedAction := reconciler.TypedActionFunc[*TestResource](
		func(ctx context.Context, req *reconciler.TypedRequest[*TestResource], resp *reconciler.Response) error {
			// Type-safe access - no type assertion needed
			capturedField = req.Object.Spec.Field
			executionCount++
			return nil
		},
	)

	// Convert to non-generic ActionFunc using WithTypedActions
	actions := WithTypedActions(typedAction)

	// Verify it returns Actions type
	g.Expect(actions.actions).To(HaveLen(1))

	// Create test resource
	resource := &TestResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Spec: TestResourceSpec{
			Field: "test-value",
		},
	}

	// Execute the converted action
	req := &reconciler.Request{
		Object: resource,
	}
	resp := reconciler.NewResponse()

	err := actions.actions[0](t.Context(), req, resp)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(capturedField).To(Equal("test-value"))
	g.Expect(executionCount).To(Equal(1))
}

func TestWithTypedActions_MultipleActions(t *testing.T) {
	g := NewWithT(t)

	var executionOrder []int

	typedAction1 := reconciler.TypedActionFunc[*TestResource](
		func(ctx context.Context, req *reconciler.TypedRequest[*TestResource], resp *reconciler.Response) error {
			executionOrder = append(executionOrder, 1)
			return nil
		},
	)

	typedAction2 := reconciler.TypedActionFunc[*TestResource](
		func(ctx context.Context, req *reconciler.TypedRequest[*TestResource], resp *reconciler.Response) error {
			executionOrder = append(executionOrder, 2)
			return nil
		},
	)

	typedAction3 := reconciler.TypedActionFunc[*TestResource](
		func(ctx context.Context, req *reconciler.TypedRequest[*TestResource], resp *reconciler.Response) error {
			executionOrder = append(executionOrder, 3)
			return nil
		},
	)

	// Convert all actions at once
	actions := WithTypedActions(typedAction1, typedAction2, typedAction3)

	g.Expect(actions.actions).To(HaveLen(3))

	// Execute all actions
	resource := &TestResource{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
	}
	req := &reconciler.Request{Object: resource}
	resp := reconciler.NewResponse()

	for _, action := range actions.actions {
		err := action(t.Context(), req, resp)
		g.Expect(err).ToNot(HaveOccurred())
	}

	g.Expect(executionOrder).To(Equal([]int{1, 2, 3}))
}

func TestWithTypedActions_TypeMismatchError(t *testing.T) {
	g := NewWithT(t)

	// Create an action that expects TestResource
	typedAction := reconciler.TypedActionFunc[*TestResource](
		func(ctx context.Context, req *reconciler.TypedRequest[*TestResource], resp *reconciler.Response) error {
			return nil
		},
	)

	actions := WithTypedActions(typedAction)

	// Create a minimal type that implements ManagedObject but isn't TestResource
	// This tests runtime type safety when the wrong type is passed
	type DifferentResource struct {
		TestResource
	}

	differentResource := &DifferentResource{}
	req := &reconciler.Request{Object: differentResource}
	resp := reconciler.NewResponse()

	err := actions.actions[0](t.Context(), req, resp)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("type assertion failed"))
	g.Expect(err.Error()).To(ContainSubstring("*pipeline.TestResource"))
	g.Expect(err.Error()).To(ContainSubstring("*pipeline.DifferentResource"))
}

func TestWithTypedCleanup_TypeSafeAccess(t *testing.T) {
	g := NewWithT(t)

	var capturedName string
	var cleanupExecuted bool

	// Create a type-safe cleanup action
	typedCleanup := reconciler.TypedCleanupFunc[*TestResource](
		func(ctx context.Context, req *reconciler.TypedRequest[*TestResource]) error {
			capturedName = req.Object.GetName()
			cleanupExecuted = true
			return nil
		},
	)

	// Convert to non-generic CleanupFunc
	cleanups := WithTypedCleanup(typedCleanup)

	g.Expect(cleanups.actions).To(HaveLen(1))

	// Execute the cleanup
	resource := &TestResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cleanup-test",
			Namespace: "default",
		},
	}

	req := &reconciler.Request{Object: resource}

	err := cleanups.actions[0](t.Context(), req)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(cleanupExecuted).To(BeTrue())
	g.Expect(capturedName).To(Equal("cleanup-test"))
}

func TestWithTypedCleanup_MultipleCleanups(t *testing.T) {
	g := NewWithT(t)

	var cleanupOrder []int

	cleanup1 := reconciler.TypedCleanupFunc[*TestResource](
		func(ctx context.Context, req *reconciler.TypedRequest[*TestResource]) error {
			cleanupOrder = append(cleanupOrder, 1)
			return nil
		},
	)

	cleanup2 := reconciler.TypedCleanupFunc[*TestResource](
		func(ctx context.Context, req *reconciler.TypedRequest[*TestResource]) error {
			cleanupOrder = append(cleanupOrder, 2)
			return nil
		},
	)

	cleanup3 := reconciler.TypedCleanupFunc[*TestResource](
		func(ctx context.Context, req *reconciler.TypedRequest[*TestResource]) error {
			cleanupOrder = append(cleanupOrder, 3)
			return nil
		},
	)

	cleanups := WithTypedCleanup(cleanup1, cleanup2, cleanup3)

	g.Expect(cleanups.actions).To(HaveLen(3))

	resource := &TestResource{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
	}
	req := &reconciler.Request{Object: resource}

	// Execute all cleanups (in a real pipeline, these would execute in reverse)
	for _, cleanup := range cleanups.actions {
		err := cleanup(t.Context(), req)
		g.Expect(err).ToNot(HaveOccurred())
	}

	g.Expect(cleanupOrder).To(Equal([]int{1, 2, 3}))
}

func TestWithTypedCleanup_TypeMismatchError(t *testing.T) {
	g := NewWithT(t)

	typedCleanup := reconciler.TypedCleanupFunc[*TestResource](
		func(ctx context.Context, req *reconciler.TypedRequest[*TestResource]) error {
			return nil
		},
	)

	cleanups := WithTypedCleanup(typedCleanup)

	// Different type that implements ManagedObject
	type DifferentResource struct {
		TestResource
	}

	differentResource := &DifferentResource{}
	req := &reconciler.Request{Object: differentResource}

	err := cleanups.actions[0](t.Context(), req)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("type assertion failed"))
}

func TestTypedActionsIntegration_WithPipeline(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	schemeBuilder := runtime.NewSchemeBuilder(func(s *runtime.Scheme) error {
		s.AddKnownTypes(schema.GroupVersion{Group: "test.example.com", Version: "v1"}, &TestResource{})
		return nil
	})
	_ = schemeBuilder.AddToScheme(scheme)

	resource := &TestResource{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "test.example.com/v1",
			Kind:       "TestResource",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "integration-test",
			Namespace: "default",
		},
		Spec: TestResourceSpec{
			Field: "original-value",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(resource).
		WithStatusSubresource(resource).
		Build()

	var executionOrder []string

	// Mix typed and non-typed actions in the same pipeline
	nonTypedAction := reconciler.ActionFunc(
		func(ctx context.Context, req *reconciler.Request, resp *reconciler.Response) error {
			executionOrder = append(executionOrder, "non-typed")
			return nil
		},
	)

	typedAction := reconciler.TypedActionFunc[*TestResource](
		func(ctx context.Context, req *reconciler.TypedRequest[*TestResource], resp *reconciler.Response) error {
			executionOrder = append(executionOrder, "typed")
			// Type-safe access to Spec.Field
			g.Expect(req.Object.Spec.Field).To(Equal("original-value"))
			return nil
		},
	)

	typedCleanup := reconciler.TypedCleanupFunc[*TestResource](
		func(ctx context.Context, req *reconciler.TypedRequest[*TestResource]) error {
			executionOrder = append(executionOrder, "typed-cleanup")
			return nil
		},
	)

	// Create pipeline with mixed actions
	p, err := NewPipeline(
		fakeClient,
		WithFieldOwner("test-controller"),
		WithActions(nonTypedAction),   // Non-generic
		WithTypedActions(typedAction), // Generic with explicit type
		WithTypedCleanup(typedCleanup),
	)
	g.Expect(err).ToNot(HaveOccurred())

	req := &reconciler.Request{
		Client: fakeClient,
		Object: resource,
	}

	// Execute actions
	resp, err := p.run(t.Context(), req)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(resp).ToNot(BeNil())

	// Verify both types of actions executed
	g.Expect(executionOrder).To(ContainElement("non-typed"))
	g.Expect(executionOrder).To(ContainElement("typed"))
}
