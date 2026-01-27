package pipeline

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/conditions"
	"github.com/lburgazzoli/k8s-controller-lib/pkg/reconciler"
	"github.com/lburgazzoli/k8s-controller-lib/pkg/reconciler/watch"
	"github.com/lburgazzoli/k8s-controller-lib/pkg/resources"
)

// Pipeline orchestrates sequential execution of actions with error accumulation.
// Actions execute in registration order. Cleanup actions execute in reverse order.
// Pipeline implements reconcile.Reconciler interface and the aware interfaces
// (ClientAware, CacheAware, ControllerAware, ExternalWatchesAware) for Builder integration.
type Pipeline struct {
	opts    Options
	watcher *watch.Watcher
	mu      sync.Mutex
}

const (
	// DefaultFinalizer is the default finalizer name used by the pipeline when cleanup actions are configured.
	DefaultFinalizer = "reconciler.k8s-controller-lib/finalizer"
	// ConditionTypeProvisioningSucceeded is the condition type used to track overall provisioning status.
	ConditionTypeProvisioningSucceeded = "ProvisioningSucceeded"

	// AnnotationOwnerGroup is the annotation key for owner group.
	AnnotationOwnerGroup = "controller-lib.k8s.io/owner-group"
	// AnnotationOwnerVersion is the annotation key for owner version.
	AnnotationOwnerVersion = "controller-lib.k8s.io/owner-version"
	// AnnotationOwnerKind is the annotation key for owner kind.
	AnnotationOwnerKind = "controller-lib.k8s.io/owner-kind"
	// AnnotationOwnerName is the annotation key for owner name.
	AnnotationOwnerName = "controller-lib.k8s.io/owner-name"
	// AnnotationOwnerNamespace is the annotation key for owner namespace.
	AnnotationOwnerNamespace = "controller-lib.k8s.io/owner-namespace"

	// LabelOwnerName is the label key for owner name tracking and watch mapping.
	LabelOwnerName = "controller-lib.k8s.io/owner-name"
	// LabelOwnerNamespace is the label key for owner namespace tracking and watch mapping.
	LabelOwnerNamespace = "controller-lib.k8s.io/owner-namespace"
)

// NewPipeline creates a new Pipeline configured with the given options.
// No parameters are required at construction - dependencies can be provided via options
// (WithClient, WithCache, WithController) or injected later via aware interfaces.
//
// Field owner can be specified via WithFieldOwner or defaults to controller name from context.
//
// Example with Builder (dependencies injected automatically):
//
//	p := pipeline.NewPipeline(
//	    pipeline.WithFieldOwner("my-controller"),
//	    pipeline.WithAutoWatch(),
//	    pipeline.WithActions(myAction),
//	)
//	b.For(&v1alpha1.MyApp{}).Complete(p)
//
// Example standalone (dependencies provided explicitly):
//
//	p := pipeline.NewPipeline(
//	    pipeline.WithClient(client),
//	    pipeline.WithController(ctrl),
//	    pipeline.WithCache(cache),
//	    pipeline.WithFieldOwner("my-controller"),
//	    pipeline.WithAutoWatch(),
//	    pipeline.WithActions(myAction),
//	)
func NewPipeline(opts ...Option) *Pipeline {
	options := &Options{
		Ownership: true, // Enable ownership by default
	}
	options.ApplyOptions(opts)

	// Set default finalizer if cleanup actions are present but no finalizer specified
	if options.Finalizer == "" && len(options.CleanupActions) > 0 {
		options.Finalizer = DefaultFinalizer
	}

	return &Pipeline{opts: *options}
}

// SetClient implements reconciler.ClientAware.
// Called by Builder.Complete() to inject the client.
func (p *Pipeline) SetClient(c client.Client) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.opts.Client = c
}

// SetCache implements reconciler.CacheAware.
// Called by Builder.Complete() to inject the cache.
func (p *Pipeline) SetCache(c cache.Cache) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.opts.Cache = c
}

// SetController implements reconciler.ControllerAware.
// Called by Builder.Complete() to inject the controller.
func (p *Pipeline) SetController(ctrl controller.Controller) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.opts.Controller = ctrl
}

// SetExternalWatches implements reconciler.ExternalWatchesAware.
// Called by Builder.Complete() to inform the pipeline about GVKs already watched by Builder.
// These GVKs will be added as disabled configs to prevent redundant watch registration.
func (p *Pipeline) SetExternalWatches(gvks []schema.GroupVersionKind) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, gvk := range gvks {
		p.opts.WatchConfigs = append(p.opts.WatchConfigs, watch.Config{GVK: gvk, Disabled: true})
	}
}

// getClient returns the client, panicking if not set.
// The client must be set via WithClient option or SetClient before Reconcile is called.
func (p *Pipeline) getClient() client.Client {
	if p.opts.Client == nil {
		panic("pipeline: client not set - use WithClient option or ensure Builder injects it")
	}

	return p.opts.Client
}

// getWatcher returns the watcher, lazily creating it if auto-watch is enabled
// and all required dependencies are available.
func (p *Pipeline) getWatcher() *watch.Watcher {
	if !p.opts.AutoWatch {
		return nil
	}

	// Check required dependencies
	if p.opts.Cache == nil || p.opts.Controller == nil || p.opts.Client == nil {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.watcher == nil {
		p.watcher = watch.New(
			p.opts.Controller,
			p.opts.Cache,
			p.opts.Client,
			watch.WithConfigs(p.opts.WatchConfigs...),
		)
	}

	return p.watcher
}

// Reconcile implements reconcile.Reconciler interface.
// It fetches the object from the cluster and delegates to run.
func (p *Pipeline) Reconcile(
	ctx context.Context,
	obj reconciler.ManagedObject,
) (reconcile.Result, error) {
	cli := p.getClient()

	// Execute reconciliation logic
	resp, err := p.run(ctx, &reconciler.Request{
		Client: cli,
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
	cli := p.getClient()

	// Handle deletion case
	if !obj.GetDeletionTimestamp().IsZero() {
		if err := p.cleanup(ctx, req); err != nil {
			return resp, err
		}

		return resp, nil
	}

	// Add finalizer if configured and missing
	if p.opts.Finalizer != "" && controllerutil.AddFinalizer(obj, p.opts.Finalizer) {
		if err := cli.Update(ctx, obj); err != nil {
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

	// Determine field owner: use configured value or controller name from context
	fieldOwner := p.opts.FieldOwner
	if fieldOwner == "" {
		var ok bool
		fieldOwner, ok = reconciler.ControllerNameFromContext(ctx)
		if !ok {
			return errors.New("field owner not configured and controller name not in context")
		}
	}

	// Process objects WITH ownership (based on pipeline-level setting)
	if err := p.processObjects(ctx, req.Object, resp.GetObjects(), fieldOwner, p.opts.Ownership); err != nil {
		return err
	}

	// Process objects WITHOUT ownership (per-object override)
	if err := p.processObjects(ctx, req.Object, resp.GetObjectsWithoutOwnership(), fieldOwner, false); err != nil {
		return err
	}

	// Combine all objects for auto-watch
	allObjects := append(resp.GetObjects(), resp.GetObjectsWithoutOwnership()...)

	// Setup watches for all provisioned objects if auto-watch is configured
	if watcher := p.getWatcher(); watcher != nil {
		if err := watcher.Watch(ctx, req.Object, allObjects); err != nil {
			return fmt.Errorf("unable to setup watches: %w", err)
		}
	}

	return utilerrors.NewAggregate(actionErrs)
}

// processObjects applies objects with optional ownership.
// If withOwnership is true, sets OwnerReferences; otherwise, adds tracking annotations/labels.
//
//nolint:revive // withOwnership controls ownership vs label-based tracking
func (p *Pipeline) processObjects(
	ctx context.Context,
	owner client.Object,
	objects []client.Object,
	fieldOwner string,
	withOwnership bool,
) error {
	cli := p.getClient()

	for _, obj := range objects {
		if withOwnership {
			if err := controllerutil.SetControllerReference(owner, obj, cli.Scheme()); err != nil {
				return fmt.Errorf("unable to set controller reference to %s: %w", resources.FormatObjectReference(obj), err)
			}
		} else {
			// No ownership but potentially add tracking
			if err := p.addOwnerTracking(owner, obj); err != nil {
				return fmt.Errorf("unable to add owner tracking to %s: %w", resources.FormatObjectReference(obj), err)
			}
		}

		if err := resources.Apply(ctx, cli, obj, client.FieldOwner(fieldOwner)); err != nil {
			return fmt.Errorf("unable to apply %s: %w", resources.FormatObjectReference(obj), err)
		}
	}

	return nil
}

// addOwnerTracking adds annotations and/or labels to obj based on pipeline configuration.
// This is called for objects that do not have OwnerReferences set.
func (p *Pipeline) addOwnerTracking(
	owner client.Object,
	obj client.Object,
) error {
	if !p.opts.AnnotateNonOwnedObjects && !p.opts.LabelNonOwnedObjects {
		return nil
	}

	cli := p.getClient()

	// Get owner GVK
	gvk, err := apiutil.GVKForObject(owner, cli.Scheme())
	if err != nil {
		return fmt.Errorf("failed to get GVK for owner: %w", err)
	}

	// Add annotations if configured
	if p.opts.AnnotateNonOwnedObjects {
		resources.SetAnnotation(obj, AnnotationOwnerGroup, gvk.Group)
		resources.SetAnnotation(obj, AnnotationOwnerVersion, gvk.Version)
		resources.SetAnnotation(obj, AnnotationOwnerKind, gvk.Kind)
		resources.SetAnnotation(obj, AnnotationOwnerName, owner.GetName())
		if owner.GetNamespace() != "" {
			resources.SetAnnotation(obj, AnnotationOwnerNamespace, owner.GetNamespace())
		}
	}

	// Add labels if configured
	if p.opts.LabelNonOwnedObjects {
		resources.SetLabel(obj, LabelOwnerName, owner.GetName())
		if owner.GetNamespace() != "" {
			resources.SetLabel(obj, LabelOwnerNamespace, owner.GetNamespace())
		}
	}

	return nil
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

	cli := p.getClient()
	if err := cli.Update(ctx, req.Object); err != nil {
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

	cli := p.getClient()

	// Determine field owner (same logic as execute)
	fieldOwner := p.opts.FieldOwner
	if fieldOwner == "" {
		name, ok := reconciler.ControllerNameFromContext(ctx) //nolint:staticcheck // false positive: name is used in line 328
		if !ok {
			// Skip status update if no field owner available
			return nil
		}
		fieldOwner = name //nolint:ineffassign,staticcheck,wastedassign // false positive: fieldOwner is used below
	}

	// Set ProvisioningFailed condition based on execution result
	if execErr != nil {
		// Update observed generation even on failure to track which generation was processed
		st.ObservedGeneration = req.Object.GetGeneration()

		conditions.MarkFalse(
			st,
			ConditionTypeProvisioningSucceeded,
			conditions.WithReason("ReconciliationFailed"),
			conditions.WithMessage(execErr.Error()),
			conditions.WithObservedGeneration(st.ObservedGeneration),
		)
	} else {
		// Update observed generation
		st.ObservedGeneration = req.Object.GetGeneration()

		conditions.MarkTrue(
			st,
			ConditionTypeProvisioningSucceeded,
			conditions.WithReason("ReconciliationSucceeded"),
			conditions.WithObservedGeneration(st.ObservedGeneration),
		)
	}

	// Use regular status update instead of server-side apply
	// SSA for status subresources has compatibility issues with fake clients in controller-runtime v0.23.0+
	//
	// Root cause: The fake client's new versioned_tracker enforces resource version checks even for SSA patches.
	// When the main object is updated (e.g., finalizer addition), the ResourceVersion increments, but the
	// req.Object reference in memory retains the old ResourceVersion. Even though ApplyStatus removes the
	// resourceVersion field from the patch data, the versioned tracker checks the object's RV before applying.
	//
	// Solution: Use regular Status().Update() which works reliably with both fake and real clients.
	// Future: Consider migrating to the new Client.Apply() method with runtime.ApplyConfiguration.

	// Ensure TypeMeta is set before status update (required for real K8s clusters)
	// When objects are fetched from the API server, TypeMeta is often cleared
	if err := resources.EnsureGroupVersionKind(cli.Scheme(), req.Object); err != nil {
		return fmt.Errorf("failed to ensure GVK on object: %w", err)
	}

	if err := cli.Status().Update(ctx, req.Object); err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	return nil
}
