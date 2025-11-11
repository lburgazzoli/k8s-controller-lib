package cleanup

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	cleanupApi "github.com/lburgazzoli/k8s-controller-lib/examples/cleanup-controller/api/v1alpha1"
	"github.com/lburgazzoli/k8s-controller-lib/pkg/reconciler"
	"github.com/lburgazzoli/k8s-controller-lib/pkg/reconciler/pipeline"
	"github.com/lburgazzoli/k8s-controller-lib/pkg/resources"
)

const (
	fieldManager   = "cleanup-controller"
	controllerName = "cleanupApp"
)

type Cleanup struct {
	p *pipeline.Pipeline
}

func SetupWithManager(
	mgr ctrl.Manager,
) error {
	s := &Cleanup{}

	c, err := ctrl.NewControllerManagedBy(mgr).
		For(&cleanupApi.CleanupApp{}).
		Named(controllerName).
		Build(reconcile.AsReconciler(mgr.GetClient(), s))

	if err != nil {
		return fmt.Errorf("unable to create controller: %w", err)
	}

	p, err := pipeline.NewPipeline(
		mgr.GetClient(),
		pipeline.WithFieldOwner(fieldManager),
		pipeline.WithActions(s.manifests),
		pipeline.WithCleanupActions(s.cleanup),
		pipeline.WithAutoWatch(c, mgr.GetCache()),
	)
	if err != nil {
		return fmt.Errorf("unable to create pipeline: %w", err)
	}

	s.p = p

	return nil
}

func (c *Cleanup) Reconcile(
	ctx context.Context,
	obj *cleanupApi.CleanupApp,
) (reconcile.Result, error) {
	l := log.FromContext(ctx)
	l.Info("reconciling", "namespace", obj.Namespace, "name", obj.Name)

	return c.p.Reconcile(
		reconciler.WithControllerName(ctx, controllerName),
		obj,
	)
}

func (c *Cleanup) manifests(
	ctx context.Context,
	req *reconciler.Request,
	_ *reconciler.Response,
) error {
	l := log.FromContext(ctx)

	app, ok := req.Object.(*cleanupApi.CleanupApp)
	if !ok {
		return fmt.Errorf("unexpected object type: %T", req.Object)
	}

	// Create the ConfigMap directly using server-side apply
	// WITHOUT setting owner references to demonstrate cleanup actions
	cm := &corev1.ConfigMap{}
	cm.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))
	cm.Name = app.Spec.ConfigMapName
	cm.Namespace = app.Namespace
	cm.Labels = map[string]string{
		"app.kubernetes.io/name":       app.Name,
		"app.kubernetes.io/managed-by": fieldManager,
	}
	cm.Data = app.Spec.Data

	if err := resources.Apply(ctx, req.Client, cm, client.FieldOwner(fieldManager)); err != nil {
		return fmt.Errorf("failed to apply configmap: %w", err)
	}

	l.Info("applied configmap without owner reference", "name", app.Spec.ConfigMapName, "namespace", app.Namespace)

	return nil
}

func (c *Cleanup) cleanup(
	ctx context.Context,
	req *reconciler.Request,
) error {
	l := log.FromContext(ctx)
	l.Info("running cleanup", "namespace", req.Object.GetNamespace(), "name", req.Object.GetName())

	app, ok := req.Object.(*cleanupApi.CleanupApp)
	if !ok {
		return fmt.Errorf("unexpected object type: %T", req.Object)
	}

	cm := &corev1.ConfigMap{}
	cm.Name = app.Spec.ConfigMapName
	cm.Namespace = app.Namespace

	err := req.Client.Delete(ctx, cm)
	if err != nil {
		if errors.IsNotFound(err) {
			l.Info("configmap already deleted", "name", cm.Name, "namespace", cm.Namespace)
			return nil
		}
		return fmt.Errorf("failed to delete configmap: %w", err)
	}

	l.Info("deleted configmap during cleanup", "name", cm.Name, "namespace", cm.Namespace)

	return nil
}
