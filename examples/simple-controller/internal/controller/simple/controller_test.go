package simple_test

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"github.com/lburgazzoli/gomega-matchers/pkg/matchers/jq"
	"github.com/lburgazzoli/k3s-envtest/pkg/k3senv"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/envtest/komega"

	simpleApi "github.com/lburgazzoli/k8s-controller-lib/examples/simple-controller/api/v1alpha1"
	"github.com/lburgazzoli/k8s-controller-lib/examples/simple-controller/internal/controller/simple"
	"github.com/lburgazzoli/k8s-controller-lib/pkg/resources"
)

func TestSimpleControllerIntegration(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	s := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(s)).To(Succeed())
	g.Expect(appsv1.AddToScheme(s)).To(Succeed())
	g.Expect(apiextensionsv1.AddToScheme(s)).To(Succeed())
	g.Expect(simpleApi.AddToScheme(s)).To(Succeed())

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
				"test-ns2": {},
			},
		},
	})
	g.Expect(err).ToNot(HaveOccurred())

	err = simple.SetupWithManager(mgr)
	g.Expect(err).ToNot(HaveOccurred())

	go func() {
		_ = mgr.Start(ctx)
	}()

	cacheCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	g.Expect(mgr.GetCache().WaitForCacheSync(cacheCtx)).To(BeTrue())

	cli := mgr.GetClient()
	k := komega.New(cli)

	t.Run("create SimpleApp creates Deployment", func(t *testing.T) {
		g := NewWithT(t)

		app := &simpleApi.SimpleApp{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-app",
				Namespace: "default",
			},
			Spec: simpleApi.SimpleAppSpec{
				Image:    "nginx:latest",
				Replicas: 2,
				Port:     8080,
			},
		}

		g.Expect(cli.Create(ctx, app)).To(Succeed())

		deploy := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-app",
				Namespace: "default",
			},
		}
		g.Eventually(k.Get(deploy)).Should(Succeed())

		deploy.SetGroupVersionKind(appsv1.SchemeGroupVersion.WithKind("Deployment"))
		u, err := resources.ToUnstructured(cli.Scheme(), deploy)
		g.Expect(err).ToNot(HaveOccurred())

		g.Expect(u.Object).To(And(
			jq.Match(`.spec.replicas == 2`),
			jq.Match(`.spec.template.spec.containers[0].image == "nginx:latest"`),
			jq.Match(`.spec.template.spec.containers[0].ports[0].containerPort == 8080`),
			jq.Match(`.metadata.labels.app == "test-app"`),
			jq.Match(`.metadata.labels."managed-by" == "simple-controller"`),
			jq.Match(`.metadata.labels."app.kubernetes.io/name" == "test-app"`),
		))

		g.Expect(cli.Delete(ctx, app)).To(Succeed())
	})

	t.Run("update SimpleApp updates Deployment", func(t *testing.T) {
		g := NewWithT(t)

		app := &simpleApi.SimpleApp{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-update-app",
				Namespace: "default",
			},
			Spec: simpleApi.SimpleAppSpec{
				Image:    "nginx:1.20",
				Replicas: 1,
				Port:     8080,
			},
		}

		g.Expect(cli.Create(ctx, app)).To(Succeed())

		deploy := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-update-app",
				Namespace: "default",
			},
		}
		g.Eventually(k.Get(deploy)).Should(Succeed())

		currentApp := &simpleApi.SimpleApp{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-update-app",
				Namespace: "default",
			},
		}
		g.Eventually(k.Update(currentApp, func() {
			currentApp.Spec.Image = "nginx:1.21"
			currentApp.Spec.Replicas = 3
		})).Should(Succeed())

		g.Eventually(func() bool {
			deploy := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-update-app",
					Namespace: "default",
				},
			}
			if err := k.Get(deploy)(); err != nil {
				return false
			}
			return *deploy.Spec.Replicas == 3 &&
				deploy.Spec.Template.Spec.Containers[0].Image == "nginx:1.21"
		}).Should(BeTrue())

		g.Expect(cli.Delete(ctx, app)).To(Succeed())
	})

	t.Run("status updates with ObservedGeneration and conditions", func(t *testing.T) {
		g := NewWithT(t)

		app := &simpleApi.SimpleApp{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-status-app",
				Namespace: "default",
			},
			Spec: simpleApi.SimpleAppSpec{
				Image:    "nginx:alpine",
				Replicas: 1,
				Port:     8080,
			},
		}

		g.Expect(cli.Create(ctx, app)).To(Succeed())

		g.Eventually(func() bool {
			currentApp := &simpleApi.SimpleApp{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-status-app",
					Namespace: "default",
				},
			}
			if err := k.Get(currentApp)(); err != nil {
				return false
			}
			return currentApp.Status.ObservedGeneration > 0
		}).Should(BeTrue())

		currentApp := &simpleApi.SimpleApp{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-status-app",
				Namespace: "default",
			},
		}
		g.Eventually(k.Get(currentApp)).Should(Succeed())

		g.Expect(currentApp.Status.ObservedGeneration).To(Equal(currentApp.Generation))
		g.Expect(currentApp.Status.Conditions).ToNot(BeEmpty())

		foundProvisioningCondition := false
		for _, cond := range currentApp.Status.Conditions {
			if cond.Type == "ProvisioningSucceeded" {
				foundProvisioningCondition = true
				g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))
			}
		}
		g.Expect(foundProvisioningCondition).To(BeTrue())

		g.Expect(cli.Delete(ctx, app)).To(Succeed())
	})

	t.Run("default values are applied correctly", func(t *testing.T) {
		g := NewWithT(t)

		app := &simpleApi.SimpleApp{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-defaults-app",
				Namespace: "default",
			},
			Spec: simpleApi.SimpleAppSpec{
				Image: "busybox:latest",
			},
		}

		g.Expect(cli.Create(ctx, app)).To(Succeed())

		deploy := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-defaults-app",
				Namespace: "default",
			},
		}
		g.Eventually(k.Get(deploy)).Should(Succeed())

		deploy.SetGroupVersionKind(appsv1.SchemeGroupVersion.WithKind("Deployment"))
		u, err := resources.ToUnstructured(cli.Scheme(), deploy)
		g.Expect(err).ToNot(HaveOccurred())

		g.Expect(u.Object).To(And(
			jq.Match(`.spec.replicas == 1`),
			jq.Match(`.spec.template.spec.containers[0].ports[0].containerPort == 8080`),
		))

		g.Expect(cli.Delete(ctx, app)).To(Succeed())
	})

	t.Run("field manager is set correctly", func(t *testing.T) {
		g := NewWithT(t)

		app := &simpleApi.SimpleApp{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-fieldmgr-app",
				Namespace: "default",
			},
			Spec: simpleApi.SimpleAppSpec{
				Image:    "redis:alpine",
				Replicas: 1,
				Port:     6379,
			},
		}

		g.Expect(cli.Create(ctx, app)).To(Succeed())

		deploy := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-fieldmgr-app",
				Namespace: "default",
			},
		}
		g.Eventually(k.Get(deploy)).Should(Succeed())

		deploy.SetGroupVersionKind(appsv1.SchemeGroupVersion.WithKind("Deployment"))
		u, err := resources.ToUnstructured(cli.Scheme(), deploy)
		g.Expect(err).ToNot(HaveOccurred())

		g.Expect(u.Object).To(jq.Match(`[.metadata.managedFields[] | select(.manager == "simple-controller")] | length > 0`))

		g.Expect(cli.Delete(ctx, app)).To(Succeed())
	})

	t.Run("multiple SimpleApps in different namespaces", func(t *testing.T) {
		g := NewWithT(t)

		ns1 := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-ns1",
			},
		}
		g.Expect(cli.Create(ctx, ns1)).To(Succeed())

		ns2 := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-ns2",
			},
		}
		g.Expect(cli.Create(ctx, ns2)).To(Succeed())

		app1 := &simpleApi.SimpleApp{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "multi-app",
				Namespace: "test-ns1",
			},
			Spec: simpleApi.SimpleAppSpec{
				Image:    "nginx:1.20",
				Replicas: 2,
				Port:     8080,
			},
		}
		g.Expect(cli.Create(ctx, app1)).To(Succeed())

		app2 := &simpleApi.SimpleApp{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "multi-app",
				Namespace: "test-ns2",
			},
			Spec: simpleApi.SimpleAppSpec{
				Image:    "redis:alpine",
				Replicas: 3,
				Port:     6379,
			},
		}
		g.Expect(cli.Create(ctx, app2)).To(Succeed())

		deploy1 := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "multi-app",
				Namespace: "test-ns1",
			},
		}
		g.Eventually(k.Get(deploy1)).Should(Succeed())

		deploy2 := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "multi-app",
				Namespace: "test-ns2",
			},
		}
		g.Eventually(k.Get(deploy2)).Should(Succeed())

		g.Expect(*deploy1.Spec.Replicas).To(Equal(int32(2)))
		g.Expect(deploy1.Spec.Template.Spec.Containers[0].Image).To(Equal("nginx:1.20"))

		g.Expect(*deploy2.Spec.Replicas).To(Equal(int32(3)))
		g.Expect(deploy2.Spec.Template.Spec.Containers[0].Image).To(Equal("redis:alpine"))

		g.Expect(cli.Delete(ctx, app1)).To(Succeed())
		g.Expect(cli.Delete(ctx, app2)).To(Succeed())
		g.Expect(cli.Delete(ctx, ns1)).To(Succeed())
		g.Expect(cli.Delete(ctx, ns2)).To(Succeed())
	})

	t.Run("template renders resource limits correctly", func(t *testing.T) {
		g := NewWithT(t)

		app := &simpleApi.SimpleApp{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-resources-app",
				Namespace: "default",
			},
			Spec: simpleApi.SimpleAppSpec{
				Image:    "nginx:stable",
				Replicas: 1,
				Port:     8080,
			},
		}

		g.Expect(cli.Create(ctx, app)).To(Succeed())

		deploy := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-resources-app",
				Namespace: "default",
			},
		}
		g.Eventually(k.Get(deploy)).Should(Succeed())

		deploy.SetGroupVersionKind(appsv1.SchemeGroupVersion.WithKind("Deployment"))
		u, err := resources.ToUnstructured(cli.Scheme(), deploy)
		g.Expect(err).ToNot(HaveOccurred())

		g.Expect(u.Object).To(And(
			jq.Match(`.spec.template.spec.containers[0].resources.limits.cpu == "1"`),
			jq.Match(`.spec.template.spec.containers[0].resources.limits.memory == "512Mi"`),
			jq.Match(`.spec.template.spec.containers[0].resources.requests.cpu == "100m"`),
			jq.Match(`.spec.template.spec.containers[0].resources.requests.memory == "128Mi"`),
		))

		g.Expect(cli.Delete(ctx, app)).To(Succeed())
	})
}
