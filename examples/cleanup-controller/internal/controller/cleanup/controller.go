package cleanup

import (
	"context"
	"fmt"
	"maps"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/builder"

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
	c := &Cleanup{}

	// Create controller using the library builder
	b, err := builder.NewControllerBuilder[*cleanupApi.CleanupApp](
		mgr,
		builder.WithName(controllerName),
	)
	if err != nil {
		return fmt.Errorf("unable to create builder: %w", err)
	}

	// Complete the builder - this creates the controller
	if err := b.For(&cleanupApi.CleanupApp{}).Complete(c); err != nil {
		return fmt.Errorf("unable to create controller: %w", err)
	}

	// Create pipeline with auto-watch using the builder's controller
	p, err := pipeline.NewPipeline(
		mgr.GetClient(),
		pipeline.WithFieldOwner(fieldManager),
		pipeline.WithActions(c.manifests),
		pipeline.WithCleanupActions(c.cleanup),
		pipeline.WithAutoWatch(b.GetController(), mgr.GetCache()),
	)
	if err != nil {
		return fmt.Errorf("unable to create pipeline: %w", err)
	}

	c.p = p

	return nil
}

// Reconcile implements reconciler.TypedReconciler[*cleanupApi.CleanupApp].
func (c *Cleanup) Reconcile(
	ctx context.Context,
	req *reconciler.TypedRequest[*cleanupApi.CleanupApp],
) (*reconciler.Response, error) {
	l := log.FromContext(ctx)
	l.Info("reconciling", "namespace", req.Object.Namespace, "name", req.Object.Name)

	// The builder automatically injects the controller name into the context.
	// Use the pipeline to reconcile and convert the result to Response.
	result, err := c.p.Reconcile(ctx, req.Object)

	// Convert reconcile.Result to reconciler.Response
	resp := reconciler.NewResponse()
	if result.RequeueAfter > 0 {
		resp.Requeue(result.RequeueAfter)
	}

	return resp, err
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
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Spec.ConfigMapName,
			Namespace: app.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       app.Name,
				"app.kubernetes.io/managed-by": "unmanaged",
			},
		},
		Data: maps.Clone(app.Spec.Data),
	}

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

	if err := req.Client.Delete(ctx, cm); err != nil {
		if errors.IsNotFound(err) {
			l.Info("configmap already deleted", "name", cm.Name, "namespace", cm.Namespace)
			return nil
		}
		return fmt.Errorf("failed to delete configmap: %w", err)
	}

	l.Info("deleted configmap during cleanup", "name", cm.Name, "namespace", cm.Namespace)

	return nil
}
