//nolint:testpackage // White-box testing: tests internal run() method and finalizer management
package pipeline

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/onsi/gomega/gstruct"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/reconciler"

	. "github.com/onsi/gomega"
)

// setupScheme registers TestResource with a new runtime scheme.
func setupScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	schemeBuilder := runtime.NewSchemeBuilder(func(s *runtime.Scheme) error {
		s.AddKnownTypes(schema.GroupVersion{Group: "test.example.com", Version: "v1"}, &TestResource{})

		return nil
	})
	_ = schemeBuilder.AddToScheme(scheme)

	return scheme
}

// newTestResource creates a TestResource with status.Accessor implementation.
func newTestResource(name string, namespace string) *TestResource {
	return &TestResource{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "test.example.com/v1",
			Kind:       "TestResource",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func TestReconcile_NoFinalizer_ExecutesActionsOnly(t *testing.T) {
	g := NewWithT(t)

	scheme := setupScheme()
	resource := newTestResource("test-resource", "default")

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(resource).
		WithStatusSubresource(resource).
		Build()

	var actionExecuted bool
	action := func(_ context.Context, _ *reconciler.Request, _ *reconciler.Response) error {
		actionExecuted = true

		return nil
	}

	p, err := NewPipeline(
		fakeClient,
		WithFieldOwner("test-controller"),
		WithActions(action),
	)
	g.Expect(err).ToNot(HaveOccurred())

	req := &reconciler.Request{
		Client: fakeClient,
		Object: resource,
	}

	resp, err := p.run(t.Context(), req)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(resp).ToNot(BeNil())
	g.Expect(actionExecuted).To(BeTrue())

	// No finalizer should be added
	updatedResource := &TestResource{}
	_ = fakeClient.Get(t.Context(), client.ObjectKeyFromObject(resource), updatedResource)
	g.Expect(updatedResource.Finalizers).To(BeEmpty())
}

func TestReconcile_AddsFinalizer_WhenCleanupActionsPresent(t *testing.T) {
	g := NewWithT(t)

	scheme := setupScheme()
	resource := newTestResource("test-resource", "default")

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(resource).
		WithStatusSubresource(resource).
		Build()

	cleanup := func(_ context.Context, _ *reconciler.Request) error {
		return nil
	}

	p, err := NewPipeline(
		fakeClient,
		WithFieldOwner("test-controller"),
		WithCleanupActions(cleanup),
	)
	g.Expect(err).ToNot(HaveOccurred())

	req := &reconciler.Request{
		Client: fakeClient,
		Object: resource,
	}

	resp, err := p.run(t.Context(), req)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(resp).ToNot(BeNil())

	// Finalizer should be added with default name
	updatedResource := &TestResource{}
	_ = fakeClient.Get(t.Context(), client.ObjectKeyFromObject(resource), updatedResource)
	g.Expect(updatedResource.Finalizers).To(ContainElement(DefaultFinalizer))
}

func TestReconcile_UsesCustomFinalizer(t *testing.T) {
	g := NewWithT(t)

	scheme := setupScheme()
	resource := newTestResource("test-resource", "default")

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(resource).WithStatusSubresource(resource).
		Build()

	cleanup := func(_ context.Context, _ *reconciler.Request) error {
		return nil
	}

	customFinalizer := "my-custom-finalizer"
	p, err := NewPipeline(
		fakeClient,
		WithFieldOwner("test-controller"),
		WithCleanupActions(cleanup),
		WithFinalizer(customFinalizer),
	)
	g.Expect(err).ToNot(HaveOccurred())

	req := &reconciler.Request{
		Client: fakeClient,
		Object: resource,
	}

	resp, err := p.run(t.Context(), req)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(resp).ToNot(BeNil())

	// Custom finalizer should be added
	updatedResource := &TestResource{}
	_ = fakeClient.Get(t.Context(), client.ObjectKeyFromObject(resource), updatedResource)
	g.Expect(updatedResource.Finalizers).To(ContainElement(customFinalizer))
}

func TestReconcile_ExecutesActions_AfterAddingFinalizer(t *testing.T) {
	g := NewWithT(t)

	scheme := setupScheme()
	resource := newTestResource("test-resource", "default")

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(resource).WithStatusSubresource(resource).
		Build()

	var actionExecuted bool
	action := func(_ context.Context, _ *reconciler.Request, _ *reconciler.Response) error {
		actionExecuted = true

		return nil
	}

	cleanup := func(_ context.Context, _ *reconciler.Request) error {
		return nil
	}

	p, err := NewPipeline(
		fakeClient,
		WithFieldOwner("test-controller"),
		WithActions(action),
		WithCleanupActions(cleanup),
	)
	g.Expect(err).ToNot(HaveOccurred())

	req := &reconciler.Request{
		Client: fakeClient,
		Object: resource,
	}

	resp, err := p.run(t.Context(), req)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(resp).ToNot(BeNil())
	g.Expect(actionExecuted).To(BeTrue())
}

func TestReconcile_SkipsFinalizerAddition_WhenAlreadyPresent(t *testing.T) {
	g := NewWithT(t)

	scheme := setupScheme()
	finalizer := DefaultFinalizer
	resource := newTestResource("test-resource", "default")
	resource.Finalizers = []string{finalizer}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(resource).WithStatusSubresource(resource).
		Build()

	var actionExecuted bool
	action := func(_ context.Context, _ *reconciler.Request, _ *reconciler.Response) error {
		actionExecuted = true

		return nil
	}

	cleanup := func(_ context.Context, _ *reconciler.Request) error {
		return nil
	}

	p, err := NewPipeline(
		fakeClient,
		WithFieldOwner("test-controller"),
		WithActions(action),
		WithCleanupActions(cleanup),
	)
	g.Expect(err).ToNot(HaveOccurred())

	req := &reconciler.Request{
		Client: fakeClient,
		Object: resource,
	}

	resp, err := p.run(t.Context(), req)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(resp).ToNot(BeNil())
	g.Expect(actionExecuted).To(BeTrue())

	// Finalizer count should remain 1
	updatedResource := &TestResource{}
	_ = fakeClient.Get(t.Context(), client.ObjectKeyFromObject(resource), updatedResource)
	g.Expect(updatedResource.Finalizers).To(HaveLen(1))
	g.Expect(updatedResource.Finalizers).To(ContainElement(finalizer))
}

func TestReconcile_Deletion_RunsCleanupOnly(t *testing.T) {
	g := NewWithT(t)

	scheme := setupScheme()
	now := metav1.Now()
	finalizer := DefaultFinalizer
	resource := newTestResource("test-resource", "default")
	resource.DeletionTimestamp = &now
	resource.Finalizers = []string{finalizer}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(resource).WithStatusSubresource(resource).
		Build()

	var actionExecuted bool
	var cleanupExecuted bool

	action := func(_ context.Context, _ *reconciler.Request, _ *reconciler.Response) error {
		actionExecuted = true

		return nil
	}

	cleanup := func(_ context.Context, _ *reconciler.Request) error {
		cleanupExecuted = true

		return nil
	}

	p, err := NewPipeline(
		fakeClient,
		WithFieldOwner("test-controller"),
		WithActions(action),
		WithCleanupActions(cleanup),
	)
	g.Expect(err).ToNot(HaveOccurred())

	req := &reconciler.Request{
		Client: fakeClient,
		Object: resource,
	}

	resp, err := p.run(t.Context(), req)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(resp).ToNot(BeNil())

	// Cleanup should run, action should NOT run during deletion
	g.Expect(cleanupExecuted).To(BeTrue())
	g.Expect(actionExecuted).To(BeFalse())
}

func TestReconcile_Deletion_RemovesFinalizer(t *testing.T) {
	g := NewWithT(t)

	scheme := setupScheme()
	now := metav1.Now()
	finalizer := DefaultFinalizer
	resource := newTestResource("test-resource", "default")
	resource.DeletionTimestamp = &now
	resource.Finalizers = []string{finalizer}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(resource).WithStatusSubresource(resource).
		Build()

	cleanup := func(_ context.Context, _ *reconciler.Request) error {
		return nil
	}

	p, err := NewPipeline(
		fakeClient,
		WithFieldOwner("test-controller"),
		WithCleanupActions(cleanup),
	)
	g.Expect(err).ToNot(HaveOccurred())

	req := &reconciler.Request{
		Client: fakeClient,
		Object: resource,
	}

	resp, err := p.run(t.Context(), req)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(resp).ToNot(BeNil())

	// Finalizer should be removed
	updatedResource := &TestResource{}
	_ = fakeClient.Get(t.Context(), client.ObjectKeyFromObject(resource), updatedResource)
	g.Expect(updatedResource.Finalizers).To(BeEmpty())
}

func TestReconcile_Deletion_NoFinalizerPresent_ReturnsEarly(t *testing.T) {
	g := NewWithT(t)

	scheme := setupScheme()
	resource := newTestResource("test-resource", "default")
	// No finalizers, no deletion timestamp initially

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(resource).WithStatusSubresource(resource).
		Build()

	var cleanupExecuted bool
	cleanup := func(_ context.Context, _ *reconciler.Request) error {
		cleanupExecuted = true

		return nil
	}

	p, err := NewPipeline(
		fakeClient,
		WithFieldOwner("test-controller"),
		WithCleanupActions(cleanup),
	)
	g.Expect(err).ToNot(HaveOccurred())

	// Manually set deletion timestamp after object is in fake client
	now := metav1.Now()
	resource.SetDeletionTimestamp(&now)

	req := &reconciler.Request{
		Client: fakeClient,
		Object: resource,
	}

	resp, err := p.run(t.Context(), req)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(resp).ToNot(BeNil())

	// Cleanup should NOT run if finalizer is not present
	g.Expect(cleanupExecuted).To(BeFalse())
}

func TestReconcile_Deletion_CleanupError_ReturnsError(t *testing.T) {
	g := NewWithT(t)

	scheme := setupScheme()
	now := metav1.Now()
	finalizer := DefaultFinalizer
	resource := newTestResource("test-resource", "default")
	resource.DeletionTimestamp = &now
	resource.Finalizers = []string{finalizer}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(resource).WithStatusSubresource(resource).
		Build()

	cleanupErr := errors.New("cleanup failed")
	cleanup := func(_ context.Context, _ *reconciler.Request) error {
		return cleanupErr
	}

	p, err := NewPipeline(
		fakeClient,
		WithFieldOwner("test-controller"),
		WithCleanupActions(cleanup),
	)
	g.Expect(err).ToNot(HaveOccurred())

	req := &reconciler.Request{
		Client: fakeClient,
		Object: resource,
	}

	resp, err := p.run(t.Context(), req)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("cleanup failed"))
	g.Expect(resp).ToNot(BeNil())

	// Finalizer should NOT be removed if cleanup fails
	updatedResource := &TestResource{}
	_ = fakeClient.Get(t.Context(), client.ObjectKeyFromObject(resource), updatedResource)
	g.Expect(updatedResource.Finalizers).To(ContainElement(finalizer))
}

func TestReconcile_ActionError_ReturnedToController(t *testing.T) {
	g := NewWithT(t)

	scheme := setupScheme()
	resource := newTestResource("test-resource", "default")

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(resource).WithStatusSubresource(resource).
		Build()

	actionErr := errors.New("action failed")
	action := func(_ context.Context, _ *reconciler.Request, _ *reconciler.Response) error {
		return actionErr
	}

	p, err := NewPipeline(
		fakeClient,
		WithFieldOwner("test-controller"),
		WithActions(action),
	)
	g.Expect(err).ToNot(HaveOccurred())

	req := &reconciler.Request{
		Client: fakeClient,
		Object: resource,
	}

	resp, err := p.run(t.Context(), req)
	g.Expect(err).To(HaveOccurred())
	g.Expect(errors.Is(err, actionErr)).To(BeTrue())
	g.Expect(resp).ToNot(BeNil())
}

func TestReconcile_ResponseAccumulation(t *testing.T) {
	g := NewWithT(t)

	scheme := setupScheme()
	resource := newTestResource("test-resource", "default")

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(resource).WithStatusSubresource(resource).
		Build()

	cm1 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "cm1", Namespace: resource.Namespace},
	}
	cm2 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "cm2", Namespace: resource.Namespace},
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
		fakeClient,
		WithFieldOwner("test-controller"),
		WithActions(action1, action2),
	)
	g.Expect(err).ToNot(HaveOccurred())

	req := &reconciler.Request{
		Client: fakeClient,
		Object: resource,
	}

	resp, err := p.run(t.Context(), req)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(resp).ToNot(BeNil())
	g.Expect(resp.GetObjects()).To(ConsistOf(cm1, cm2))
}

func TestReconcile_RequeueControl(t *testing.T) {
	g := NewWithT(t)

	scheme := setupScheme()
	resource := newTestResource("test-resource", "default")

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(resource).WithStatusSubresource(resource).
		Build()

	action := func(_ context.Context, _ *reconciler.Request, resp *reconciler.Response) error {
		resp.Requeue(1 * time.Second)

		return nil
	}

	p, err := NewPipeline(
		fakeClient,
		WithFieldOwner("test-controller"),
		WithActions(action),
	)
	g.Expect(err).ToNot(HaveOccurred())

	req := &reconciler.Request{
		Client: fakeClient,
		Object: resource,
	}

	resp, err := p.run(t.Context(), req)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(resp).ToNot(BeNil())

	requeue := resp.ShouldRequeue()
	g.Expect(requeue).ShouldNot(BeZero())
}

func TestReconcile_ContextPropagation(t *testing.T) {
	g := NewWithT(t)

	scheme := setupScheme()
	resource := newTestResource("test-resource", "default")

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(resource).WithStatusSubresource(resource).
		Build()

	var receivedCtx context.Context
	action := func(ctx context.Context, _ *reconciler.Request, _ *reconciler.Response) error {
		receivedCtx = ctx //nolint:fatcontext // Test intentionally captures context for validation

		return nil
	}

	p, err := NewPipeline(
		fakeClient,
		WithFieldOwner("test-controller"),
		WithActions(action),
	)
	g.Expect(err).ToNot(HaveOccurred())

	req := &reconciler.Request{
		Client: fakeClient,
		Object: resource,
	}

	testCtx := t.Context()
	resp, err := p.run(testCtx, req)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(resp).ToNot(BeNil())
	g.Expect(receivedCtx).To(Equal(testCtx))
}

func TestReconcile_StopError_HaltsExecution(t *testing.T) {
	g := NewWithT(t)

	scheme := setupScheme()
	resource := newTestResource("test-resource", "default")

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(resource).WithStatusSubresource(resource).
		Build()

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
		fakeClient,
		WithFieldOwner("test-controller"),
		WithActions(action1, action2, action3),
	)
	g.Expect(err).ToNot(HaveOccurred())

	req := &reconciler.Request{
		Client: fakeClient,
		Object: resource,
	}

	resp, err := p.run(t.Context(), req)
	g.Expect(err).To(HaveOccurred())
	g.Expect(IsStopError(err)).To(BeTrue())
	g.Expect(resp).ToNot(BeNil())

	// Only actions 1 and 2 should have executed
	g.Expect(executionOrder).To(Equal([]int{1, 2}))
}

func TestReconcile_MultipleCleanupActions_ExecuteInReverse(t *testing.T) {
	g := NewWithT(t)

	scheme := setupScheme()
	now := metav1.Now()
	finalizer := "reconciler.k8s-controller-lib/finalizer"
	resource := newTestResource("test-resource", "default")
	resource.DeletionTimestamp = &now
	resource.Finalizers = []string{finalizer}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(resource).WithStatusSubresource(resource).
		Build()

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
		fakeClient,
		WithFieldOwner("test-controller"),
		WithCleanupActions(cleanup1, cleanup2, cleanup3),
	)
	g.Expect(err).ToNot(HaveOccurred())

	req := &reconciler.Request{
		Client: fakeClient,
		Object: resource,
	}

	resp, err := p.run(t.Context(), req)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(resp).ToNot(BeNil())

	// Cleanup should execute in reverse order: 3, 2, 1
	g.Expect(cleanupOrder).To(Equal([]int{3, 2, 1}))
}

func TestReconcile_ObjectUpdate_PreservesOtherFields(t *testing.T) {
	g := NewWithT(t)

	scheme := setupScheme()
	resource := newTestResource("test-resource", "default")
	resource.Labels = map[string]string{
		"app": "test",
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(resource).WithStatusSubresource(resource).
		Build()

	cleanup := func(_ context.Context, _ *reconciler.Request) error {
		return nil
	}

	p, err := NewPipeline(
		fakeClient,
		WithFieldOwner("test-controller"),
		WithCleanupActions(cleanup),
	)
	g.Expect(err).ToNot(HaveOccurred())

	req := &reconciler.Request{
		Client: fakeClient,
		Object: resource,
	}

	resp, err := p.run(t.Context(), req)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(resp).ToNot(BeNil())

	// Labels should be preserved after finalizer addition
	updatedResource := &TestResource{}
	_ = fakeClient.Get(t.Context(), client.ObjectKeyFromObject(resource), updatedResource)
	g.Expect(updatedResource.Labels).To(gstruct.MatchAllKeys(gstruct.Keys{
		"app": Equal("test"),
	}))
}

func TestReconcile_ProvisionObjects_Success(t *testing.T) {
	g := NewWithT(t)

	scheme := setupScheme()
	resource := newTestResource("test-resource", "default")

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(resource).WithStatusSubresource(resource).
		Build()

	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "provisioned-cm",
			Namespace: "default",
		},
		Data: map[string]string{
			"key": "value",
		},
	}

	action := func(_ context.Context, _ *reconciler.Request, resp *reconciler.Response) error {
		resp.Objects(cm)

		return nil
	}

	p, err := NewPipeline(
		fakeClient,
		WithFieldOwner("test-controller"),
		WithActions(action),
	)
	g.Expect(err).ToNot(HaveOccurred())

	req := &reconciler.Request{
		Client: fakeClient,
		Object: resource,
	}

	resp, err := p.run(t.Context(), req)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(resp).ToNot(BeNil())

	// Verify the ConfigMap was provisioned to the cluster
	var fetchedCM corev1.ConfigMap
	err = fakeClient.Get(t.Context(), client.ObjectKey{Name: "provisioned-cm", Namespace: "default"}, &fetchedCM)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(fetchedCM.Data).To(HaveKeyWithValue("key", "value"))
}

func TestReconcile_ProvisionObjects_WithActionErrors(t *testing.T) {
	g := NewWithT(t)

	scheme := setupScheme()
	resource := newTestResource("test-resource", "default")

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(resource).WithStatusSubresource(resource).
		Build()

	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cm",
			Namespace: "default",
		},
	}

	actionErr := errors.New("action failed")
	action := func(_ context.Context, _ *reconciler.Request, resp *reconciler.Response) error {
		resp.Objects(cm)

		return actionErr
	}

	p, err := NewPipeline(
		fakeClient,
		WithFieldOwner("test-controller"),
		WithActions(action),
	)
	g.Expect(err).ToNot(HaveOccurred())

	req := &reconciler.Request{
		Client: fakeClient,
		Object: resource,
	}

	resp, err := p.run(t.Context(), req)
	g.Expect(err).To(HaveOccurred())
	g.Expect(errors.Is(err, actionErr)).To(BeTrue())
	g.Expect(resp).ToNot(BeNil())

	// Despite action error, object should still be provisioned
	var fetchedCM corev1.ConfigMap
	err = fakeClient.Get(t.Context(), client.ObjectKey{Name: "cm", Namespace: "default"}, &fetchedCM)
	g.Expect(err).ToNot(HaveOccurred())
}

func TestReconcile_ProvisionMultipleObjects(t *testing.T) {
	g := NewWithT(t)

	scheme := setupScheme()
	resource := newTestResource("test-resource", "default")

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(resource).WithStatusSubresource(resource).
		Build()

	cm1 := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cm1",
			Namespace: "default",
		},
		Data: map[string]string{"key": "value1"},
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
		Data: map[string]string{"key": "value2"},
	}

	action := func(_ context.Context, _ *reconciler.Request, resp *reconciler.Response) error {
		resp.Objects(cm1, cm2)

		return nil
	}

	p, err := NewPipeline(
		fakeClient,
		WithFieldOwner("test-controller"),
		WithActions(action),
	)
	g.Expect(err).ToNot(HaveOccurred())

	req := &reconciler.Request{
		Client: fakeClient,
		Object: resource,
	}

	resp, err := p.run(t.Context(), req)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(resp).ToNot(BeNil())

	// Verify both ConfigMaps were provisioned
	var fetchedCM1 corev1.ConfigMap
	err = fakeClient.Get(t.Context(), client.ObjectKey{Name: "cm1", Namespace: "default"}, &fetchedCM1)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(fetchedCM1.Data).To(HaveKeyWithValue("key", "value1"))

	var fetchedCM2 corev1.ConfigMap
	err = fakeClient.Get(t.Context(), client.ObjectKey{Name: "cm2", Namespace: "default"}, &fetchedCM2)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(fetchedCM2.Data).To(HaveKeyWithValue("key", "value2"))
}
