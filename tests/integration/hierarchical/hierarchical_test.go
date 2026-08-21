package hierarchical_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lburgazzoli/k3s-envtest/pkg/k3senv"
	crbuilder "sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/hierarchical"

	. "github.com/onsi/gomega"
)

func widgetGroupVersion() schema.GroupVersion {
	return schema.GroupVersion{Group: "hierarchical.test", Version: "v1"}
}

func TestHierarchicalCluster(t *testing.T) {
	g := NewWithT(t)
	ctx := t.Context()

	parentScheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(parentScheme)).To(Succeed())
	g.Expect(apiextensionsv1.AddToScheme(parentScheme)).To(Succeed())

	childScheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(childScheme)).To(Succeed())
	g.Expect(apiextensionsv1.AddToScheme(childScheme)).To(Succeed())
	g.Expect(addWidgetToScheme(childScheme)).To(Succeed())

	env, err := k3senv.New(k3senv.WithScheme(childScheme))
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(env.Start(ctx)).To(Succeed())
	t.Cleanup(func() { _ = env.Stop(context.Background()) })
	g.Expect(env.InstallCRD(ctx, widgetCRD())).To(Succeed())

	parent, err := manager.New(env.Config(), manager.Options{Scheme: parentScheme})
	g.Expect(err).ToNot(HaveOccurred())

	child, err := hierarchical.NewCluster(
		parent,
		hierarchical.WithName("widgets"),
		hierarchical.WithScheme(childScheme),
		hierarchical.WithNamespaces(corev1.NamespaceDefault),
	)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(parent.Add(child)).To(Succeed())

	childReconciles := atomic.Int32{}
	err = hierarchical.ControllerManagedBy(child).
		Named("hierarchical-widgets").
		For(&Widget{}).
		Watches(
			&corev1.ConfigMap{},
			handler.EnqueueRequestsFromMapFunc(func(_ context.Context, obj client.Object) []reconcile.Request {
				return []reconcile.Request{{NamespacedName: client.ObjectKeyFromObject(obj)}}
			}),
		).
		Complete(reconcile.Func(func(context.Context, reconcile.Request) (reconcile.Result, error) {
			childReconciles.Add(1)

			return reconcile.Result{}, nil
		}))
	g.Expect(err).ToNot(HaveOccurred())

	filteredChild, err := hierarchical.NewCluster(
		parent,
		hierarchical.WithName("filtered-configmaps"),
		hierarchical.WithScheme(parentScheme),
		hierarchical.WithNamespaces(corev1.NamespaceDefault),
		hierarchical.WithByObject(&corev1.ConfigMap{}, cache.ByObject{
			Label: labels.SelectorFromSet(labels.Set{"hierarchical": "enabled"}),
		}),
	)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(parent.Add(filteredChild)).To(Succeed())

	filteredReconciles := atomic.Int32{}
	err = hierarchical.ControllerManagedBy(filteredChild).
		Named("hierarchical-filtered-configmaps").
		For(&corev1.ConfigMap{}).
		Complete(reconcile.Func(func(context.Context, reconcile.Request) (reconcile.Result, error) {
			filteredReconciles.Add(1)

			return reconcile.Result{}, nil
		}))
	g.Expect(err).ToNot(HaveOccurred())

	parentReconciles := atomic.Int32{}
	err = crbuilder.ControllerManagedBy(parent).
		Named("hierarchical-parent-secrets").
		For(&corev1.Secret{}).
		Complete(reconcile.Func(func(context.Context, reconcile.Request) (reconcile.Result, error) {
			parentReconciles.Add(1)

			return reconcile.Result{}, nil
		}))
	g.Expect(err).ToNot(HaveOccurred())

	managerCtx, cancelManager := context.WithCancel(ctx)
	managerResult := make(chan error, 1)
	go func() { managerResult <- parent.Start(managerCtx) }()
	t.Cleanup(cancelManager)

	widget := &Widget{ObjectMeta: metav1.ObjectMeta{Name: "local", Namespace: corev1.NamespaceDefault}}
	g.Expect(child.GetClient().Create(ctx, widget)).To(Succeed())
	g.Eventually(childReconciles.Load).
		WithContext(ctx).WithTimeout(10 * time.Second).
		Should(BeNumerically(">=", 1))

	shared := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "shared", Namespace: corev1.NamespaceDefault}}
	beforeShared := childReconciles.Load()
	g.Expect(parent.GetClient().Create(ctx, shared)).To(Succeed())
	g.Eventually(childReconciles.Load).
		WithContext(ctx).WithTimeout(10 * time.Second).
		Should(BeNumerically(">", beforeShared))

	filtered := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Name: "filtered", Namespace: corev1.NamespaceDefault, Labels: map[string]string{"hierarchical": "enabled"},
	}}
	g.Expect(parent.GetClient().Create(ctx, filtered)).To(Succeed())
	g.Eventually(filteredReconciles.Load).
		WithContext(ctx).WithTimeout(10 * time.Second).
		Should(BeNumerically(">=", 1))

	filteredBeforeIgnored := filteredReconciles.Load()
	unfiltered := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "unfiltered", Namespace: corev1.NamespaceDefault}}
	g.Expect(parent.GetClient().Create(ctx, unfiltered)).To(Succeed())
	wrongNamespace := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Name: "wrong-namespace", Namespace: metav1.NamespaceSystem, Labels: map[string]string{"hierarchical": "enabled"},
	}}
	g.Expect(parent.GetClient().Create(ctx, wrongNamespace)).To(Succeed())
	g.Consistently(filteredReconciles.Load).
		WithContext(ctx).
		Within(500 * time.Millisecond).
		Should(Equal(filteredBeforeIgnored))

	stopCtx, cancelStop := context.WithTimeout(ctx, 10*time.Second)
	g.Expect(child.Stop(stopCtx)).To(Succeed())
	cancelStop()
	childBeforeStop := childReconciles.Load()
	afterStop := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "after-stop", Namespace: corev1.NamespaceDefault}}
	g.Expect(parent.GetClient().Create(ctx, afterStop)).To(Succeed())
	g.Consistently(childReconciles.Load).
		WithContext(ctx).
		Within(500 * time.Millisecond).
		Should(Equal(childBeforeStop))

	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "parent-alive", Namespace: corev1.NamespaceDefault}}
	g.Expect(parent.GetClient().Create(ctx, secret)).To(Succeed())
	g.Eventually(parentReconciles.Load).
		WithContext(ctx).WithTimeout(10 * time.Second).
		Should(BeNumerically(">=", 1))

	err = hierarchical.ControllerManagedBy(child).
		Named("hierarchical-after-stop").
		For(&corev1.Pod{}).
		Complete(reconcile.Func(func(context.Context, reconcile.Request) (reconcile.Result, error) {
			return reconcile.Result{}, nil
		}))
	g.Expect(errors.Is(err, hierarchical.ErrClusterStopped)).To(BeTrue())

	stopCtx, cancelStop = context.WithTimeout(ctx, 10*time.Second)
	g.Expect(filteredChild.Stop(stopCtx)).To(Succeed())
	cancelStop()
	cancelManager()
	g.Eventually(managerResult).
		WithContext(ctx).WithTimeout(10 * time.Second).
		Should(Receive(Succeed()))
}

type Widget struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
}

func (w *Widget) DeepCopyObject() runtime.Object {
	if w == nil {
		return nil
	}
	out := new(Widget)
	*out = *w
	w.DeepCopyInto(&out.ObjectMeta)

	return out
}

type WidgetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Widget `json:"items"`
}

func (w *WidgetList) DeepCopyObject() runtime.Object {
	if w == nil {
		return nil
	}
	out := new(WidgetList)
	*out = *w
	w.DeepCopyInto(&out.ListMeta)
	if w.Items != nil {
		out.Items = make([]Widget, len(w.Items))
		for i := range w.Items {
			out.Items[i] = w.Items[i]
			w.Items[i].DeepCopyInto(&out.Items[i].ObjectMeta)
		}
	}

	return out
}

func addWidgetToScheme(scheme *runtime.Scheme) error {
	widgetGV := widgetGroupVersion()
	scheme.AddKnownTypes(widgetGV, &Widget{}, &WidgetList{})
	metav1.AddToGroupVersion(scheme, widgetGV)

	return nil
}

func widgetCRD() *apiextensionsv1.CustomResourceDefinition {
	widgetGV := widgetGroupVersion()

	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "widgets.hierarchical.test"},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: widgetGV.Group,
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Kind: "Widget", ListKind: "WidgetList", Plural: "widgets", Singular: "widget",
			},
			Scope: apiextensionsv1.NamespaceScoped,
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{{
				Name: widgetGV.Version, Served: true, Storage: true,
				Schema: &apiextensionsv1.CustomResourceValidation{OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
					Type: "object", XPreserveUnknownFields: new(true),
				}},
			}},
		},
	}
}
