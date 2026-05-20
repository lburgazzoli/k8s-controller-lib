package watch

import (
	"context"
	"fmt"
	"strings"
	"sync"

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
// The Config field contains the watch configuration (predicates, handler, etc.).
// The Watched field indicates whether a watch has been successfully registered for this GVK.
type State struct {
	Config  Config
	Watched bool
}

// Clone returns a deep copy of the State.
// The returned copy is safe to modify without affecting the original.
func (s *State) Clone() *State {
	if s == nil {
		return nil
	}

	return &State{
		Config:  s.Config.Clone(),
		Watched: s.Watched,
	}
}

// Watcher manages automatic watch setup for Kubernetes objects.
// It tracks which GVKs have been watched and applies custom or default
// predicates and handlers when setting up new watches.
//
// For objects watched with PartialObjectMetadata, default predicates are NOT
// applied automatically. You must explicitly provide predicates via WithPredicates()
// if filtering is needed.
//
// Watcher is safe for concurrent use by multiple goroutines.
type Watcher struct {
	controller controller.Controller
	cache      cache.Cache
	client     client.Client
	mu         *sync.RWMutex
	states     map[schema.GroupVersionKind]*State
}

// New creates a new Watcher configured with the given options.
// Dependencies (controller, cache, client) can be provided via options
// (WithClient, WithCache, WithController) or injected later via setter methods.
// Watch() is a no-op until all dependencies are available.
//
// Example standalone:
//
//	watcher := watch.New(
//	    watch.WithClient(client),
//	    watch.WithController(ctrl),
//	    watch.WithCache(cache),
//	    watch.WithConfigs(
//	        watch.NewConfig(deploymentGVK, watch.WithPredicates(pred1, pred2)),
//	        watch.NewConfig(serviceGVK, watch.Disabled()),
//	    ),
//	)
//
// Example with Builder (dependencies injected automatically):
//
//	watcher := watch.New()  // deps injected by Builder via Pipeline forwarding
func New(opts ...Option) *Watcher {
	options := &Options{}
	options.ApplyOptions(opts)

	w := &Watcher{
		controller: options.Controller,
		cache:      options.Cache,
		client:     options.Client,
		mu:         &sync.RWMutex{},
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

// SetClient sets the client on the Watcher.
func (w *Watcher) SetClient(c client.Client) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.client = c
}

// SetCache sets the cache on the Watcher.
func (w *Watcher) SetCache(c cache.Cache) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.cache = c
}

// SetController sets the controller on the Watcher.
func (w *Watcher) SetController(ctrl controller.Controller) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.controller = ctrl
}

// ready returns true if all required dependencies are available.
func (w *Watcher) ready() bool {
	return w.controller != nil && w.cache != nil && w.client != nil
}

// Watch sets up a watch for the provided object if not already watched.
// Uses owner references to map events from watched objects back to the owner.
//
// The context should contain the controller name for metrics labeling, injected via
// reconciler.WithControllerName(). If not present, the lowercase GVK kind will be used as fallback.
//
// Optional ConfigOption arguments allow per-object watch configuration (predicates, handler,
// partial, disabled) that merges with any pre-configured state for the GVK. Since watches
// register once per GVK, the first Watch call for a given GVK establishes the config.
//
// A nil obj is silently skipped (no-op).
//
// Watch is safe to call concurrently from multiple goroutines.
func (w *Watcher) Watch(
	ctx context.Context,
	ownerObj client.Object,
	obj client.Object,
	opts ...ConfigOption,
) error {
	if obj == nil || !w.ready() {
		return nil
	}

	return w.watchObject(ctx, ownerObj, obj, opts...)
}

// All returns an ApplyHookFunc that calls w.Watch for each object in the slice.
// Nil objects are silently skipped.
//
// Use this for pipeline integration:
//
//	w := watch.New(...)
//	pipeline.WithPostApply(watch.All(w))
func All(w *Watcher) reconciler.ApplyHookFunc {
	return func(ctx context.Context, ownerObj client.Object, objects []client.Object) error {
		for _, obj := range objects {
			if obj == nil {
				continue
			}

			if err := w.Watch(ctx, ownerObj, obj); err != nil {
				return err
			}
		}

		return nil
	}
}

// watchObject sets up a watch for a single object if not already watched.
// It extracts the GVK, checks/creates state, and registers the watch with the controller.
// Call-site opts are applied to the state's config before registration.
// Updates metrics upon successful registration.
//
// Uses optimistic locking: first checks with RLock (fast path for already-watched GVKs),
// then acquires full Lock only when registration is needed.
//
// The caller must NOT hold w.mu lock as this method acquires it.
func (w *Watcher) watchObject(
	ctx context.Context,
	ownerObj client.Object,
	obj client.Object,
	opts ...ConfigOption,
) error {
	gvk, err := apiutil.GVKForObject(obj, w.client.Scheme())
	if err != nil {
		return fmt.Errorf("unable to get GVK for %T: %w", obj, err)
	}

	// Fast path: check with read lock if already watched or disabled
	w.mu.RLock()
	if state, exists := w.states[gvk]; exists {
		if state.Watched || state.Config.Disabled {
			w.mu.RUnlock()

			return nil
		}
	}
	w.mu.RUnlock()

	// Slow path: acquire write lock for registration
	w.mu.Lock()
	defer w.mu.Unlock()

	// Re-check after acquiring write lock (another goroutine may have won)
	state := w.setupState(gvk)

	for _, opt := range opts {
		opt.ApplyTo(&state.Config)
	}

	if state.Watched || state.Config.Disabled {
		return nil
	}

	src, err := w.setupSource(obj, ownerObj, state)
	if err != nil {
		return fmt.Errorf("unable to setup watch for %s: %w", gvk, err)
	}

	if err := w.controller.Watch(src); err != nil {
		return fmt.Errorf("failed to watch %s: %w", gvk, err)
	}

	// Retrieve controller name from context, use lowercase Kind as
	// fallback
	var controllerName string
	if name, ok := reconciler.ControllerNameFromContext(ctx); ok {
		controllerName = name
	} else {
		controllerName = strings.ToLower(gvk.Kind)
	}

	DynamicWatchedResourcesTotal.WithLabelValues(
		controllerName,
		gvk.GroupVersion().String(),
		gvk.Kind,
	).Set(1)

	state.Watched = true

	return nil
}

// setupSource creates a source.Source for watching an object.
// Handles PartialObjectMetadata conversion if configured, applies predicates
// (with special handling for partial watches), and sets up the event handler.
// Returns a configured source.Source ready for controller.Watch().
func (w *Watcher) setupSource(
	obj client.Object,
	ownerObj client.Object,
	state *State,
) (source.Source, error) {
	watchObj := obj
	if state.Config.Partial {
		partial, err := resources.ToPartialObjectMetadata(w.client.Scheme(), obj)
		if err != nil {
			return nil, fmt.Errorf("failed to convert to PartialObjectMetadata: %w", err)
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
		hdler = EnqueueRequestForOwnerOrLabel(
			w.client.Scheme(),
			ownerObj,
			true, // only controller owner
			LabelOwnerName,
			LabelOwnerNamespace,
		)
	}

	src := source.Kind(
		w.cache,
		watchObj,
		hdler,
		preds...,
	)

	return src, nil
}

// State returns a copy of the watch state for the given GVK.
// Returns nil if the GVK has not been encountered yet (either pre-configured or watched).
//
// The returned State is a deep clone and safe to inspect or modify without affecting
// internal state.
// This method is safe for concurrent use.
func (w *Watcher) State(gvk schema.GroupVersionKind) *State {
	w.mu.RLock()
	defer w.mu.RUnlock()

	state, exists := w.states[gvk]
	if !exists {
		return nil
	}

	return state.Clone()
}

// setupState returns the State for a GVK, creating it if it doesn't exist.
// The caller must hold w.mu lock when calling this method.
func (w *Watcher) setupState(gvk schema.GroupVersionKind) *State {
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
