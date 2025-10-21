package pipeline

import (
	"context"
	"errors"
	"fmt"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/conditions"
	"github.com/lburgazzoli/k8s-controller-lib/pkg/reconciler"
	"github.com/lburgazzoli/k8s-controller-lib/pkg/resources"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

// Pipeline orchestrates sequential execution of actions with error accumulation.
// Actions execute in registration order. Cleanup actions execute in reverse order.
// Pipeline implements reconcile.Reconciler interface.
type Pipeline struct {
	client client.Client
	opts   Options
}

const (
	defaultFinalizer                = "reconciler.k8s-controller-lib/finalizer"
	conditionTypeProvisioningFailed = "ProvisioningFailed"
)

// NewPipeline creates a new Pipeline configured with the given client and options.
// Returns an error if client is nil or field owner is not configured.
// The type parameter T specifies the concrete type of ManagedObject this pipeline reconciles.
func NewPipeline(c client.Client, opts ...Option) (*Pipeline, error) {
	if c == nil {
		return nil, errors.New("client is required")
	}

	options := &Options{}
	options.ApplyOptions(opts)

	if options.FieldOwner == "" {
		return nil, errors.New("field owner is required")
	}

	// Set default finalizer if cleanup actions are present but no finalizer specified
	if options.Finalizer == "" && len(options.CleanupActions) > 0 {
		options.Finalizer = defaultFinalizer
	}

	p := Pipeline{
		client: c,
		opts:   *options,
	}

	return &p, nil
}

// Reconcile implements reconcile.Reconciler interface.
// It fetches the object from the cluster and delegates to run.
func (p *Pipeline) Reconcile(
	ctx context.Context,
	obj reconciler.ManagedObject,
) (reconcile.Result, error) {
	// Execute reconciliation logic
	resp, err := p.run(ctx, &reconciler.Request{
		Client: p.client,
		Object: obj,
	})

	// Convert Response to Result
	result := reconcile.Result{}
	if resp != nil {
		result.RequeueAfter = resp.ShouldRequeue()
	}

	return result, err
}

// run orchestrates the reconciliation loop with automatic finalizer management.
// It handles finalizer addition, cleanup on deletion, and finalizer removal.
// Use this method when you have the object already fetched.
// Use Reconcile when implementing reconcile.Reconciler interface.
func (p *Pipeline) run(
	ctx context.Context,
	req *reconciler.Request,
) (*reconciler.Response, error) {
	resp := reconciler.NewResponse()
	obj := req.Object

	// Handle deletion case
	if !obj.GetDeletionTimestamp().IsZero() {
		if err := p.cleanup(ctx, req); err != nil {
			return resp, err
		}

		return resp, nil
	}

	// Add finalizer if configured and missing
	if p.opts.Finalizer != "" && controllerutil.AddFinalizer(obj, p.opts.Finalizer) {
		if err := p.client.Update(ctx, obj); err != nil {
			return resp, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	// Execute actions and provision objects
	execErr := p.execute(ctx, req, resp)

	// Update status with execution result
	if err := p.updateStatus(ctx, req, execErr); err != nil {
		return resp, err
	}

	// Return execution error (if any) after status update
	return resp, execErr
}

// execute runs all actions sequentially, provisions objects, and accumulates errors.
// If a StopError is encountered, execution halts immediately without provisioning.
// Returns an aggregated error containing all failures.
func (p *Pipeline) execute(
	ctx context.Context,
	req *reconciler.Request,
	resp *reconciler.Response,
) error {
	var actionErrs []error

	// Execute actions sequentially
	for _, action := range p.opts.Actions {
		if err := action(ctx, req, resp); err != nil {
			if IsStopError(err) {
				// Stop immediately on StopError without provisioning
				return err
			}
			actionErrs = append(actionErrs, err)
		}
	}

	// Provision objects from response (attempt even if actions failed)
	var provisionErrs []error
	for _, obj := range resp.GetObjects() {
		if err := controllerutil.SetControllerReference(req.Object, obj, p.client.Scheme()); err != nil {
			return fmt.Errorf("unable to set controller reference to %s: %w", resources.FormatObjectReference(obj), err)
		}
		if err := resources.Apply(ctx, p.client, obj, client.FieldOwner(p.opts.FieldOwner)); err != nil {
			return fmt.Errorf("unable to apply %s: %w", resources.FormatObjectReference(obj), err)
		}
	}

	// Combine action and provisioning errors
	return utilerrors.NewAggregate(append(actionErrs, provisionErrs...))
}

// cleanup handles object deletion by running cleanup actions and removing the finalizer.
// Returns early if no finalizer is configured or the finalizer is not present on the object.
func (p *Pipeline) cleanup(
	ctx context.Context,
	req *reconciler.Request,
) error {
	if p.opts.Finalizer == "" || !controllerutil.ContainsFinalizer(req.Object, p.opts.Finalizer) {
		return nil
	}

	var errs []error

	// Execute cleanup actions in reverse order
	for i := len(p.opts.CleanupActions) - 1; i >= 0; i-- {
		if err := p.opts.CleanupActions[i](ctx, req); err != nil {
			errs = append(errs, err)
		}
	}

	// Check for cleanup errors before removing finalizer
	if err := utilerrors.NewAggregate(errs); err != nil {
		return fmt.Errorf("cleanup failed: %w", err)
	}

	// Remove finalizer
	if !controllerutil.RemoveFinalizer(req.Object, p.opts.Finalizer) {
		return nil
	}

	if err := p.client.Update(ctx, req.Object); err != nil {
		return fmt.Errorf("failed to remove finalizer: %w", err)
	}

	return nil
}

// updateStatus updates the object's status using server-side apply.
// It sets the ObservedGeneration and ProvisioningFailed condition based on execution result.
func (p *Pipeline) updateStatus(
	ctx context.Context,
	req *reconciler.Request,
	execErr error,
) error {
	// ManagedObject implements status.Accessor directly
	st := req.Object.GetStatus()
	if st == nil {
		return nil
	}

	// Set ProvisioningFailed condition based on execution result
	if execErr != nil {
		conditions.MarkFalse(
			st,
			conditionTypeProvisioningFailed,
			conditions.WithReason("ReconciliationFailed"),
			conditions.WithMessage(execErr.Error()),
			conditions.WithObservedGeneration(st.ObservedGeneration),
		)
	} else {
		// Update observed generation
		st.ObservedGeneration = req.Object.GetGeneration()

		conditions.MarkTrue(
			st,
			conditionTypeProvisioningFailed,
			conditions.WithReason("ReconciliationSucceeded"),
			conditions.WithObservedGeneration(st.ObservedGeneration),
		)
	}

	if err := resources.ApplyStatus(ctx, p.client, req.Object, client.FieldOwner(p.opts.FieldOwner)); err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	return nil
}
