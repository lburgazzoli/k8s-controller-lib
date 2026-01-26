package simple

import (
	"context"
	"embed"
	"fmt"

	engine "github.com/k8s-manifest-kit/engine/pkg"
	gotemplate "github.com/k8s-manifest-kit/renderer-gotemplate/pkg"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/builder"

	simpleApi "github.com/lburgazzoli/k8s-controller-lib/examples/simple-controller/api/v1alpha1"
	"github.com/lburgazzoli/k8s-controller-lib/pkg/reconciler"
	"github.com/lburgazzoli/k8s-controller-lib/pkg/reconciler/pipeline"
	"github.com/lburgazzoli/k8s-controller-lib/pkg/resources"
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

	s := &Simple{
		e: e,
	}

	// Create controller using the library builder
	b, err := builder.NewControllerBuilder[*simpleApi.SimpleApp](
		mgr,
		builder.WithName(controllerName),
	)
	if err != nil {
		return fmt.Errorf("unable to create builder: %w", err)
	}

	// Complete the builder - this creates the controller
	if err := b.For(&simpleApi.SimpleApp{}).Complete(s); err != nil {
		return fmt.Errorf("unable to create controller: %w", err)
	}

	// Create pipeline with auto-watch using the builder's controller
	p, err := pipeline.NewPipeline(
		mgr.GetClient(),
		pipeline.WithFieldOwner(fieldManager),
		pipeline.WithActions(s.manifests),
		pipeline.WithAutoWatch(b.GetController(), mgr.GetCache()),
	)
	if err != nil {
		return fmt.Errorf("unable to create pipeline: %w", err)
	}

	s.p = p

	return nil
}

// Reconcile implements reconciler.TypedReconciler[*simpleApi.SimpleApp].
func (s *Simple) Reconcile(
	ctx context.Context,
	req *reconciler.TypedRequest[*simpleApi.SimpleApp],
) (*reconciler.Response, error) {
	l := log.FromContext(ctx)
	l.Info("reconciling", "namespace", req.Object.Namespace, "name", req.Object.Name)

	// The builder automatically injects the controller name into the context.
	// Use the pipeline to reconcile and convert the result to Response.
	result, err := s.p.Reconcile(ctx, req.Object)

	// Convert reconcile.Result to reconciler.Response
	resp := reconciler.NewResponse()
	if result.RequeueAfter > 0 {
		resp.Requeue(result.RequeueAfter)
	}

	return resp, err
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
