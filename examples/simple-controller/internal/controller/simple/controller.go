package simple

import (
	"context"
	"embed"
	"fmt"

	simpleApi "github.com/lburgazzoli/k8s-controller-lib/examples/simple-controller/api/v1alpha1"
	"github.com/lburgazzoli/k8s-controller-lib/pkg/reconciler"
	"github.com/lburgazzoli/k8s-controller-lib/pkg/reconciler/pipeline"
	"github.com/lburgazzoli/k8s-controller-lib/pkg/resources"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/k8s-manifest-kit/engine/pkg"
	"github.com/k8s-manifest-kit/renderer-gotemplate/pkg"
)

var (
	//go:embed templates/*.yaml.tmpl
	templates embed.FS
)

const (
	fieldManager   = "simple-controller"
	controllerName = "simpleApp"
)

type Simple struct {
	e *engine.Engine
	p *pipeline.Pipeline
}

func SetupWithManager(
	mgr ctrl.Manager,
) error {
	e, err := gotemplate.NewEngine(gotemplate.Source{
		FS:   templates,
		Path: "templates/*.yaml.tmpl",
	})
	if err != nil {
		return fmt.Errorf("failed to create templates renderer: %v", err)
	}

	s := Simple{}

	c, err := ctrl.NewControllerManagedBy(mgr).
		For(&simpleApi.SimpleApp{}).
		Named(controllerName).
		Build(reconcile.AsReconciler(mgr.GetClient(), &s))

	if err != nil {
		return fmt.Errorf("unable to create controller: %w", err)
	}

	p, err := pipeline.NewPipeline(
		mgr.GetClient(),
		pipeline.WithFieldOwner(fieldManager),
		pipeline.WithActions(s.manifests),
		pipeline.WithAutoWatch(c, mgr.GetCache()),
	)
	if err != nil {
		return fmt.Errorf("unable to create pipeline: %w", err)
	}

	s.p = p
	s.e = e

	return nil
}

func (s *Simple) Reconcile(ctx context.Context, obj *simpleApi.SimpleApp) (reconcile.Result, error) {
	l := log.FromContext(ctx)
	l.Info("reconciling", "namespace", obj.Namespace, "name", obj.Name)

	return s.p.Reconcile(
		// the pipeline and metrics expect the reconciler name to be passes
		// through the contex, no ideal, but it simplifies the configuration
		// a lot
		reconciler.WithControllerName(ctx, controllerName),
		obj,
	)
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
			return obj, nil
		}),
		engine.WithValues(u.Object),
	)

	if err != nil {
		return fmt.Errorf("failed to render manifests: %w", err)
	}

	for _, obj := range objs {
		resp.Objects(obj.DeepCopy())
	}

	return nil
}
