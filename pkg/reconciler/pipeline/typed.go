/*
Copyright 2025 The k8s-controller-lib Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package pipeline

import (
	"context"
	"errors"
	"sync"

	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/reconciler"
	"github.com/lburgazzoli/k8s-controller-lib/pkg/reconciler/watch"
)

// TypedPipeline wraps Pipeline with a type-safe reconciler interface.
// It implements TypedReconciler[T], ControllerAware, ClientAware, CacheAware,
// and ExternalWatchesAware for automatic dependency injection from Builder.
//
// This allows the pipeline to be passed directly to Builder.Complete(), enabling
// a simplified setup pattern where controller and cache are automatically injected.
//
// Example usage:
//
//	p := pipeline.NewTyped[*v1alpha1.MyApp](
//	    mgr.GetClient(),
//	    pipeline.WithFieldOwner("my-controller"),
//	    pipeline.WithActions(myAction),
//	    pipeline.DeferredAutoWatch(),  // Controller/cache injected by builder
//	)
//
//	b, _ := builder.NewControllerBuilder[*v1alpha1.MyApp](mgr)
//	b.For(&v1alpha1.MyApp{}).Complete(p)
type TypedPipeline[T reconciler.ManagedObject] struct {
	pipeline             *Pipeline
	client               client.Client
	cache                cache.Cache
	ctrl                 controller.Controller
	opts                 []Option
	externalWatches      []schema.GroupVersionKind
	hasDeferredAutoWatch bool // Cached at construction to avoid repeated option scanning
	mu                   sync.Mutex
}

// NewTyped creates a TypedPipeline ready for Builder.Complete().
// The controller and cache are injected by Builder via aware interfaces
// when using DeferredAutoWatch().
//
// Parameters:
//   - c: The Kubernetes client for API operations
//   - opts: Pipeline options including actions, cleanup actions, field owner, etc.
//
// For auto-watch support, use DeferredAutoWatch() instead of WithAutoWatch().
// The controller and cache will be automatically injected by Builder.Complete().
func NewTyped[T reconciler.ManagedObject](
	c client.Client,
	opts ...Option,
) *TypedPipeline[T] {
	// Check once during construction if deferred auto-watch is configured
	hasDeferredAutoWatch := false
	for _, opt := range opts {
		if _, ok := opt.(*deferredAutoWatch); ok {
			hasDeferredAutoWatch = true

			break
		}
	}

	p := &TypedPipeline[T]{
		client:               c,
		opts:                 opts,
		hasDeferredAutoWatch: hasDeferredAutoWatch,
	}

	// Initialize immediately if no DeferredAutoWatch is configured
	p.init()

	return p
}

// Reconcile implements reconciler.TypedReconciler[T].
// It delegates to the underlying Pipeline.Reconcile and converts the result.
func (p *TypedPipeline[T]) Reconcile(
	ctx context.Context,
	req *reconciler.TypedRequest[T],
) (*reconciler.Response, error) {
	p.mu.Lock()
	pipeline := p.pipeline
	p.mu.Unlock()

	if pipeline == nil {
		return nil, errors.New("pipeline not initialized - ensure Builder.Complete() was called")
	}

	result, err := pipeline.Reconcile(ctx, req.Object)

	return reconciler.ResponseFromResult(result), err
}

// SetController implements reconciler.ControllerAware.
// Called by Builder.Complete() to inject the controller after creation.
func (p *TypedPipeline[T]) SetController(ctrl controller.Controller) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.ctrl = ctrl
	p.init()
}

// SetCache implements reconciler.CacheAware.
// Called by Builder.Complete() to inject the cache.
func (p *TypedPipeline[T]) SetCache(c cache.Cache) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.cache = c
	p.init()
}

// SetClient implements reconciler.ClientAware.
// Called by Builder.Complete() to inject the client.
// Note: The client is typically provided in NewTyped(), but this allows override.
func (p *TypedPipeline[T]) SetClient(c client.Client) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.client = c
	p.init()
}

// SetExternalWatches implements reconciler.ExternalWatchesAware.
// Called by Builder.Complete() to inform the pipeline about GVKs already watched by Builder.
// These GVKs will be marked as already watched in the Watcher, preventing redundant registration.
func (p *TypedPipeline[T]) SetExternalWatches(gvks []schema.GroupVersionKind) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.externalWatches = gvks
	p.init()
}

// init initializes the underlying Pipeline once all dependencies are available.
// The caller must hold p.mu.
func (p *TypedPipeline[T]) init() {
	// Only initialize once all required dependencies are set
	if p.client == nil || p.pipeline != nil {
		return
	}

	// If deferred auto-watch is configured (cached at construction), wait for both controller and cache
	if p.hasDeferredAutoWatch && (p.ctrl == nil || p.cache == nil) {
		return
	}

	// Process options, replacing DeferredAutoWatch with concrete WithAutoWatch
	finalOpts := make([]Option, 0, len(p.opts))
	for _, opt := range p.opts {
		if awOpt, ok := opt.(*deferredAutoWatch); ok {
			if p.ctrl != nil && p.cache != nil {
				// Combine deferred options with external watches injected by Builder
				autoWatchOpts := make([]watch.AutoWatchOption, 0, len(awOpt.opts)+1)
				autoWatchOpts = append(autoWatchOpts, awOpt.opts...)
				if len(p.externalWatches) > 0 {
					autoWatchOpts = append(autoWatchOpts, watch.WithExternallyWatched(p.externalWatches...))
				}
				finalOpts = append(finalOpts, WithAutoWatch(p.ctrl, p.cache, autoWatchOpts...))
			}
			// Skip deferred auto-watch if controller/cache not available
		} else {
			finalOpts = append(finalOpts, opt)
		}
	}

	// Create the pipeline - error is only returned if client is nil, which we checked above
	pipeline, _ := NewPipeline(p.client, finalOpts...)
	p.pipeline = pipeline
}

// GetPipeline returns the underlying Pipeline, or nil if not yet initialized.
// This can be used to access Pipeline-specific functionality after initialization.
func (p *TypedPipeline[T]) GetPipeline() *Pipeline {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.pipeline
}
