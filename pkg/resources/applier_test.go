package resources_test

import (
	"context"
	"testing"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/resources"

	. "github.com/onsi/gomega"
)

func newFakeClient(objs ...client.Object) client.Client {
	s := runtime.NewScheme()
	_ = corev1.AddToScheme(s)

	return fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(objs...).
		Build()
}

func newConfigMap(name string, namespace string, data map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: data,
	}
}

func TestDefaultApplier_Apply(t *testing.T) {
	t.Run("creates a new object", func(t *testing.T) {
		g := NewWithT(t)
		cli := newFakeClient()
		applier := resources.NewDefaultApplier(cli)

		cm := newConfigMap("test", "default", map[string]string{"key": "value"})

		err := applier.Apply(t.Context(), cm, client.FieldOwner("test"), client.ForceOwnership)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(cm.GetResourceVersion()).ToNot(BeEmpty())
	})

	t.Run("updates an existing object", func(t *testing.T) {
		g := NewWithT(t)

		existing := newConfigMap("test", "default", map[string]string{"key": "old"})
		cli := newFakeClient(existing)
		applier := resources.NewDefaultApplier(cli)

		updated := newConfigMap("test", "default", map[string]string{"key": "new"})

		err := applier.Apply(t.Context(), updated, client.FieldOwner("test"), client.ForceOwnership)
		g.Expect(err).ToNot(HaveOccurred())

		// Verify the object was updated
		var result corev1.ConfigMap
		err = cli.Get(t.Context(), client.ObjectKeyFromObject(updated), &result)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(result.Data).To(HaveKeyWithValue("key", "new"))
	})

	t.Run("exposes client", func(t *testing.T) {
		g := NewWithT(t)
		cli := newFakeClient()
		applier := resources.NewDefaultApplier(cli)

		g.Expect(applier.Client()).To(Equal(cli))
	})
}

func TestCachingApplier_Apply(t *testing.T) {
	t.Run("first apply delegates to inner and caches", func(t *testing.T) {
		g := NewWithT(t)
		cli := newFakeClient()

		inner := &countingApplier{inner: resources.NewDefaultApplier(cli)}
		applier := resources.NewCachingApplier(inner)

		cm := newConfigMap("test", "default", map[string]string{"key": "value"})

		err := applier.Apply(t.Context(), cm, client.FieldOwner("test"), client.ForceOwnership)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(inner.count).To(Equal(1))
		g.Expect(cm.GetResourceVersion()).ToNot(BeEmpty())
	})

	t.Run("second apply with unchanged rv skips inner", func(t *testing.T) {
		g := NewWithT(t)
		cli := newFakeClient()

		inner := &countingApplier{inner: resources.NewDefaultApplier(cli)}
		applier := resources.NewCachingApplier(inner)

		cm := newConfigMap("test", "default", map[string]string{"key": "value"})

		// First apply — should delegate
		err := applier.Apply(t.Context(), cm, client.FieldOwner("test"), client.ForceOwnership)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(inner.count).To(Equal(1))

		firstRV := cm.GetResourceVersion()
		firstUID := cm.GetUID()

		// Second apply — rv unchanged, should skip
		cm2 := newConfigMap("test", "default", map[string]string{"key": "value"})
		err = applier.Apply(t.Context(), cm2, client.FieldOwner("test"), client.ForceOwnership)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(inner.count).To(Equal(1)) // Still 1 — inner was not called

		// Input object should still have server state written back
		g.Expect(cm2.GetResourceVersion()).To(Equal(firstRV))
		g.Expect(cm2.GetUID()).To(Equal(firstUID))
	})

	t.Run("apply after external modification delegates to inner", func(t *testing.T) {
		g := NewWithT(t)
		cli := newFakeClient()

		inner := &countingApplier{inner: resources.NewDefaultApplier(cli)}
		applier := resources.NewCachingApplier(inner)

		cm := newConfigMap("test", "default", map[string]string{"key": "value"})

		// First apply
		err := applier.Apply(t.Context(), cm, client.FieldOwner("test"), client.ForceOwnership)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(inner.count).To(Equal(1))

		// Simulate external modification by directly updating the object
		var existing corev1.ConfigMap
		err = cli.Get(t.Context(), client.ObjectKeyFromObject(cm), &existing)
		g.Expect(err).ToNot(HaveOccurred())

		existing.Data["key"] = "modified-externally"
		err = cli.Update(t.Context(), &existing)
		g.Expect(err).ToNot(HaveOccurred())

		// Second apply — rv changed, should delegate
		cm2 := newConfigMap("test", "default", map[string]string{"key": "value"})
		err = applier.Apply(t.Context(), cm2, client.FieldOwner("test"), client.ForceOwnership)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(inner.count).To(Equal(2))
	})

	t.Run("apply to terminating object returns error", func(t *testing.T) {
		g := NewWithT(t)

		now := metav1.Now()
		existing := &corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:              "test",
				Namespace:         "default",
				DeletionTimestamp: &now,
				Finalizers:        []string{"test-finalizer"}, // Required for DeletionTimestamp
			},
		}

		cli := newFakeClient(existing)

		inner := &countingApplier{inner: resources.NewDefaultApplier(cli)}
		applier := resources.NewCachingApplier(inner)

		cm := newConfigMap("test", "default", map[string]string{"key": "value"})

		err := applier.Apply(t.Context(), cm, client.FieldOwner("test"), client.ForceOwnership)
		g.Expect(err).To(MatchError(ContainSubstring("object is being deleted")))
		g.Expect(inner.count).To(Equal(0)) // Inner was never called
	})

	t.Run("apply to nonexistent object delegates to inner", func(t *testing.T) {
		g := NewWithT(t)
		cli := newFakeClient()

		inner := &countingApplier{inner: resources.NewDefaultApplier(cli)}
		applier := resources.NewCachingApplier(inner)

		cm := newConfigMap("new-cm", "default", map[string]string{"key": "value"})

		err := applier.Apply(t.Context(), cm, client.FieldOwner("test"), client.ForceOwnership)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(inner.count).To(Equal(1))
	})

	t.Run("expired cache entry delegates to inner", func(t *testing.T) {
		g := NewWithT(t)
		cli := newFakeClient()

		inner := &countingApplier{inner: resources.NewDefaultApplier(cli)}
		applier := resources.NewCachingApplier(inner,
			resources.WithTTL(1*time.Millisecond),
		)

		cm := newConfigMap("test", "default", map[string]string{"key": "value"})

		// First apply
		err := applier.Apply(t.Context(), cm, client.FieldOwner("test"), client.ForceOwnership)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(inner.count).To(Equal(1))

		// Wait for cache to expire
		time.Sleep(5 * time.Millisecond)

		// Second apply — cache expired, should delegate
		cm2 := newConfigMap("test", "default", map[string]string{"key": "value"})
		err = applier.Apply(t.Context(), cm2, client.FieldOwner("test"), client.ForceOwnership)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(inner.count).To(Equal(2))
	})

	t.Run("exposes client from inner", func(t *testing.T) {
		g := NewWithT(t)
		cli := newFakeClient()

		inner := resources.NewDefaultApplier(cli)
		applier := resources.NewCachingApplier(inner)

		g.Expect(applier.Client()).To(Equal(cli))
	})
}

func TestCachingApplierOptions(t *testing.T) {
	t.Run("default options", func(t *testing.T) {
		g := NewWithT(t)

		inner := resources.NewDefaultApplier(newFakeClient())
		applier := resources.NewCachingApplier(inner)
		g.Expect(applier).ToNot(BeNil())
	})

	t.Run("custom options", func(t *testing.T) {
		g := NewWithT(t)

		inner := resources.NewDefaultApplier(newFakeClient())
		applier := resources.NewCachingApplier(inner,
			resources.WithMaxEntries(500),
			resources.WithTTL(10*time.Minute),
		)
		g.Expect(applier).ToNot(BeNil())
	})
}

// countingApplier wraps an Applier and counts how many times Apply was called.
type countingApplier struct {
	inner resources.Applier
	count int
}

func (a *countingApplier) Client() client.Client {
	return a.inner.Client()
}

func (a *countingApplier) Apply(
	ctx context.Context,
	obj client.Object,
	opts ...client.ApplyOption,
) error {
	a.count++

	return a.inner.Apply(ctx, obj, opts...)
}
