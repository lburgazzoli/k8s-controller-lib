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
	"fmt"
	"net/http"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/events"
	"k8s.io/client-go/tools/record"
)

// ControllerManagedBy creates a standard controller-runtime builder whose
// controllers use the child cache, client, scheme, and lifecycle while the
// parent manager retains controller infrastructure and leader election.
func ControllerManagedBy(child Cluster) *builder.Builder {
	if child == nil {
		panic("hierarchical cluster is required")
	}

	return builder.ControllerManagedBy(&scopedManager{
		Manager: child.Parent(),
		child:   child,
	})
}

type scopedManager struct {
	manager.Manager

	child Cluster
}

var _ manager.Manager = (*scopedManager)(nil)

func (m *scopedManager) Add(runnable manager.Runnable) error {
	return m.child.registerRunnable(runnable)
}

func (m *scopedManager) GetHTTPClient() *http.Client {
	return m.child.GetHTTPClient()
}

func (m *scopedManager) GetConfig() *rest.Config {
	return m.child.GetConfig()
}

func (m *scopedManager) GetCache() cache.Cache {
	return m.child.GetCache()
}

func (m *scopedManager) GetScheme() *runtime.Scheme {
	return m.child.GetScheme()
}

func (m *scopedManager) GetClient() client.Client {
	return m.child.GetClient()
}

func (m *scopedManager) GetFieldIndexer() client.FieldIndexer {
	return m.child.GetFieldIndexer()
}

func (m *scopedManager) GetRESTMapper() meta.RESTMapper {
	return m.child.GetRESTMapper()
}

func (m *scopedManager) GetAPIReader() client.Reader {
	return m.child.GetAPIReader()
}

func (m *scopedManager) GetEventRecorderFor(name string) record.EventRecorder {
	return m.child.GetEventRecorderFor(name)
}

func (m *scopedManager) GetEventRecorder(name string) events.EventRecorder {
	return m.child.GetEventRecorder(name)
}

func (m *scopedManager) GetLogger() logr.Logger {
	return m.child.logger()
}

func (m *scopedManager) Start(ctx context.Context) error {
	if err := m.Manager.Start(ctx); err != nil {
		return fmt.Errorf("start parent manager: %w", err)
	}

	return nil
}
