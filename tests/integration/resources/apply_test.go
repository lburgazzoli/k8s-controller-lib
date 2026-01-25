package resources_test

import (
	"context"
	"testing"

	"github.com/lburgazzoli/gomega-matchers/pkg/matchers/jq"
	"github.com/lburgazzoli/k3s-envtest/pkg/k3senv"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/resources"

	. "github.com/onsi/gomega"
)

func TestApplyWithK3sEnv(t *testing.T) {
	ctx := context.Background()

	s := runtime.NewScheme()
	if err := corev1.AddToScheme(s); err != nil {
		t.Fatalf("failed to add corev1 to scheme: %v", err)
	}
	if err := appsv1.AddToScheme(s); err != nil {
		t.Fatalf("failed to add appsv1 to scheme: %v", err)
	}

	env, err := k3senv.New(
		k3senv.WithScheme(s),
	)
	if err != nil {
		t.Fatalf("failed to create k3s env: %v", err)
	}

	err = env.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start k3s env: %v", err)
	}
	t.Cleanup(func() {
		_ = env.Stop(ctx)
	})

	cli := env.Client()

	t.Run("create ConfigMap with Apply", func(t *testing.T) {
		g := NewWithT(t)

		cm := &corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-apply-cm",
				Namespace: "default",
			},
			Data: map[string]string{
				"key1": "value1",
			},
		}

		err := resources.Apply(ctx, cli, cm, client.FieldOwner("test-controller"))
		g.Expect(err).ToNot(HaveOccurred())

		// Verify the ConfigMap was created
		var fetched corev1.ConfigMap
		err = cli.Get(ctx, client.ObjectKey{Name: "test-apply-cm", Namespace: "default"}, &fetched)
		g.Expect(err).ToNot(HaveOccurred())

		fetched.SetGroupVersionKind(cm.GroupVersionKind())
		u, err := resources.ToUnstructured(cli.Scheme(), &fetched)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(u.Object).To(jq.Match(`.data.key1 == "value1"`))

		// Cleanup
		err = cli.Delete(ctx, cm)
		g.Expect(err).ToNot(HaveOccurred())
	})

	t.Run("update ConfigMap with Apply", func(t *testing.T) {
		g := NewWithT(t)

		cm := &corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-apply-update-cm",
				Namespace: "default",
			},
			Data: map[string]string{
				"key1": "value1",
			},
		}

		// Create initial version
		err := resources.Apply(ctx, cli, cm, client.FieldOwner("test-controller"))
		g.Expect(err).ToNot(HaveOccurred())

		// Update the ConfigMap
		cm.Data["key2"] = "value2"
		err = resources.Apply(ctx, cli, cm, client.FieldOwner("test-controller"))
		g.Expect(err).ToNot(HaveOccurred())

		// Verify the update
		var fetched corev1.ConfigMap
		err = cli.Get(ctx, client.ObjectKey{Name: "test-apply-update-cm", Namespace: "default"}, &fetched)
		g.Expect(err).ToNot(HaveOccurred())

		fetched.SetGroupVersionKind(cm.GroupVersionKind())
		u, err := resources.ToUnstructured(cli.Scheme(), &fetched)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(u.Object).To(And(
			jq.Match(`.data.key1 == "value1"`),
			jq.Match(`.data.key2 == "value2"`),
		))

		// Cleanup
		err = cli.Delete(ctx, cm)
		g.Expect(err).ToNot(HaveOccurred())
	})

	t.Run("Apply with labels and annotations", func(t *testing.T) {
		g := NewWithT(t)

		cm := &corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-apply-labels-cm",
				Namespace: "default",
				Labels: map[string]string{
					"app":     "test",
					"version": "v1",
				},
				Annotations: map[string]string{
					"description": "test config",
				},
			},
			Data: map[string]string{
				"key": "value",
			},
		}

		err := resources.Apply(ctx, cli, cm, client.FieldOwner("test-controller"))
		g.Expect(err).ToNot(HaveOccurred())

		var fetched corev1.ConfigMap
		err = cli.Get(ctx, client.ObjectKey{Name: "test-apply-labels-cm", Namespace: "default"}, &fetched)
		g.Expect(err).ToNot(HaveOccurred())

		fetched.SetGroupVersionKind(cm.GroupVersionKind())
		u, err := resources.ToUnstructured(cli.Scheme(), &fetched)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(u.Object).To(And(
			jq.Match(`.metadata.labels.app == "test"`),
			jq.Match(`.metadata.labels.version == "v1"`),
			jq.Match(`.metadata.annotations.description == "test config"`),
		))

		// Cleanup
		err = cli.Delete(ctx, cm)
		g.Expect(err).ToNot(HaveOccurred())
	})

	t.Run("Apply with ForceOwnership", func(t *testing.T) {
		g := NewWithT(t)

		cm := &corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-apply-force-cm",
				Namespace: "default",
			},
			Data: map[string]string{
				"key": "value1",
			},
		}

		// First apply with one owner
		err := resources.Apply(ctx, cli, cm, client.FieldOwner("owner1"))
		g.Expect(err).ToNot(HaveOccurred())

		// Second apply with different owner and force
		cm.Data["key"] = "value2"
		err = resources.Apply(ctx, cli, cm, client.FieldOwner("owner2"), client.ForceOwnership)
		g.Expect(err).ToNot(HaveOccurred())

		var fetched corev1.ConfigMap
		err = cli.Get(ctx, client.ObjectKey{Name: "test-apply-force-cm", Namespace: "default"}, &fetched)
		g.Expect(err).ToNot(HaveOccurred())

		fetched.SetGroupVersionKind(cm.GroupVersionKind())
		u, err := resources.ToUnstructured(cli.Scheme(), &fetched)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(u.Object).To(jq.Match(`.data.key == "value2"`))

		// Cleanup
		err = cli.Delete(ctx, cm)
		g.Expect(err).ToNot(HaveOccurred())
	})

	t.Run("Apply writes back server values to input object", func(t *testing.T) {
		g := NewWithT(t)

		cm := &corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-preserve-cm",
				Namespace: "default",
			},
			Data: map[string]string{
				"key": "value",
			},
		}

		err := resources.Apply(ctx, cli, cm, client.FieldOwner("test-controller"))
		g.Expect(err).ToNot(HaveOccurred())

		// The input object should be updated with server values after Apply
		g.Expect(cm.ResourceVersion).ToNot(BeEmpty())
		g.Expect(cm.UID).ToNot(BeEmpty())

		// Cleanup
		err = cli.Delete(ctx, cm)
		g.Expect(err).ToNot(HaveOccurred())
	})

	t.Run("verify field manager is set correctly", func(t *testing.T) {
		g := NewWithT(t)

		cm := &corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-field-manager-cm",
				Namespace: "default",
			},
			Data: map[string]string{
				"key": "value",
			},
		}

		err := resources.Apply(ctx, cli, cm, client.FieldOwner("test-controller"))
		g.Expect(err).ToNot(HaveOccurred())

		// Verify the field manager is set
		var fetched corev1.ConfigMap
		err = cli.Get(ctx, client.ObjectKey{Name: "test-field-manager-cm", Namespace: "default"}, &fetched)
		g.Expect(err).ToNot(HaveOccurred())

		fetched.SetGroupVersionKind(cm.GroupVersionKind())
		u, err := resources.ToUnstructured(cli.Scheme(), &fetched)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(u.Object).To(jq.Match(`[.metadata.managedFields[] | select(.manager == "test-controller")] | length > 0`))

		// Cleanup
		err = cli.Delete(ctx, cm)
		g.Expect(err).ToNot(HaveOccurred())
	})

	t.Run("verify field manager changes with ForceOwnership", func(t *testing.T) {
		g := NewWithT(t)

		cm := &corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-manager-change-cm",
				Namespace: "default",
			},
			Data: map[string]string{
				"key": "value1",
			},
		}

		// First apply with owner1
		err := resources.Apply(ctx, cli, cm, client.FieldOwner("owner1"))
		g.Expect(err).ToNot(HaveOccurred())

		// Verify owner1 is the field manager
		var fetched corev1.ConfigMap
		err = cli.Get(ctx, client.ObjectKey{Name: "test-manager-change-cm", Namespace: "default"}, &fetched)
		g.Expect(err).ToNot(HaveOccurred())

		fetched.SetGroupVersionKind(cm.GroupVersionKind())
		u, err := resources.ToUnstructured(cli.Scheme(), &fetched)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(u.Object).To(jq.Match(`[.metadata.managedFields[] | select(.manager == "owner1")] | length > 0`))

		// Second apply with owner2 and force
		cm.Data["key"] = "value2"
		err = resources.Apply(ctx, cli, cm, client.FieldOwner("owner2"), client.ForceOwnership)
		g.Expect(err).ToNot(HaveOccurred())

		// Verify owner2 is now the field manager
		err = cli.Get(ctx, client.ObjectKey{Name: "test-manager-change-cm", Namespace: "default"}, &fetched)
		g.Expect(err).ToNot(HaveOccurred())

		fetched.SetGroupVersionKind(cm.GroupVersionKind())
		u, err = resources.ToUnstructured(cli.Scheme(), &fetched)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(u.Object).To(jq.Match(`[.metadata.managedFields[] | select(.manager == "owner2")] | length > 0`))

		// Cleanup
		err = cli.Delete(ctx, cm)
		g.Expect(err).ToNot(HaveOccurred())
	})
}

func TestApplyStatusWithK3sEnv(t *testing.T) {
	ctx := context.Background()

	s := runtime.NewScheme()
	if err := corev1.AddToScheme(s); err != nil {
		t.Fatalf("failed to add corev1 to scheme: %v", err)
	}
	if err := appsv1.AddToScheme(s); err != nil {
		t.Fatalf("failed to add appsv1 to scheme: %v", err)
	}

	env, err := k3senv.New(
		k3senv.WithScheme(s),
	)
	if err != nil {
		t.Fatalf("failed to create k3s env: %v", err)
	}

	err = env.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start k3s env: %v", err)
	}
	t.Cleanup(func() {
		_ = env.Stop(ctx)
	})

	cli := env.Client()

	t.Run("ApplyStatus for non-existent resource returns nil", func(t *testing.T) {
		g := NewWithT(t)

		cm := &corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "non-existent-cm",
				Namespace: "default",
			},
		}

		// ApplyStatus should return nil for non-existent resources
		err := resources.ApplyStatus(ctx, cli, cm, client.FieldOwner("test-controller"))
		g.Expect(err).ToNot(HaveOccurred())
	})

	t.Run("ApplyStatus after Apply", func(t *testing.T) {
		g := NewWithT(t)

		cm := &corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-apply-then-status-cm",
				Namespace: "default",
			},
			Data: map[string]string{
				"key": "value",
			},
		}

		// Create the resource first
		err := resources.Apply(ctx, cli, cm, client.FieldOwner("test-controller"))
		g.Expect(err).ToNot(HaveOccurred())

		// Then apply status (ConfigMap doesn't have status, but it shouldn't error)
		err = resources.ApplyStatus(ctx, cli, cm, client.FieldOwner("test-controller"))
		g.Expect(err).ToNot(HaveOccurred())

		// Verify the resource still exists
		var fetched corev1.ConfigMap
		err = cli.Get(ctx, client.ObjectKey{Name: "test-apply-then-status-cm", Namespace: "default"}, &fetched)
		g.Expect(err).ToNot(HaveOccurred())

		fetched.SetGroupVersionKind(cm.GroupVersionKind())
		u, err := resources.ToUnstructured(cli.Scheme(), &fetched)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(u.Object).To(jq.Match(`.data.key == "value"`))

		// Cleanup
		err = cli.Delete(ctx, cm)
		g.Expect(err).ToNot(HaveOccurred())
	})
}

func TestApplyErrorHandling(t *testing.T) {
	ctx := context.Background()

	s := runtime.NewScheme()
	if err := corev1.AddToScheme(s); err != nil {
		t.Fatalf("failed to add corev1 to scheme: %v", err)
	}
	if err := appsv1.AddToScheme(s); err != nil {
		t.Fatalf("failed to add appsv1 to scheme: %v", err)
	}

	env, err := k3senv.New(
		k3senv.WithScheme(s),
	)
	if err != nil {
		t.Fatalf("failed to create k3s env: %v", err)
	}

	err = env.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start k3s env: %v", err)
	}
	t.Cleanup(func() {
		_ = env.Stop(ctx)
	})

	cli := env.Client()

	t.Run("Apply with invalid namespace", func(t *testing.T) {
		g := NewWithT(t)

		cm := &corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-invalid-ns-cm",
				Namespace: "non-existent-namespace",
			},
			Data: map[string]string{
				"key": "value",
			},
		}

		err := resources.Apply(ctx, cli, cm, client.FieldOwner("test-controller"))
		g.Expect(err).To(HaveOccurred())
	})
}

func TestApplyMultipleResources(t *testing.T) {
	ctx := context.Background()

	s := runtime.NewScheme()
	if err := corev1.AddToScheme(s); err != nil {
		t.Fatalf("failed to add corev1 to scheme: %v", err)
	}
	if err := appsv1.AddToScheme(s); err != nil {
		t.Fatalf("failed to add appsv1 to scheme: %v", err)
	}

	env, err := k3senv.New(
		k3senv.WithScheme(s),
	)
	if err != nil {
		t.Fatalf("failed to create k3s env: %v", err)
	}

	err = env.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start k3s env: %v", err)
	}
	t.Cleanup(func() {
		_ = env.Stop(ctx)
	})

	cli := env.Client()

	t.Run("Apply multiple ConfigMaps", func(t *testing.T) {
		g := NewWithT(t)

		res := []corev1.ConfigMap{
			{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ConfigMap",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-multi-cm-1",
					Namespace: "default",
				},
				Data: map[string]string{"key": "value1"},
			},
			{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ConfigMap",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-multi-cm-2",
					Namespace: "default",
				},
				Data: map[string]string{"key": "value2"},
			},
			{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ConfigMap",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-multi-cm-3",
					Namespace: "default",
				},
				Data: map[string]string{"key": "value3"},
			},
		}

		// Apply all resources
		for i := range res {
			err := resources.Apply(ctx, cli, &res[i], client.FieldOwner("test-controller"))
			g.Expect(err).ToNot(HaveOccurred())
		}

		// Verify all resources exist
		var fetched corev1.ConfigMap
		gvk := res[0].GroupVersionKind()

		err = cli.Get(ctx, client.ObjectKey{Name: "test-multi-cm-1", Namespace: "default"}, &fetched)
		g.Expect(err).ToNot(HaveOccurred())
		fetched.SetGroupVersionKind(gvk)
		u, err := resources.ToUnstructured(cli.Scheme(), &fetched)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(u.Object).To(jq.Match(`.data.key == "value1"`))

		err = cli.Get(ctx, client.ObjectKey{Name: "test-multi-cm-2", Namespace: "default"}, &fetched)
		g.Expect(err).ToNot(HaveOccurred())
		fetched.SetGroupVersionKind(gvk)
		u, err = resources.ToUnstructured(cli.Scheme(), &fetched)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(u.Object).To(jq.Match(`.data.key == "value2"`))

		err = cli.Get(ctx, client.ObjectKey{Name: "test-multi-cm-3", Namespace: "default"}, &fetched)
		g.Expect(err).ToNot(HaveOccurred())
		fetched.SetGroupVersionKind(gvk)
		u, err = resources.ToUnstructured(cli.Scheme(), &fetched)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(u.Object).To(jq.Match(`.data.key == "value3"`))

		// Cleanup
		for i := range res {
			err = cli.Delete(ctx, &res[i])
			g.Expect(err).ToNot(HaveOccurred())
		}
	})
}

func TestApplyWithNamespace(t *testing.T) {
	ctx := context.Background()

	s := runtime.NewScheme()
	if err := corev1.AddToScheme(s); err != nil {
		t.Fatalf("failed to add corev1 to scheme: %v", err)
	}
	if err := appsv1.AddToScheme(s); err != nil {
		t.Fatalf("failed to add appsv1 to scheme: %v", err)
	}

	env, err := k3senv.New(
		k3senv.WithScheme(s),
	)
	if err != nil {
		t.Fatalf("failed to create k3s env: %v", err)
	}

	err = env.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start k3s env: %v", err)
	}
	t.Cleanup(func() {
		_ = env.Stop(ctx)
	})

	cli := env.Client()

	t.Run("Apply creates namespace and resource", func(t *testing.T) {
		g := NewWithT(t)

		// Create a namespace first
		ns := &corev1.Namespace{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Namespace",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-ns",
			},
		}
		err := resources.Apply(ctx, cli, ns, client.FieldOwner("test-controller"))
		g.Expect(err).ToNot(HaveOccurred())

		// Create a ConfigMap in the new namespace
		cm := &corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cm-in-ns",
				Namespace: "test-ns",
			},
			Data: map[string]string{
				"key": "value",
			},
		}
		err = resources.Apply(ctx, cli, cm, client.FieldOwner("test-controller"))
		g.Expect(err).ToNot(HaveOccurred())

		// Verify the ConfigMap exists in the namespace
		var fetched corev1.ConfigMap
		err = cli.Get(ctx, client.ObjectKey{Name: "test-cm-in-ns", Namespace: "test-ns"}, &fetched)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(fetched.Namespace).To(Equal("test-ns"))

		// Cleanup
		err = cli.Delete(ctx, cm)
		g.Expect(err).ToNot(HaveOccurred())
		err = cli.Delete(ctx, ns)
		g.Expect(err).ToNot(HaveOccurred())
	})
}
