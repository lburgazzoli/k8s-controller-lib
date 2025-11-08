package watch

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/predicates"
	"github.com/lburgazzoli/k8s-controller-lib/pkg/reconciler"
	"github.com/lburgazzoli/k8s-controller-lib/pkg/resources"
)

type (
	// Predicate is an alias for predicate.Predicate used for filtering watch events.
	Predicate = predicate.Predicate
	// Handler is an alias for handler.EventHandler used for processing watch events.
	Handler = handler.EventHandler
)

// State tracks watch configuration and registration status for a GVK.
type State struct {
	Config  Config
	Watched bool
}

// Watcher manages automatic watch setup for Kubernetes objects.
// It tracks which GVKs have been watched and applies custom or default
// predicates and handlers when setting up new watches.
type Watcher struct {
	controller controller.Controller
	cache      cache.Cache
	client     client.Client
	states     map[schema.GroupVersionKind]*State
}

// New creates a new Watcher with the specified controller, cache, and client.
// Optional Option parameters configure watch behavior.
//
// The controller name for metrics labeling is retrieved from the context passed to Watch().
//
// Example:
//
//	watcher := watch.New(ctrl, cache, client,
//	    watch.WithConfigs(
//	        watch.For(deploymentGVK, watch.WithPredicates(pred1, pred2)),
//	        watch.For(serviceGVK, watch.Disabled()),
//	    ),
//	)
func New(
	ctrl controller.Controller,
	c cache.Cache,
	cli client.Client,
	opts ...Option,
) *Watcher {
	options := &Options{}
	options.ApplyOptions(opts)

	w := &Watcher{
		controller: ctrl,
		cache:      c,
		client:     cli,
		states:     make(map[schema.GroupVersionKind]*State),
	}

	for _, cfg := range options.Configs {
		w.states[cfg.GVK] = &State{
			Config:  cfg,
			Watched: false,
		}
	}

	return w
}

// Watch sets up watches for the provided objects if not already watched.
// Uses owner references to map events from watched objects back to the owner.
//
// The context should contain the controller name for metrics labeling, injected via
// reconciler.WithControllerName(). If not present, "unknown" will be used as fallback.
//
// For each object:
// - Extracts the GVK
// - Checks if already watched
// - Applies custom or default predicate and handler
// - Registers the watch with the controller.
func (w *Watcher) Watch(
	ctx context.Context,
	ownerObj client.Object,
	objects []client.Object,
) error {
	for _, obj := range objects {
		gvk, err := apiutil.GVKForObject(obj, w.client.Scheme())
		if err != nil {
			return fmt.Errorf("unable to get GVK for %T: %w", obj, err)
		}

		state := w.state(gvk)
		if state.Watched {
			continue
		}
		if state.Config.Disabled {
			return nil
		}

		if err := w.setupWatch(ctx, gvk, obj, ownerObj, state); err != nil {
			return fmt.Errorf("unable to setup watch for %s: %w", gvk, err)
		}

		state.Watched = true
	}

	return nil
}

// setupWatch registers a watch for the given GVK using configured or default settings.
// Returns nil without registering if the watch is disabled.
func (w *Watcher) setupWatch(
	ctx context.Context,
	gvk schema.GroupVersionKind,
	obj client.Object,
	ownerObj client.Object,
	state *State,
) error {
	// If Partial is enabled, convert to PartialObjectMetadata
	watchObj := obj
	if state.Config.Partial {
		partial, err := resources.ToPartialObjectMetadata(w.client.Scheme(), obj)
		if err != nil {
			return fmt.Errorf("failed to convert to PartialObjectMetadata: %w", err)
		}
		watchObj = partial
	}

	// Setup predicates
	// When using PartialObjectMetadata, default predicates are NOT applied automatically
	preds := state.Config.Predicates
	if len(preds) == 0 && !resources.IsPartialObjectMetadata(watchObj) {
		preds = []predicate.Predicate{predicates.Default()}
	}

	hdler := state.Config.Handler
	if hdler == nil {
		hdler = handler.EnqueueRequestForOwner(
			w.client.Scheme(),
			w.client.RESTMapper(),
			ownerObj,
			handler.OnlyControllerOwner(),
		)
	}

	src := source.Kind(
		w.cache,
		watchObj,
		hdler,
		preds...,
	)

	if err := w.controller.Watch(src); err != nil {
		return fmt.Errorf("failed to watch %s: %w", gvk, err)
	}

	// Retrieve controller name from context, use "unknown" as fallback
	controllerName := "unknown"
	if name, ok := reconciler.ControllerNameFromContext(ctx); ok {
		controllerName = name
	}

	DynamicWatchedResourcesTotal.WithLabelValues(
		controllerName,
		gvk.GroupVersion().String(),
		gvk.Kind,
	).Set(1)

	return nil
}

// state returns the State for a GVK, creating it if it doesn't exist.
func (w *Watcher) state(gvk schema.GroupVersionKind) *State {
	state, exists := w.states[gvk]
	if !exists {
		state = &State{
			Config:  Config{GVK: gvk},
			Watched: false,
		}

		w.states[gvk] = state
	}

	return state
}
