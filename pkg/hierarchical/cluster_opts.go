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
	"errors"
	"maps"
	"slices"
	"strings"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/util"
)

// ClusterOption configures a child Cluster during construction.
type ClusterOption = util.Option[ClusterOptions]

// ClusterOptions holds configuration for a child Cluster.
type ClusterOptions struct {
	// Name identifies the child in logs. It must be non-empty.
	Name string

	// Scheme contains every typed object used by the child. Types that are not
	// known by the parent are routed to the local cache.
	Scheme *runtime.Scheme

	// Logger is used by the child cluster. It defaults to the parent logger.
	Logger logr.Logger

	// ByObject forces the configured object types into the local cache and
	// applies controller-runtime's per-object cache settings.
	ByObject map[client.Object]cache.ByObject

	// Namespaces limits locally cached namespaced objects. Parent-routed types
	// retain the parent cache's namespace scope.
	Namespaces []string
}

// Validate verifies that the child cluster options are internally consistent.
func (o *ClusterOptions) Validate() error {
	if o == nil {
		return errors.New("child cluster options are required")
	}
	if strings.TrimSpace(o.Name) == "" {
		return errors.New("child cluster name is required")
	}
	if o.Scheme == nil {
		return errors.New("child cluster scheme is required")
	}
	if slices.Contains(o.Namespaces, "") {
		return errors.New("child cache namespace must not be empty")
	}

	return nil
}

// ApplyTo implements Option for ClusterOptions.
func (o *ClusterOptions) ApplyTo(target *ClusterOptions) {
	if o.Name != "" {
		target.Name = o.Name
	}
	if o.Scheme != nil {
		target.Scheme = o.Scheme
	}
	if o.Logger.GetSink() != nil {
		target.Logger = o.Logger
	}
	if len(o.ByObject) > 0 {
		if target.ByObject == nil {
			target.ByObject = make(map[client.Object]cache.ByObject, len(o.ByObject))
		}
		for obj, byObject := range o.ByObject {
			target.ByObject[obj] = cloneByObject(byObject)
		}
	}
	target.Namespaces = append(target.Namespaces, o.Namespaces...)
}

// ApplyOptions applies all options in order.
func (o *ClusterOptions) ApplyOptions(opts []ClusterOption) *ClusterOptions {
	for _, opt := range opts {
		opt.ApplyTo(o)
	}

	return o
}

// WithName sets the child cluster name.
func WithName(name string) ClusterOption {
	return util.FunctionalOption[ClusterOptions](func(opts *ClusterOptions) {
		opts.Name = name
	})
}

// WithScheme sets the scheme used by the child cache and client.
func WithScheme(scheme *runtime.Scheme) ClusterOption {
	return util.FunctionalOption[ClusterOptions](func(opts *ClusterOptions) {
		opts.Scheme = scheme
	})
}

// WithLogger sets the child cluster logger.
func WithLogger(logger logr.Logger) ClusterOption {
	return util.FunctionalOption[ClusterOptions](func(opts *ClusterOptions) {
		opts.Logger = logger
	})
}

// WithByObject forces obj into the local cache with the supplied cache settings.
func WithByObject(obj client.Object, byObject cache.ByObject) ClusterOption {
	return util.FunctionalOption[ClusterOptions](func(opts *ClusterOptions) {
		if opts.ByObject == nil {
			opts.ByObject = make(map[client.Object]cache.ByObject)
		}
		opts.ByObject[obj] = cloneByObject(byObject)
	})
}

// WithNamespaces limits locally cached namespaced objects to namespaces.
// Multiple calls are additive.
func WithNamespaces(namespaces ...string) ClusterOption {
	return util.FunctionalOption[ClusterOptions](func(opts *ClusterOptions) {
		opts.Namespaces = append(opts.Namespaces, namespaces...)
	})
}

func cloneByObject(in cache.ByObject) cache.ByObject {
	in.Namespaces = maps.Clone(in.Namespaces)

	return in
}
