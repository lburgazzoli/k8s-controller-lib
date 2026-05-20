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

	"github.com/lburgazzoli/k8s-controller-lib/pkg/reconciler"
)

// TypedPipeline wraps Pipeline with a type-safe reconciler interface.
// It implements TypedReconciler[T] and inherits ControllerAware, ClientAware,
// CacheAware, and ExternalWatchesAware from the embedded Pipeline.
//
// This allows the pipeline to be passed directly to Builder.Complete(), enabling
// a simplified setup pattern where controller, cache, and client are automatically injected.
//
// Example usage:
//
//	p := pipeline.NewTyped[*v1alpha1.MyApp](
//	    pipeline.WithFieldOwner("my-controller"),
//	    pipeline.WithActions(myAction),
//	    pipeline.WithPostApply(watch.All(watch.New())),
//	)
//
//	b, _ := builder.NewControllerBuilder[*v1alpha1.MyApp](mgr)
//	b.For(&v1alpha1.MyApp{}).Complete(p)
type TypedPipeline[T reconciler.ManagedObject] struct {
	*Pipeline // Embed Pipeline to inherit aware interfaces
}

// NewTyped creates a TypedPipeline ready for Builder.Complete().
// All dependencies (client, controller, cache) are injected by Builder via
// the aware interfaces inherited from the embedded Pipeline.
//
// Parameters:
//   - opts: Pipeline options including actions, cleanup actions, field owner, hooks, etc.
//
// Example:
//
//	p := pipeline.NewTyped[*v1alpha1.MyApp](
//	    pipeline.WithFieldOwner("my-controller"),
//	    pipeline.WithActions(myAction),
//	    pipeline.WithPostApply(watch.All(watcher)),
//	)
//	b.For(&v1alpha1.MyApp{}).Complete(p)
func NewTyped[T reconciler.ManagedObject](opts ...Option) *TypedPipeline[T] {
	return &TypedPipeline[T]{
		Pipeline: NewPipeline(opts...),
	}
}

// Reconcile implements reconciler.TypedReconciler[T].
// It delegates to the underlying Pipeline.Reconcile and converts the result.
func (p *TypedPipeline[T]) Reconcile(
	ctx context.Context,
	req *reconciler.TypedRequest[T],
) (*reconciler.Response, error) {
	result, err := p.Pipeline.Reconcile(ctx, req.Object)

	return reconciler.ResponseFromResult(result), err
}
