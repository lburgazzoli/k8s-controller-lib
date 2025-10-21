package simple

import (
	"context"
	"embed"
	"fmt"

	simpleApi "github.com/lburgazzoli/k8s-controller-lib/examples/simple-controller/api/v1alpha1"
	"github.com/lburgazzoli/k8s-controller-lib/pkg/reconciler"
	"github.com/lburgazzoli/k8s-controller-lib/pkg/reconciler/pipeline"
	"github.com/lburgazzoli/k8s-controller-lib/pkg/resources"
	"github.com/lburgazzoli/k8s-manifests-lib/pkg/engine"
	"github.com/lburgazzoli/k8s-manifests-lib/pkg/renderer/gotemplate"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	//go:embed templates/*.yaml.tmpl
	templates embed.FS
)

const (
	fieldManager = "simple-controller"
)

type Simple struct {
	e *engine.Engine
	p *pipeline.Pipeline
}

func SetupWithManager(
	mgr ctrl.Manager,
) error {
	r, err := gotemplate.New([]gotemplate.Source{{
		FS: templates,
	}})
	if err != nil {
		return fmt.Errorf("failed to create templates renderer: %v", err)
	}

	s := Simple{
		e: engine.New(
			engine.WithRenderer(r),
		),
	}

	p, err := pipeline.NewPipeline[*simpleApi.SimpleApp](
		mgr.GetClient(),
		pipeline.WithFieldOwner(fieldManager),
		pipeline.WithActions(s.manifests),
	)
	if err != nil {
		return fmt.Errorf("unable to create pipeline: %w", err)
	}

	_, err = ctrl.NewControllerManagedBy(mgr).
		For(&simpleApi.SimpleApp{}).
		Build(reconcile.AsReconciler(mgr.GetClient(), &s))

	if err != nil {
		return fmt.Errorf("unable to create controller: %w", err)
	}

	s.p = p

	return nil
}

func (s *Simple) Reconcile(ctx context.Context, obj *simpleApi.SimpleApp) (reconcile.Result, error) {
	return s.p.Reconcile(ctx, obj)
}

func (s *Simple) manifests(
	ctx context.Context,
	req *reconciler.Request,
	resp *reconciler.Response,
) error {

	u, err := resources.ToUnstructured(req.Client.Scheme(), req.Object)
	if err != nil {
		return err
	}

	objs, err := s.e.Render(
		ctx,
		engine.WithRenderTransformer(func(ctx context.Context, obj unstructured.Unstructured) (unstructured.Unstructured, error) {
			resources.SetLabel(&obj, "app.kubernetes.io/name", obj.GetName())
			resources.SetLabel(&obj, "app.kubernetes.io/instance", string(obj.GetUID()))
			return obj, nil
		}),
		engine.WithValues(u.Object),
	)

	for _, obj := range objs {
		resp.Objects(obj.DeepCopy())
	}

	return nil
}
