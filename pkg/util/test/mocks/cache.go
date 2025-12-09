// Copyright 2025 The k8s-controller-lib Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Cache is a mock implementation of cache.Cache for testing.
type Cache struct {
	mock.Mock
}

// NewCache creates a new Cache mock for testing.
func NewCache() *Cache {
	return &Cache{}
}

// Get implements cache.Cache.
func (m *Cache) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	args := m.Called(ctx, key, obj, opts)
	return args.Error(0)
}

// List implements cache.Cache.
func (m *Cache) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	args := m.Called(ctx, list, opts)
	return args.Error(0)
}

// GetInformer implements cache.Cache.
func (m *Cache) GetInformer(ctx context.Context, obj client.Object, opts ...cache.InformerGetOption) (cache.Informer, error) {
	args := m.Called(ctx, obj, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(cache.Informer), args.Error(1)
}

// GetInformerForKind implements cache.Cache.
func (m *Cache) GetInformerForKind(ctx context.Context, gvk schema.GroupVersionKind, opts ...cache.InformerGetOption) (cache.Informer, error) {
	args := m.Called(ctx, gvk, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(cache.Informer), args.Error(1)
}

// RemoveInformer implements cache.Cache.
func (m *Cache) RemoveInformer(ctx context.Context, obj client.Object) error {
	args := m.Called(ctx, obj)
	return args.Error(0)
}

// Start implements cache.Cache.
func (m *Cache) Start(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// WaitForCacheSync implements cache.Cache.
func (m *Cache) WaitForCacheSync(ctx context.Context) bool {
	args := m.Called(ctx)
	return args.Bool(0)
}

// IndexField implements cache.Cache.
func (m *Cache) IndexField(ctx context.Context, obj client.Object, field string, extractValue client.IndexerFunc) error {
	args := m.Called(ctx, obj, field, extractValue)
	return args.Error(0)
}

// Ensure Cache implements cache.Cache.
var _ cache.Cache = (*Cache)(nil)
