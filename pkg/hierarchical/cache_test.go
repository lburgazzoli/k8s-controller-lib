//nolint:testpackage // White-box tests verify cache routing internals.
package hierarchical

import (
	"context"
	"errors"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	. "github.com/onsi/gomega"
)

func testChildGVK() schema.GroupVersionKind {
	return schema.GroupVersionKind{Group: "child.test", Version: "v1", Kind: "Widget"}
}

func TestHierarchicalCacheRoutesOperations(t *testing.T) {
	g := NewWithT(t)
	local := &recordingCache{synced: true}
	parent := &recordingCache{synced: true}
	childScheme := runtime.NewScheme()
	parentScheme := runtime.NewScheme()
	childGVK := testChildGVK()
	g.Expect(corev1.AddToScheme(parentScheme)).To(Succeed())

	c := &hierarchicalCache{
		scheme:       childScheme,
		parentScheme: parentScheme,
		local:        local,
		parent:       parent,
		localGVKs:    map[schema.GroupVersionKind]struct{}{childGVK: {}},
	}
	ctx := t.Context()
	child := objectFor(childGVK)
	shared := objectFor(corev1.SchemeGroupVersion.WithKind("ConfigMap"))
	childList := &unstructured.UnstructuredList{}
	childList.SetGroupVersionKind(childGVK.GroupVersion().WithKind("WidgetList"))

	g.Expect(c.Get(ctx, client.ObjectKey{Name: "child"}, child)).To(Succeed())
	g.Expect(c.List(ctx, childList)).To(Succeed())
	_, err := c.GetInformer(ctx, child)
	g.Expect(err).To(MatchError(ContainSubstring("test informer is not implemented")))
	_, err = c.GetInformerForKind(ctx, childGVK)
	g.Expect(err).To(MatchError(ContainSubstring("test informer is not implemented")))
	g.Expect(c.IndexField(ctx, child, "field", func(client.Object) []string { return nil })).To(Succeed())
	g.Expect(c.RemoveInformer(ctx, child)).To(Succeed())
	g.Expect(local.calls).To(Equal([]string{"get", "list", "informer", "informer-kind", "index", "remove"}))

	g.Expect(c.Get(ctx, client.ObjectKey{Name: "shared"}, shared)).To(Succeed())
	_, err = c.GetInformerForKind(ctx, corev1.SchemeGroupVersion.WithKind("ConfigMap"))
	g.Expect(err).To(MatchError(ContainSubstring("test informer is not implemented")))
	g.Expect(parent.calls).To(Equal([]string{"get", "informer-kind"}))

	err = c.RemoveInformer(ctx, shared)
	g.Expect(err).To(MatchError(MatchRegexp("cannot remove an informer owned by the parent cache.*ConfigMap")))
	g.Expect(errors.Is(err, ErrSharedInformerRemoval)).To(BeTrue())
	g.Expect(parent.calls).To(Equal([]string{"get", "informer-kind"}))
}

func TestHierarchicalCacheLifecycleIsLocalOnly(t *testing.T) {
	g := NewWithT(t)
	local := &recordingCache{synced: true}
	parent := &recordingCache{synced: true}
	c := &hierarchicalCache{local: local, parent: parent}

	g.Expect(c.Start(t.Context())).To(Succeed())
	g.Expect(c.WaitForCacheSync(t.Context())).To(BeTrue())
	g.Expect(local.calls).To(Equal([]string{"start", "wait"}))
	g.Expect(parent.calls).To(BeEmpty())
}

func TestDeriveLocalGVKs(t *testing.T) {
	g := NewWithT(t)
	parentScheme := runtime.NewScheme()
	childScheme := runtime.NewScheme()
	childGVK := testChildGVK()
	g.Expect(corev1.AddToScheme(parentScheme)).To(Succeed())
	g.Expect(corev1.AddToScheme(childScheme)).To(Succeed())
	childScheme.AddKnownTypeWithName(childGVK, &corev1.ConfigMap{})

	local, err := deriveLocalGVKs(
		childScheme,
		parentScheme,
		map[client.Object]cache.ByObject{&corev1.Secret{}: {}},
	)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(local).To(HaveKey(childGVK))
	g.Expect(local).To(HaveKey(corev1.SchemeGroupVersion.WithKind("Secret")))
	g.Expect(local).ToNot(HaveKey(corev1.SchemeGroupVersion.WithKind("ConfigMap")))
}

func TestDeriveLocalGVKsRejectsConflictingTypes(t *testing.T) {
	g := NewWithT(t)
	gvk := schema.GroupVersionKind{Group: "test", Version: "v1", Kind: "Conflict"}
	parentScheme := runtime.NewScheme()
	parentScheme.AddKnownTypeWithName(gvk, &corev1.ConfigMap{})
	childScheme := runtime.NewScheme()
	childScheme.AddKnownTypeWithName(gvk, &corev1.Secret{})

	_, err := deriveLocalGVKs(childScheme, parentScheme, nil)
	g.Expect(err).To(MatchError(ContainSubstring("maps to child type")))
}

func TestGVKForObjectUsesChildThenParentScheme(t *testing.T) {
	g := NewWithT(t)
	childScheme := runtime.NewScheme()
	parentScheme := runtime.NewScheme()
	childGVK := testChildGVK()
	childScheme.AddKnownTypeWithName(childGVK, &corev1.ConfigMap{})
	g.Expect(corev1.AddToScheme(parentScheme)).To(Succeed())
	c := &hierarchicalCache{scheme: childScheme, parentScheme: parentScheme}

	gvk, err := c.gvkForObject(&corev1.ConfigMap{})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(gvk).To(Equal(childGVK))

	gvk, err = c.gvkForObject(&corev1.Secret{})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(gvk).To(Equal(corev1.SchemeGroupVersion.WithKind("Secret")))

	_, err = c.gvkForObject(&metav1.PartialObjectMetadata{})
	g.Expect(err).To(MatchError(ContainSubstring("resolve cache GVK")))
}

func objectFor(gvk schema.GroupVersionKind) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)

	return obj
}

type recordingCache struct {
	calls  []string
	synced bool
}

func (c *recordingCache) record(call string) {
	c.calls = append(c.calls, call)
}

func (c *recordingCache) Get(
	context.Context,
	client.ObjectKey,
	client.Object,
	...client.GetOption,
) error {
	c.record("get")

	return nil
}

func (c *recordingCache) List(context.Context, client.ObjectList, ...client.ListOption) error {
	c.record("list")

	return nil
}

func (c *recordingCache) GetInformer(
	context.Context,
	client.Object,
	...cache.InformerGetOption,
) (cache.Informer, error) {
	c.record("informer")

	return nil, errors.New("test informer is not implemented")
}

func (c *recordingCache) GetInformerForKind(
	context.Context,
	schema.GroupVersionKind,
	...cache.InformerGetOption,
) (cache.Informer, error) {
	c.record("informer-kind")

	return nil, errors.New("test informer is not implemented")
}

func (c *recordingCache) RemoveInformer(context.Context, client.Object) error {
	c.record("remove")

	return nil
}

func (c *recordingCache) Start(context.Context) error {
	c.record("start")

	return nil
}

func (c *recordingCache) WaitForCacheSync(context.Context) bool {
	c.record("wait")

	return c.synced
}

func (c *recordingCache) IndexField(
	context.Context,
	client.Object,
	string,
	client.IndexerFunc,
) error {
	c.record("index")

	return nil
}

var _ cache.Cache = (*recordingCache)(nil)
