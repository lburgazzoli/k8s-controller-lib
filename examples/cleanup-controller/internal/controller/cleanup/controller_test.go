package cleanup_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/lburgazzoli/k3s-envtest/pkg/k3senv"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/envtest/komega"

	cleanupApi "github.com/lburgazzoli/k8s-controller-lib/examples/cleanup-controller/api/v1alpha1"
	"github.com/lburgazzoli/k8s-controller-lib/examples/cleanup-controller/internal/controller/cleanup"
)

const (
	defaultFinalizer = "reconciler.k8s-controller-lib/finalizer"
)

func TestCleanupControllerIntegration(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	s := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(s)).To(Succeed())
	g.Expect(apiextensionsv1.AddToScheme(s)).To(Succeed())
	g.Expect(cleanupApi.AddToScheme(s)).To(Succeed())

	env, err := k3senv.New(
		k3senv.WithScheme(s),
		k3senv.WithManifests("config/crds/bases"),
	)
	g.Expect(err).ToNot(HaveOccurred())

	err = env.Start(ctx)
	g.Expect(err).ToNot(HaveOccurred())
	t.Cleanup(func() {
		_ = env.Stop(ctx)
	})

	mgr, err := ctrl.NewManager(env.Config(), ctrl.Options{
		Scheme: s,
		Cache: cache.Options{
			DefaultNamespaces: map[string]cache.Config{
				"default":  {},
				"test-ns1": {},
			},
		},
	})
	g.Expect(err).ToNot(HaveOccurred())

	err = cleanup.SetupWithManager(mgr)
	g.Expect(err).ToNot(HaveOccurred())

	go func() {
		_ = mgr.Start(ctx)
	}()

	cacheCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	g.Expect(mgr.GetCache().WaitForCacheSync(cacheCtx)).To(BeTrue())

	cli := mgr.GetClient()
	k := komega.New(cli)

	t.Run("create CleanupApp creates ConfigMap without owner references", func(t *testing.T) {
		g := NewWithT(t)

		app := &cleanupApi.CleanupApp{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-app",
				Namespace: "default",
			},
			Spec: cleanupApi.CleanupAppSpec{
				ConfigMapName: "my-test-config",
				Data: map[string]string{
					"key1": "value1",
					"key2": "value2",
				},
			},
		}

		g.Expect(cli.Create(ctx, app)).To(Succeed())
		t.Cleanup(func() {
			_ = cli.Delete(ctx, app)
		})

		// Check if reconciliation happened by looking at status
		g.Eventually(func() string {
			if err := cli.Get(ctx, client.ObjectKeyFromObject(app), app); err != nil {
				return fmt.Sprintf("error getting app: %v", err)
			}
			if app.Status.ObservedGeneration == 0 {
				return fmt.Sprintf("observedGeneration=0, conditions=%d", len(app.Status.Conditions))
			}
			if len(app.Status.Conditions) > 0 {
				lastCond := app.Status.Conditions[len(app.Status.Conditions)-1]
				if lastCond.Status != "True" {
					return fmt.Sprintf("condition %s=%s: %s", lastCond.Type, lastCond.Status, lastCond.Message)
				}
			}
			return "reconciled"
		}, 10*time.Second).Should(Equal("reconciled"), "Controller should have reconciled")

		// Verify ConfigMap is created
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-test-config",
				Namespace: "default",
			},
		}
		g.Eventually(k.Get(cm), 10*time.Second).Should(Succeed())

		// Verify ConfigMap has NO owner references
		g.Expect(cm.OwnerReferences).To(BeEmpty(), "ConfigMap should not have owner references")

		// Verify ConfigMap has correct labels
		g.Expect(cm.Labels).To(HaveKeyWithValue("app.kubernetes.io/name", "test-app"))
		g.Expect(cm.Labels).To(HaveKeyWithValue("app.kubernetes.io/managed-by", "unmanaged"))

		// Verify ConfigMap data
		g.Expect(cm.Data).To(HaveKeyWithValue("key1", "value1"))
		g.Expect(cm.Data).To(HaveKeyWithValue("key2", "value2"))
	})

	t.Run("finalizer is added automatically", func(t *testing.T) {
		g := NewWithT(t)

		app := &cleanupApi.CleanupApp{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-finalizer",
				Namespace: "default",
			},
			Spec: cleanupApi.CleanupAppSpec{
				ConfigMapName: "finalizer-test-config",
				Data: map[string]string{
					"test": "data",
				},
			},
		}

		g.Expect(cli.Create(ctx, app)).To(Succeed())
		t.Cleanup(func() {
			_ = cli.Delete(ctx, app)
		})

		// Wait for ConfigMap to ensure reconciliation happened
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "finalizer-test-config",
				Namespace: "default",
			},
		}
		g.Eventually(k.Get(cm)).Should(Succeed())

		// Verify finalizer is added
		g.Eventually(func() bool {
			if err := cli.Get(ctx, client.ObjectKeyFromObject(app), app); err != nil {
				return false
			}
			return controllerutil.ContainsFinalizer(app, defaultFinalizer)
		}).Should(BeTrue(), "Pipeline should automatically add finalizer")
	})

	t.Run("cleanup action deletes ConfigMap on deletion", func(t *testing.T) {
		g := NewWithT(t)

		app := &cleanupApi.CleanupApp{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cleanup",
				Namespace: "default",
			},
			Spec: cleanupApi.CleanupAppSpec{
				ConfigMapName: "cleanup-test-config",
				Data: map[string]string{
					"cleanup": "test",
				},
			},
		}

		g.Expect(cli.Create(ctx, app)).To(Succeed())

		// Wait for ConfigMap creation
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cleanup-test-config",
				Namespace: "default",
			},
		}
		g.Eventually(k.Get(cm)).Should(Succeed())

		// Wait for finalizer
		g.Eventually(func() bool {
			if err := cli.Get(ctx, client.ObjectKeyFromObject(app), app); err != nil {
				return false
			}
			return controllerutil.ContainsFinalizer(app, defaultFinalizer)
		}).Should(BeTrue())

		// Delete the CleanupApp
		g.Expect(cli.Delete(ctx, app)).To(Succeed())

		// Verify ConfigMap is deleted by cleanup action
		g.Eventually(func() bool {
			err := cli.Get(ctx, client.ObjectKeyFromObject(cm), cm)
			return errors.IsNotFound(err)
		}).Should(BeTrue(), "ConfigMap should be deleted by cleanup action")

		// Verify CleanupApp is eventually deleted (finalizer removed)
		g.Eventually(func() bool {
			err := cli.Get(ctx, client.ObjectKeyFromObject(app), app)
			return errors.IsNotFound(err)
		}).Should(BeTrue(), "CleanupApp should be deleted after cleanup completes")
	})

	t.Run("cleanup is idempotent - handles already deleted ConfigMap", func(t *testing.T) {
		g := NewWithT(t)

		app := &cleanupApi.CleanupApp{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-idempotent",
				Namespace: "default",
			},
			Spec: cleanupApi.CleanupAppSpec{
				ConfigMapName: "idempotent-test-config",
				Data: map[string]string{
					"data": "value",
				},
			},
		}

		g.Expect(cli.Create(ctx, app)).To(Succeed())

		// Wait for ConfigMap creation
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "idempotent-test-config",
				Namespace: "default",
			},
		}
		g.Eventually(k.Get(cm)).Should(Succeed())

		// Wait for finalizer
		g.Eventually(func() bool {
			if err := cli.Get(ctx, client.ObjectKeyFromObject(app), app); err != nil {
				return false
			}
			return controllerutil.ContainsFinalizer(app, defaultFinalizer)
		}).Should(BeTrue())

		// Manually delete the ConfigMap before deleting CleanupApp
		g.Expect(cli.Delete(ctx, cm)).To(Succeed())

		// Verify ConfigMap is gone
		g.Eventually(func() bool {
			err := cli.Get(ctx, client.ObjectKeyFromObject(cm), cm)
			return errors.IsNotFound(err)
		}).Should(BeTrue())

		// Delete the CleanupApp
		g.Expect(cli.Delete(ctx, app)).To(Succeed())

		// Cleanup should succeed even though ConfigMap already deleted
		// Verify CleanupApp is eventually deleted
		g.Eventually(func() bool {
			err := cli.Get(ctx, client.ObjectKeyFromObject(app), app)
			return errors.IsNotFound(err)
		}).Should(BeTrue(), "Cleanup should be idempotent and succeed even if ConfigMap already deleted")
	})

	t.Run("ConfigMap data updates when spec changes", func(t *testing.T) {
		g := NewWithT(t)

		app := &cleanupApi.CleanupApp{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-update",
				Namespace: "default",
			},
			Spec: cleanupApi.CleanupAppSpec{
				ConfigMapName: "update-test-config",
				Data: map[string]string{
					"original": "value",
				},
			},
		}

		g.Expect(cli.Create(ctx, app)).To(Succeed())
		t.Cleanup(func() {
			_ = cli.Delete(ctx, app)
		})

		// Wait for ConfigMap with initial data
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "update-test-config",
				Namespace: "default",
			},
		}
		g.Eventually(k.Get(cm)).Should(Succeed())
		g.Expect(cm.Data).To(HaveKeyWithValue("original", "value"))

		// Update the CleanupApp spec
		g.Eventually(k.Update(app, func() {
			app.Spec.Data = map[string]string{
				"updated": "newvalue",
				"added":   "key",
			}
		})).Should(Succeed())

		// Verify ConfigMap is updated
		g.Eventually(func() bool {
			if err := cli.Get(ctx, client.ObjectKeyFromObject(cm), cm); err != nil {
				return false
			}
			return cm.Data["updated"] == "newvalue" && cm.Data["added"] == "key"
		}).Should(BeTrue(), "ConfigMap should be updated with new data")
	})

	t.Run("status is updated with conditions", func(t *testing.T) {
		g := NewWithT(t)

		app := &cleanupApi.CleanupApp{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-status",
				Namespace: "default",
			},
			Spec: cleanupApi.CleanupAppSpec{
				ConfigMapName: "status-test-config",
				Data: map[string]string{
					"test": "status",
				},
			},
		}

		g.Expect(cli.Create(ctx, app)).To(Succeed())
		t.Cleanup(func() {
			_ = cli.Delete(ctx, app)
		})

		// Wait for ConfigMap to ensure reconciliation
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "status-test-config",
				Namespace: "default",
			},
		}
		g.Eventually(k.Get(cm)).Should(Succeed())

		// Verify status is updated
		g.Eventually(func() bool {
			if err := cli.Get(ctx, client.ObjectKeyFromObject(app), app); err != nil {
				return false
			}
			// ObservedGeneration should match current generation
			if app.Status.ObservedGeneration != app.Generation {
				return false
			}
			// Should have provisioning condition (set by pipeline)
			for _, cond := range app.Status.Conditions {
				if cond.Type == "ProvisioningSucceeded" && cond.Status == "True" {
					return true
				}
			}
			return false
		}).Should(BeTrue(), "Status should be updated with ProvisioningSucceeded condition")
	})
}
