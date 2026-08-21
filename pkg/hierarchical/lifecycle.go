/*
Copyright 2026 The k8s-controller-lib Authors.

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

package hierarchical

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type lifecycleGroup uint8

const (
	controllerGroup lifecycleGroup = iota
	cacheGroup
)

type lifecycle struct {
	mu sync.Mutex

	stopped bool
	done    chan struct{}

	controllerDone    <-chan struct{}
	cancelControllers context.CancelFunc
	controllerActive  int
	controllerIdle    chan struct{}

	cacheDone   <-chan struct{}
	cancelCache context.CancelFunc
	cacheActive int
	cacheIdle   chan struct{}

	stopOnce sync.Once
}

func newLifecycle() *lifecycle {
	controllerCtx, cancelControllers := context.WithCancel(context.Background())
	cacheCtx, cancelCache := context.WithCancel(context.Background())

	return &lifecycle{
		done:              make(chan struct{}),
		controllerDone:    controllerCtx.Done(),
		cancelControllers: cancelControllers,
		controllerIdle:    closedChannel(),
		cacheDone:         cacheCtx.Done(),
		cancelCache:       cancelCache,
		cacheIdle:         closedChannel(),
	}
}

func (l *lifecycle) stop(ctx context.Context) error {
	l.stopOnce.Do(func() {
		l.mu.Lock()
		l.stopped = true
		controllerIdle := l.controllerIdle
		cacheIdle := l.cacheIdle
		l.cancelControllers()
		l.mu.Unlock()

		go func() {
			<-controllerIdle
			l.cancelCache()
			<-cacheIdle
			close(l.done)
		}()
	})

	select {
	case <-l.done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("stop hierarchical cluster: %w", ctx.Err())
	}
}

func (l *lifecycle) isStopped() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.stopped
}

func (l *lifecycle) run(
	parent context.Context,
	group lifecycleGroup,
	start func(context.Context) error,
) error {
	stopped, ok := l.begin(group)
	if !ok {
		return nil
	}
	defer l.end(group)

	ctx, cancel := mergeContexts(parent, stopped)
	defer cancel()

	err := start(ctx)
	if err != nil && ctx.Err() != nil && errors.Is(err, ctx.Err()) {
		return nil
	}

	return err
}

func (l *lifecycle) begin(group lifecycleGroup) (<-chan struct{}, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.stopped {
		return nil, false
	}

	switch group {
	case controllerGroup:
		if l.controllerActive == 0 {
			l.controllerIdle = make(chan struct{})
		}
		l.controllerActive++

		return l.controllerDone, true
	case cacheGroup:
		if l.cacheActive == 0 {
			l.cacheIdle = make(chan struct{})
		}
		l.cacheActive++

		return l.cacheDone, true
	default:
		panic("unknown hierarchical lifecycle group")
	}
}

func (l *lifecycle) end(group lifecycleGroup) {
	l.mu.Lock()
	defer l.mu.Unlock()

	switch group {
	case controllerGroup:
		l.controllerActive--
		if l.controllerActive == 0 {
			close(l.controllerIdle)
		}
	case cacheGroup:
		l.cacheActive--
		if l.cacheActive == 0 {
			close(l.cacheIdle)
		}
	default:
		panic("unknown hierarchical lifecycle group")
	}
}

func (l *lifecycle) cacheContext(parent context.Context) (context.Context, context.CancelFunc) {
	return mergeContexts(parent, l.cacheDone)
}

func mergeContexts(parent context.Context, stopped <-chan struct{}) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)
	finished := make(chan struct{})
	var finishOnce sync.Once
	go func() {
		select {
		case <-stopped:
			cancel()
		case <-finished:
		}
	}()

	return ctx, func() {
		finishOnce.Do(func() { close(finished) })
		cancel()
	}
}

func closedChannel() chan struct{} {
	ch := make(chan struct{})
	close(ch)

	return ch
}

type lifecycleCache struct {
	cache.Cache

	lifecycle *lifecycle
}

var _ cache.Cache = (*lifecycleCache)(nil)

func (c *lifecycleCache) WaitForCacheSync(ctx context.Context) bool {
	if c.lifecycle.isStopped() {
		return true
	}

	ctx, cancel := c.lifecycle.cacheContext(ctx)
	defer cancel()

	synced := c.Cache.WaitForCacheSync(ctx)

	return synced || c.lifecycle.isStopped()
}

type lifecycleRunnable struct {
	manager.Runnable

	lifecycle *lifecycle
}

var (
	_ manager.Runnable               = (*lifecycleRunnable)(nil)
	_ manager.LeaderElectionRunnable = (*lifecycleRunnable)(nil)
)

func (r *lifecycleRunnable) Start(ctx context.Context) error {
	return r.lifecycle.run(ctx, controllerGroup, r.Runnable.Start)
}

func (r *lifecycleRunnable) NeedLeaderElection() bool {
	if runnable, ok := r.Runnable.(manager.LeaderElectionRunnable); ok {
		return runnable.NeedLeaderElection()
	}

	return true
}

func (r *lifecycleRunnable) Warmup(ctx context.Context) error {
	if runnable, ok := r.Runnable.(interface {
		Warmup(ctx context.Context) error
	}); ok {
		return r.lifecycle.run(ctx, controllerGroup, runnable.Warmup)
	}

	return nil
}
