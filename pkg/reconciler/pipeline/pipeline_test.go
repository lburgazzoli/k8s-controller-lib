//nolint:testpackage // Need access to private execute/cleanup methods
package pipeline

import (
	"context"
	"errors"
	"testing"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/reconciler"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	. "github.com/onsi/gomega"
)

func TestPipeline_EmptyPipeline(t *testing.T) {
	g := NewWithT(t)

	p, err := NewPipeline(
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

	p, err := NewPipeline(
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

	p, err := NewPipeline(
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

	p, err := NewPipeline(
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

	p, err := NewPipeline(
		WithCleanupActions(cleanup1, cleanup2, cleanup3),
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

	p, err := NewPipeline(
		WithCleanupActions(cleanup),
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

	p, err := NewPipeline(
		WithActions(action1, action2),
		WithFieldOwner("test-controller"),
	)
	g.Expect(err).ToNot(HaveOccurred())

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

	p, err := NewPipeline(
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

	p, err := NewPipeline(
		WithActions(action1, action2),
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

	p, err := NewPipeline(
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

	shouldRequeue, _ := resp.ShouldRequeue()
	g.Expect(shouldRequeue).To(BeTrue())
}

func TestPipeline_ContextPropagation(t *testing.T) {
	g := NewWithT(t)

	var receivedCtx context.Context

	action := func(ctx context.Context, _ *reconciler.Request, _ *reconciler.Response) error {
		receivedCtx = ctx //nolint:fatcontext // Test intentionally captures context for validation
		return nil
	}

	p, err := NewPipeline(
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
	p, err := NewPipeline(
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

	action1 := func(_ context.Context, _ *reconciler.Request, _ *reconciler.Response) error {
		executionOrder = append(executionOrder, "action1")
		return nil
	}

	action2 := func(_ context.Context, _ *reconciler.Request, _ *reconciler.Response) error {
		executionOrder = append(executionOrder, "action2")
		return nil
	}

	cleanup := func(_ context.Context, _ *reconciler.Request) error {
		executionOrder = append(executionOrder, "cleanup")
		return nil
	}

	// Build options programmatically using the struct directly
	opts := &PipelineOptions{
		Actions:        []reconciler.ActionFunc{action1, action2},
		CleanupActions: []reconciler.CleanupFunc{cleanup},
		FieldOwner:     "test-controller",
	}

	// PipelineOptions implements PipelineOption, so it can be passed directly
	p, err := NewPipeline(opts)
	g.Expect(err).ToNot(HaveOccurred())

	req := &reconciler.Request{
		Object: &TestResource{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"}},
	}
	resp := reconciler.NewResponse()

	err = p.execute(t.Context(), req, resp)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(executionOrder).To(Equal([]string{"action1", "action2"}))
}

func TestNewPipeline_RequiresFieldOwner(t *testing.T) {
	g := NewWithT(t)

	// Creating pipeline without field owner should return error
	p, err := NewPipeline(
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
	p, err := NewPipeline(
		WithFieldOwner("test-controller"),
		WithActions(func(_ context.Context, _ *reconciler.Request, _ *reconciler.Response) error {
			return nil
		}),
	)
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(p).ToNot(BeNil())
	g.Expect(p.fieldOwner).To(Equal("test-controller"))
}
