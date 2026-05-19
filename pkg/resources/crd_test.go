package resources_test

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/resources"

	. "github.com/onsi/gomega"
)

func TestGVKFromCRD_ValidCRD(t *testing.T) {
	g := NewWithT(t)

	crd := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": "widgets.example.com"},
		"spec": map[string]any{
			"group": "example.com",
			"names": map[string]any{"kind": "Widget"},
			"versions": []any{
				map[string]any{"name": "v1", "served": true, "storage": true},
			},
		},
	}}

	gvk, err := resources.GVKFromCRD(crd)

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(gvk).To(Equal(schema.GroupVersionKind{
		Group:   "example.com",
		Version: "v1",
		Kind:    "Widget",
	}))
}

func TestGVKFromCRD_MultipleVersions_PicksFirstServed(t *testing.T) {
	g := NewWithT(t)

	crd := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": "widgets.example.com"},
		"spec": map[string]any{
			"group": "example.com",
			"names": map[string]any{"kind": "Widget"},
			"versions": []any{
				map[string]any{"name": "v1alpha1", "served": false},
				map[string]any{"name": "v1beta1", "served": true},
				map[string]any{"name": "v1", "served": true, "storage": true},
			},
		},
	}}

	gvk, err := resources.GVKFromCRD(crd)

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(gvk.Version).To(Equal("v1beta1"))
}

func TestGVKFromCRD_MissingGroup(t *testing.T) {
	g := NewWithT(t)

	crd := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": "test"},
		"spec": map[string]any{
			"names":    map[string]any{"kind": "Widget"},
			"versions": []any{map[string]any{"name": "v1", "served": true}},
		},
	}}

	_, err := resources.GVKFromCRD(crd)

	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("missing spec.group"))
}

func TestGVKFromCRD_MissingKind(t *testing.T) {
	g := NewWithT(t)

	crd := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": "test"},
		"spec": map[string]any{
			"group":    "example.com",
			"names":    map[string]any{},
			"versions": []any{map[string]any{"name": "v1", "served": true}},
		},
	}}

	_, err := resources.GVKFromCRD(crd)

	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("missing spec.names.kind"))
}

func TestGVKFromCRD_NoServedVersion(t *testing.T) {
	g := NewWithT(t)

	crd := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": "test"},
		"spec": map[string]any{
			"group": "example.com",
			"names": map[string]any{"kind": "Widget"},
			"versions": []any{
				map[string]any{"name": "v1", "served": false},
			},
		},
	}}

	_, err := resources.GVKFromCRD(crd)

	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("no served version"))
}

func TestGVKFromCRD_MissingVersions(t *testing.T) {
	g := NewWithT(t)

	crd := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": "test"},
		"spec": map[string]any{
			"group": "example.com",
			"names": map[string]any{"kind": "Widget"},
		},
	}}

	_, err := resources.GVKFromCRD(crd)

	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("missing spec.versions"))
}
