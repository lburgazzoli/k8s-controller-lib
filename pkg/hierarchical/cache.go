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
	"reflect"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ErrSharedInformerRemoval is returned when a child attempts to remove an
// informer owned by the parent cache.
var ErrSharedInformerRemoval = errors.New("cannot remove an informer owned by the parent cache")

type hierarchicalCache struct {
	scheme       *runtime.Scheme
	parentScheme *runtime.Scheme
	local        cache.Cache
	parent       cache.Cache
	localGVKs    map[schema.GroupVersionKind]struct{}
}

var _ cache.Cache = (*hierarchicalCache)(nil)

func (c *hierarchicalCache) Get(
	ctx context.Context,
	key client.ObjectKey,
	obj client.Object,
	opts ...client.GetOption,
) error {
	selected, _, err := c.cacheForObject(obj)
	if err != nil {
		return err
	}

	if err := selected.Get(ctx, key, obj, opts...); err != nil {
		return fmt.Errorf("get %T %s: %w", obj, key, err)
	}

	return nil
}

func (c *hierarchicalCache) List(
	ctx context.Context,
	list client.ObjectList,
	opts ...client.ListOption,
) error {
	selected, _, err := c.cacheForObject(list)
	if err != nil {
		return err
	}

	if err := selected.List(ctx, list, opts...); err != nil {
		return fmt.Errorf("list %T: %w", list, err)
	}

	return nil
}

func (c *hierarchicalCache) GetInformer(
	ctx context.Context,
	obj client.Object,
	opts ...cache.InformerGetOption,
) (cache.Informer, error) {
	selected, _, err := c.cacheForObject(obj)
	if err != nil {
		return nil, err
	}

	informer, err := selected.GetInformer(ctx, obj, opts...)
	if err != nil {
		return nil, fmt.Errorf("get informer for %T: %w", obj, err)
	}

	return informer, nil
}

func (c *hierarchicalCache) GetInformerForKind(
	ctx context.Context,
	gvk schema.GroupVersionKind,
	opts ...cache.InformerGetOption,
) (cache.Informer, error) {
	informer, err := c.cacheForGVK(gvk).GetInformerForKind(ctx, gvk, opts...)
	if err != nil {
		return nil, fmt.Errorf("get informer for GVK %s: %w", gvk, err)
	}

	return informer, nil
}

func (c *hierarchicalCache) RemoveInformer(ctx context.Context, obj client.Object) error {
	selected, gvk, err := c.cacheForObject(obj)
	if err != nil {
		return err
	}
	if selected == c.parent {
		return fmt.Errorf("%w: %s", ErrSharedInformerRemoval, gvk)
	}

	if err := selected.RemoveInformer(ctx, obj); err != nil {
		return fmt.Errorf("remove local informer for %s: %w", gvk, err)
	}

	return nil
}

func (c *hierarchicalCache) Start(ctx context.Context) error {
	if err := c.local.Start(ctx); err != nil {
		return fmt.Errorf("start local cache: %w", err)
	}

	return nil
}

func (c *hierarchicalCache) WaitForCacheSync(ctx context.Context) bool {
	return c.local.WaitForCacheSync(ctx)
}

func (c *hierarchicalCache) IndexField(
	ctx context.Context,
	obj client.Object,
	field string,
	extractValue client.IndexerFunc,
) error {
	selected, _, err := c.cacheForObject(obj)
	if err != nil {
		return err
	}

	if err := selected.IndexField(ctx, obj, field, extractValue); err != nil {
		return fmt.Errorf("index field %q for %T: %w", field, obj, err)
	}

	return nil
}

func (c *hierarchicalCache) cacheForObject(obj runtime.Object) (cache.Cache, schema.GroupVersionKind, error) {
	gvk, err := c.gvkForObject(obj)
	if err != nil {
		return nil, schema.GroupVersionKind{}, err
	}
	gvk = normalizeGVK(gvk)

	return c.cacheForGVK(gvk), gvk, nil
}

func (c *hierarchicalCache) gvkForObject(obj runtime.Object) (schema.GroupVersionKind, error) {
	gvk, err := apiutil.GVKForObject(obj, c.scheme)
	if err == nil {
		return gvk, nil
	}

	gvk, err = apiutil.GVKForObject(obj, c.parentScheme)
	if err != nil {
		return schema.GroupVersionKind{}, fmt.Errorf("resolve cache GVK for %T: %w", obj, err)
	}

	return gvk, nil
}

func (c *hierarchicalCache) cacheForGVK(gvk schema.GroupVersionKind) cache.Cache {
	if _, ok := c.localGVKs[normalizeGVK(gvk)]; ok {
		return c.local
	}

	return c.parent
}

func normalizeGVK(gvk schema.GroupVersionKind) schema.GroupVersionKind {
	gvk.Kind = strings.TrimSuffix(gvk.Kind, "List")

	return gvk
}

func deriveLocalGVKs(
	childScheme *runtime.Scheme,
	parentScheme *runtime.Scheme,
	byObject map[client.Object]cache.ByObject,
) (map[schema.GroupVersionKind]struct{}, error) {
	local := make(map[schema.GroupVersionKind]struct{})
	parentTypes := parentScheme.AllKnownTypes()

	for gvk, childType := range childScheme.AllKnownTypes() {
		parentType, shared := parentTypes[gvk]
		if !shared {
			local[normalizeGVK(gvk)] = struct{}{}

			continue
		}
		if parentType == childType {
			continue
		}

		return nil, fmt.Errorf(
			"GVK %s maps to child type %s and parent type %s",
			gvk,
			childType,
			parentType,
		)
	}

	for obj := range byObject {
		if isNilObject(obj) {
			return nil, errors.New("ByObject contains a nil object")
		}
		gvk, err := apiutil.GVKForObject(obj, childScheme)
		if err != nil {
			return nil, fmt.Errorf("resolve ByObject GVK for %T: %w", obj, err)
		}
		local[normalizeGVK(gvk)] = struct{}{}
	}

	return local, nil
}

func isNilObject(obj client.Object) bool {
	if obj == nil {
		return true
	}

	value := reflect.ValueOf(obj)

	return value.Kind() == reflect.Pointer && value.IsNil()
}
