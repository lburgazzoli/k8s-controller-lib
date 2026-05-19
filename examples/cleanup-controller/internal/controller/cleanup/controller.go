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
	"github.com/lburgazzoli/k8s-controller-lib/pkg/reconciler/watch"
	"github.com/lburgazzoli/k8s-controller-lib/pkg/resources"
)

const (
	fieldManager   = "cleanup-controller"
	controllerName = "cleanupApp"
)

func SetupWithManager(
	mgr ctrl.Manager,
) error {
	// Create TypedPipeline with post-apply watcher - client, controller, and cache
	// must be provided to the watcher directly or injected by the user
	p := pipeline.NewTyped[*cleanupApi.CleanupApp](
		pipeline.WithFieldOwner(fieldManager),
		pipeline.WithActions(manifests),
		pipeline.WithCleanupActions(cleanup),
		pipeline.WithPostApply(watch.New().Watch),
	)

	// Create and complete the builder - TypedPipeline implements TypedReconciler
	b, err := builder.NewControllerBuilder[*cleanupApi.CleanupApp](
		mgr,
		builder.WithName(controllerName),
	)
	if err != nil {
		return fmt.Errorf("unable to create builder: %w", err)
	}

	return b.For(&cleanupApi.CleanupApp{}).Complete(p)
}

func manifests(
	ctx context.Context,
	req *reconciler.Request,
	_ *reconciler.Response,
) error {
	l := log.FromContext(ctx)
	l.Info("reconciling", "namespace", req.Object.GetNamespace(), "name", req.Object.GetName())

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

func cleanup(
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
