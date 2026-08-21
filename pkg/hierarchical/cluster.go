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

// Package hierarchical creates child controller-runtime clusters whose local
// caches fall back to a parent manager cache for shared object types.
package hierarchical

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
)

// ErrClusterStopped is returned when work is registered after child shutdown begins.
var ErrClusterStopped = errors.New("hierarchical cluster is stopped")

// Cluster is a controller-runtime cluster backed by a hierarchical cache.
// Values are created by NewCluster; the unexported methods intentionally keep
// lifecycle registration consistent with ControllerManagedBy.
type Cluster interface {
	cluster.Cluster

	// Parent returns the manager that owns shared infrastructure and cache data.
	Parent() manager.Manager

	// Stop permanently stops this child's controllers and then its local cache.
	// It is safe to call Stop more than once.
	Stop(ctx context.Context) error

	registerRunnable(runnable manager.Runnable) error
	logger() logr.Logger
}

type hierarchicalCluster struct {
	cluster.Cluster

	parent        manager.Manager
	cache         *lifecycleCache
	lifecycle     *lifecycle
	clusterLogger logr.Logger
}

var _ Cluster = (*hierarchicalCluster)(nil)

// NewCluster constructs a child cluster. Call parent.Add(child) before the
// parent manager starts so controller-runtime starts and synchronizes the local
// cache with its other caches.
func NewCluster(parent manager.Manager, opts ...ClusterOption) (Cluster, error) {
	if parent == nil {
		return nil, errors.New("parent manager is required")
	}

	options := &ClusterOptions{
		Scheme: parent.GetScheme(),
		Logger: parent.GetLogger(),
	}
	options.ApplyOptions(opts)
	if parent.GetScheme() == nil {
		return nil, errors.New("parent manager scheme is required")
	}
	if parent.GetConfig() == nil {
		return nil, errors.New("parent manager REST config is required")
	}
	if err := options.Validate(); err != nil {
		return nil, err
	}

	options.ByObject = cloneByObjectMap(options.ByObject)
	options.Namespaces = append([]string(nil), options.Namespaces...)

	localGVKs, err := deriveLocalGVKs(options.Scheme, parent.GetScheme(), options.ByObject)
	if err != nil {
		return nil, fmt.Errorf("derive local cache routing: %w", err)
	}

	logger := options.Logger
	if logger.GetSink() == nil {
		logger = parent.GetLogger()
	}
	logger = logger.WithName("hierarchical").WithValues("cluster", options.Name)

	core, err := newCoreCluster(parent, options, logger, localGVKs)
	if err != nil {
		return nil, fmt.Errorf("create child cluster %q: %w", options.Name, err)
	}

	lifetime := newLifecycle()
	managedCache := &lifecycleCache{Cache: core.GetCache(), lifecycle: lifetime}

	return &hierarchicalCluster{
		Cluster:       core,
		parent:        parent,
		cache:         managedCache,
		lifecycle:     lifetime,
		clusterLogger: logger,
	}, nil
}

func newCoreCluster(
	parent manager.Manager,
	options *ClusterOptions,
	logger logr.Logger,
	localGVKs map[schema.GroupVersionKind]struct{},
) (cluster.Cluster, error) {
	cacheOptions := cache.Options{
		ReaderFailOnMissingInformer: true,
		ByObject:                    cloneByObjectMap(options.ByObject),
		DefaultNamespaces:           namespaceConfigs(options.Namespaces),
	}

	core, err := cluster.New(rest.CopyConfig(parent.GetConfig()), func(co *cluster.Options) {
		co.Scheme = options.Scheme
		co.Logger = logger
		co.HTTPClient = parent.GetHTTPClient()
		co.MapperProvider = func(*rest.Config, *http.Client) (meta.RESTMapper, error) {
			return parent.GetRESTMapper(), nil
		}
		co.Cache = cacheOptions
		co.NewCache = func(cfg *rest.Config, opts cache.Options) (cache.Cache, error) {
			local, err := cache.New(cfg, opts)
			if err != nil {
				return nil, fmt.Errorf("create local cache: %w", err)
			}

			return &hierarchicalCache{
				scheme:       options.Scheme,
				parentScheme: parent.GetScheme(),
				local:        local,
				parent:       parent.GetCache(),
				localGVKs:    localGVKs,
			}, nil
		}
	})
	if err != nil {
		return nil, fmt.Errorf("initialize controller-runtime cluster: %w", err)
	}

	return core, nil
}

// Parent returns the manager that owns this child.
func (c *hierarchicalCluster) Parent() manager.Manager {
	return c.parent
}

// GetCache returns the lifecycle-aware hierarchical cache.
func (c *hierarchicalCluster) GetCache() cache.Cache {
	return c.cache
}

// GetFieldIndexer returns the hierarchical cache's field indexer.
func (c *hierarchicalCluster) GetFieldIndexer() client.FieldIndexer {
	return c.cache
}

// Start starts the local cache until the parent or child lifecycle is cancelled.
func (c *hierarchicalCluster) Start(ctx context.Context) error {
	return c.lifecycle.run(ctx, cacheGroup, c.Cluster.Start)
}

// Stop permanently stops child controllers followed by the local cache.
func (c *hierarchicalCluster) Stop(ctx context.Context) error {
	return c.lifecycle.stop(ctx)
}

func (c *hierarchicalCluster) registerRunnable(runnable manager.Runnable) error {
	if c.lifecycle.isStopped() {
		return ErrClusterStopped
	}

	if err := c.parent.Add(&lifecycleRunnable{Runnable: runnable, lifecycle: c.lifecycle}); err != nil {
		return fmt.Errorf("register child runnable: %w", err)
	}

	return nil
}

func (c *hierarchicalCluster) logger() logr.Logger {
	return c.clusterLogger
}

func cloneByObjectMap(in map[client.Object]cache.ByObject) map[client.Object]cache.ByObject {
	if in == nil {
		return nil
	}

	out := maps.Clone(in)
	for obj, byObject := range out {
		out[obj] = cloneByObject(byObject)
	}

	return out
}

func namespaceConfigs(namespaces []string) map[string]cache.Config {
	if len(namespaces) == 0 {
		return nil
	}

	configs := make(map[string]cache.Config, len(namespaces))
	for _, namespace := range namespaces {
		configs[namespace] = cache.Config{}
	}

	return configs
}
