//nolint:fatcontext
package pipeline_test

import (
	"context"
	"testing"

	"github.com/lburgazzoli/k3s-envtest/pkg/k3senv"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/reconciler"
	"github.com/lburgazzoli/k8s-controller-lib/pkg/reconciler/pipeline"
	"github.com/lburgazzoli/k8s-controller-lib/pkg/reconciler/watch"
	"github.com/lburgazzoli/k8s-controller-lib/pkg/resources/gvks"
	"github.com/lburgazzoli/k8s-controller-lib/tests/integration/support"

	. "github.com/onsi/gomega"
)

func TestPipelineAutoWatch(t *testing.T) {
	ctx := context.Background()

	s := runtime.NewScheme()

	// Register TestResource with the scheme (needed for SSA and owner references)
	schemeBuilder := runtime.NewSchemeBuilder(func(s *runtime.Scheme) error {
		gv := schema.GroupVersion{Group: "test.example.com", Version: "v1"}
		s.AddKnownTypes(gv, &TestResource{}, &TestResourceList{})
		metav1.AddToGroupVersion(s, gv)

		return nil
	})

	// These checks will be done in the first subtest
	env, err := k3senv.New(
		k3senv.WithScheme(s),
	)
	if err != nil {
		t.Fatalf("Failed to create env: %v", err)
	}

	err = env.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start env: %v", err)
	}
	t.Cleanup(func() {
		_ = env.Stop(ctx)
	})

	// Create cache
	cacheObj, err := cache.New(env.Config(), cache.Options{
		Scheme: s,
	})
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Start cache in background
	cacheCtx, cacheCancel := context.WithCancel(ctx)
	t.Cleanup(func() {
		cacheCancel()
	})

	go func() {
		_ = cacheObj.Start(cacheCtx)
	}()

	cli := env.Client()

	t.Run("auto-watch with default controller name from owner kind", func(t *testing.T) {
		g := NewWithT(t)

		// Verify scheme setup and CRD registration
		g.Expect(corev1.AddToScheme(s)).ShouldNot(HaveOccurred())
		g.Expect(appsv1.AddToScheme(s)).ShouldNot(HaveOccurred())
		g.Expect(apiextensionsv1.AddToScheme(s)).ShouldNot(HaveOccurred())
		g.Expect(schemeBuilder.AddToScheme(s)).ShouldNot(HaveOccurred())

		// Create TestResource CRD
		testResourceCRD := &apiextensionsv1.CustomResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name: "testresources.test.example.com",
			},
			Spec: apiextensionsv1.CustomResourceDefinitionSpec{
				Group: "test.example.com",
				Names: apiextensionsv1.CustomResourceDefinitionNames{
					Kind:     "TestResource",
					ListKind: "TestResourceList",
					Plural:   "testresources",
					Singular: "testresource",
				},
				Scope: apiextensionsv1.NamespaceScoped,
				Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
					{
						Name:    "v1",
						Served:  true,
						Storage: true,
						Schema: &apiextensionsv1.CustomResourceValidation{
							OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
								Type: "object",
								Properties: map[string]apiextensionsv1.JSONSchemaProps{
									"spec": {
										Type: "object",
										Properties: map[string]apiextensionsv1.JSONSchemaProps{
											"field": {Type: "string"},
										},
									},
									"status": {
										Type: "object",
										XPreserveUnknownFields: func() *bool {
											b := true

											return &b
										}(),
									},
								},
							},
						},
						Subresources: &apiextensionsv1.CustomResourceSubresources{
							Status: &apiextensionsv1.CustomResourceSubresourceStatus{},
						},
					},
				},
			},
		}
		g.Expect(env.InstallCRD(ctx, testResourceCRD)).To(Succeed())

		// Wait for cache to sync
		g.Expect(cacheObj.WaitForCacheSync(ctx)).To(BeTrue())

		ctrl, err := controller.NewUnmanaged("test-default-name", controller.Options{
			Reconciler: &support.NoOpReconciler{},
		})
		g.Expect(err).ToNot(HaveOccurred())

		// Create pipeline with auto-watch (no custom controller name)
		p, err := pipeline.NewPipeline(cli,
			pipeline.WithFieldOwner("test-controller"),
			pipeline.WithAutoWatch(ctrl, cacheObj),
			pipeline.WithActions(func(_ context.Context, _ *reconciler.Request, resp *reconciler.Response) error {
				// Provision a ConfigMap and a Secret
				cm := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-cm",
						Namespace: "default",
					},
				}
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-secret",
						Namespace: "default",
					},
				}
				resp.Objects(cm, secret)

				return nil
			}),
		)
		g.Expect(err).ToNot(HaveOccurred())

		// Create owner object in cluster
		owner := &TestResource{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "test.example.com/v1",
				Kind:       "TestResource",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-owner",
				Namespace: "default",
			},
		}
		g.Expect(cli.Create(ctx, owner)).To(Succeed())
		t.Cleanup(func() {
			_ = cli.Delete(ctx, owner)
		})

		// Reconcile to trigger watch setup
		// Inject controller name into context (lowercase kind of TestResource)
		ctx = reconciler.WithControllerName(ctx, "testresource")
		_, err = p.Reconcile(ctx, owner)
		g.Expect(err).ToNot(HaveOccurred())

		// Verify metrics - controller name should be "testresource" (lowercase kind)
		cmValue := testutil.ToFloat64(watch.DynamicWatchedResourcesTotal.WithLabelValues(
			"testresource",
			"v1",
			"ConfigMap",
		))
		g.Expect(cmValue).To(Equal(1.0))

		secretValue := testutil.ToFloat64(watch.DynamicWatchedResourcesTotal.WithLabelValues(
			"testresource",
			"v1",
			"Secret",
		))
		g.Expect(secretValue).To(Equal(1.0))
	})

	t.Run("auto-watch with custom controller name", func(t *testing.T) {
		g := NewWithT(t)

		ctrl, err := controller.NewUnmanaged("test-custom-name", controller.Options{
			Reconciler: &support.NoOpReconciler{},
		})
		g.Expect(err).ToNot(HaveOccurred())

		// Create pipeline with custom controller name
		p, err := pipeline.NewPipeline(cli,
			pipeline.WithFieldOwner("test-controller"),
			pipeline.WithAutoWatch(ctrl, cacheObj),
			pipeline.WithActions(func(_ context.Context, _ *reconciler.Request, resp *reconciler.Response) error {
				// Provision a Deployment and a Service
				deploy := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-deploy",
						Namespace: "default",
					},
					Spec: appsv1.DeploymentSpec{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "test"},
						},
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{"app": "test"},
							},
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{Name: "test", Image: "test:latest"},
								},
							},
						},
					},
				}
				svc := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-svc",
						Namespace: "default",
					},
					Spec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{
							{Port: 80},
						},
					},
				}
				resp.Objects(deploy, svc)

				return nil
			}),
		)
		g.Expect(err).ToNot(HaveOccurred())

		// Create owner object in cluster
		owner := &TestResource{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "test.example.com/v1",
				Kind:       "TestResource",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-owner-2",
				Namespace: "default",
			},
		}
		g.Expect(cli.Create(ctx, owner)).To(Succeed())
		t.Cleanup(func() {
			_ = cli.Delete(ctx, owner)
		})

		// Reconcile to trigger watch setup
		// Inject controller name into context (custom name set via WithControllerName)
		ctx = reconciler.WithControllerName(ctx, "my-custom-controller")
		_, err = p.Reconcile(ctx, owner)
		g.Expect(err).ToNot(HaveOccurred())

		// Verify metrics - controller name should be "my-custom-controller"
		deployValue := testutil.ToFloat64(watch.DynamicWatchedResourcesTotal.WithLabelValues(
			"my-custom-controller",
			"apps/v1",
			"Deployment",
		))
		g.Expect(deployValue).To(Equal(1.0))

		svcValue := testutil.ToFloat64(watch.DynamicWatchedResourcesTotal.WithLabelValues(
			"my-custom-controller",
			"v1",
			"Service",
		))
		g.Expect(svcValue).To(Equal(1.0))
	})

	t.Run("auto-watch with disabled GVK", func(t *testing.T) {
		g := NewWithT(t)

		ctrl, err := controller.NewUnmanaged("test-disabled", controller.Options{
			Reconciler: &support.NoOpReconciler{},
		})
		g.Expect(err).ToNot(HaveOccurred())

		// Create pipeline with Secret GVK disabled and custom controller name to avoid metric collision
		p, err := pipeline.NewPipeline(cli,
			pipeline.WithFieldOwner("test-controller"),
			pipeline.WithAutoWatch(ctrl, cacheObj,
				watch.For(gvks.Secret, watch.Disabled()),
			),
			pipeline.WithActions(func(_ context.Context, _ *reconciler.Request, resp *reconciler.Response) error {
				// Provision both ConfigMap (enabled) and Secret (disabled)
				cm := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-cm-disabled",
						Namespace: "default",
					},
				}
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-secret-disabled",
						Namespace: "default",
					},
				}
				resp.Objects(cm, secret)

				return nil
			}),
		)
		g.Expect(err).ToNot(HaveOccurred())

		// Create owner object in cluster
		owner := &TestResource{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "test.example.com/v1",
				Kind:       "TestResource",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-owner-3",
				Namespace: "default",
			},
		}
		g.Expect(cli.Create(ctx, owner)).To(Succeed())
		t.Cleanup(func() {
			_ = cli.Delete(ctx, owner)
		})

		// Reconcile to trigger watch setup
		// Inject controller name into context (custom name set via WithControllerName)
		ctx = reconciler.WithControllerName(ctx, "test-disabled-controller")
		_, err = p.Reconcile(ctx, owner)
		g.Expect(err).ToNot(HaveOccurred())

		// Verify ConfigMap watch was registered
		cmValue := testutil.ToFloat64(watch.DynamicWatchedResourcesTotal.WithLabelValues(
			"test-disabled-controller",
			"v1",
			"ConfigMap",
		))
		g.Expect(cmValue).To(Equal(1.0))

		// Verify Secret watch was NOT registered (disabled)
		secretValue := testutil.ToFloat64(watch.DynamicWatchedResourcesTotal.WithLabelValues(
			"test-disabled-controller",
			"v1",
			"Secret",
		))
		g.Expect(secretValue).To(Equal(0.0))
	})

	t.Run("auto-watch with custom predicates", func(t *testing.T) {
		g := NewWithT(t)

		ctrl, err := controller.NewUnmanaged("test-predicates", controller.Options{
			Reconciler: &support.NoOpReconciler{},
		})
		g.Expect(err).ToNot(HaveOccurred())

		// Create pipeline with custom predicate for ConfigMap
		p, err := pipeline.NewPipeline(cli,
			pipeline.WithFieldOwner("test-controller"),
			pipeline.WithAutoWatch(ctrl, cacheObj,
				watch.For(gvks.ConfigMap,
					watch.WithPredicates(predicate.GenerationChangedPredicate{}),
				),
			),
			pipeline.WithActions(func(_ context.Context, _ *reconciler.Request, resp *reconciler.Response) error {
				cm := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-cm-predicate",
						Namespace: "default",
					},
				}
				resp.Objects(cm)

				return nil
			}),
		)
		g.Expect(err).ToNot(HaveOccurred())

		// Create owner object in cluster
		owner := &TestResource{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "test.example.com/v1",
				Kind:       "TestResource",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-owner-4",
				Namespace: "default",
			},
		}
		g.Expect(cli.Create(ctx, owner)).To(Succeed())
		t.Cleanup(func() {
			_ = cli.Delete(ctx, owner)
		})

		// Reconcile to trigger watch setup
		// Inject controller name into context (lowercase kind of TestResource)
		ctx = reconciler.WithControllerName(ctx, "testresource")
		_, err = p.Reconcile(ctx, owner)
		g.Expect(err).ToNot(HaveOccurred())

		// Verify watch was registered with custom predicate
		cmValue := testutil.ToFloat64(watch.DynamicWatchedResourcesTotal.WithLabelValues(
			"testresource",
			"v1",
			"ConfigMap",
		))
		g.Expect(cmValue).To(Equal(1.0))
	})

	t.Run("auto-watch with external watches skips already-watched GVKs", func(t *testing.T) {
		g := NewWithT(t)

		ctrl, err := controller.NewUnmanaged("test-external-watches", controller.Options{
			Reconciler: &support.NoOpReconciler{},
		})
		g.Expect(err).ToNot(HaveOccurred())

		// Create pipeline with external watches - ConfigMap is already watched externally (e.g., by Builder)
		// This simulates what happens when Builder.Complete() injects external watches via ExternalWatchesAware
		p, err := pipeline.NewPipeline(cli,
			pipeline.WithFieldOwner("test-controller"),
			pipeline.WithAutoWatch(ctrl, cacheObj,
				// Mark ConfigMap as already watched externally
				watch.WithExternalWatches(gvks.ConfigMap),
			),
			pipeline.WithActions(func(_ context.Context, _ *reconciler.Request, resp *reconciler.Response) error {
				// Provision both ConfigMap (external) and Secret (should be auto-watched)
				cm := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-cm-external",
						Namespace: "default",
					},
				}
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-secret-external",
						Namespace: "default",
					},
				}
				resp.Objects(cm, secret)

				return nil
			}),
		)
		g.Expect(err).ToNot(HaveOccurred())

		// Create owner object in cluster
		owner := &TestResource{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "test.example.com/v1",
				Kind:       "TestResource",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-owner-external",
				Namespace: "default",
			},
		}
		g.Expect(cli.Create(ctx, owner)).To(Succeed())
		t.Cleanup(func() {
			_ = cli.Delete(ctx, owner)
		})

		// Reconcile to trigger watch setup
		ctx = reconciler.WithControllerName(ctx, "test-external-watches")
		_, err = p.Reconcile(ctx, owner)
		g.Expect(err).ToNot(HaveOccurred())

		// ConfigMap should NOT have a dynamic watch metric (it's external)
		cmValue := testutil.ToFloat64(watch.DynamicWatchedResourcesTotal.WithLabelValues(
			"test-external-watches",
			"v1",
			"ConfigMap",
		))
		g.Expect(cmValue).To(Equal(0.0), "ConfigMap should not be dynamically watched (external)")

		// Secret should have a dynamic watch metric (not external)
		secretValue := testutil.ToFloat64(watch.DynamicWatchedResourcesTotal.WithLabelValues(
			"test-external-watches",
			"v1",
			"Secret",
		))
		g.Expect(secretValue).To(Equal(1.0), "Secret should be dynamically watched")
	})

	t.Run("auto-watch with multiple external watches", func(t *testing.T) {
		g := NewWithT(t)

		ctrl, err := controller.NewUnmanaged("test-multi-external", controller.Options{
			Reconciler: &support.NoOpReconciler{},
		})
		g.Expect(err).ToNot(HaveOccurred())

		// Create pipeline with multiple external watches
		p, err := pipeline.NewPipeline(cli,
			pipeline.WithFieldOwner("test-controller"),
			pipeline.WithAutoWatch(ctrl, cacheObj,
				// Both ConfigMap and Deployment are watched externally
				watch.WithExternalWatches(gvks.ConfigMap, gvks.Deployment),
			),
			pipeline.WithActions(func(_ context.Context, _ *reconciler.Request, resp *reconciler.Response) error {
				// Provision ConfigMap (external), Deployment (external), and Service (should be auto-watched)
				cm := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-cm-multi",
						Namespace: "default",
					},
				}
				deploy := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-deploy-multi",
						Namespace: "default",
					},
					Spec: appsv1.DeploymentSpec{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "test-multi"},
						},
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{"app": "test-multi"},
							},
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{Name: "test", Image: "test:latest"},
								},
							},
						},
					},
				}
				svc := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-svc-multi",
						Namespace: "default",
					},
					Spec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{
							{Port: 80},
						},
					},
				}
				resp.Objects(cm, deploy, svc)

				return nil
			}),
		)
		g.Expect(err).ToNot(HaveOccurred())

		// Create owner object in cluster
		owner := &TestResource{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "test.example.com/v1",
				Kind:       "TestResource",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-owner-multi-external",
				Namespace: "default",
			},
		}
		g.Expect(cli.Create(ctx, owner)).To(Succeed())
		t.Cleanup(func() {
			_ = cli.Delete(ctx, owner)
		})

		// Reconcile to trigger watch setup
		ctx = reconciler.WithControllerName(ctx, "test-multi-external")
		_, err = p.Reconcile(ctx, owner)
		g.Expect(err).ToNot(HaveOccurred())

		// ConfigMap should NOT have a dynamic watch metric (external)
		cmValue := testutil.ToFloat64(watch.DynamicWatchedResourcesTotal.WithLabelValues(
			"test-multi-external",
			"v1",
			"ConfigMap",
		))
		g.Expect(cmValue).To(Equal(0.0), "ConfigMap should not be dynamically watched (external)")

		// Deployment should NOT have a dynamic watch metric (external)
		deployValue := testutil.ToFloat64(watch.DynamicWatchedResourcesTotal.WithLabelValues(
			"test-multi-external",
			"apps/v1",
			"Deployment",
		))
		g.Expect(deployValue).To(Equal(0.0), "Deployment should not be dynamically watched (external)")

		// Service should have a dynamic watch metric (not external)
		svcValue := testutil.ToFloat64(watch.DynamicWatchedResourcesTotal.WithLabelValues(
			"test-multi-external",
			"v1",
			"Service",
		))
		g.Expect(svcValue).To(Equal(1.0), "Service should be dynamically watched")
	})
}
