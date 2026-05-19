package resources

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// GVKFromCRD extracts the GroupVersionKind of the type defined by a CRD.
// It parses spec.group, spec.names.kind, and the first served version
// from spec.versions. The CRD must be an unstructured representation
// of a CustomResourceDefinition.
func GVKFromCRD(crd *unstructured.Unstructured) (schema.GroupVersionKind, error) {
	group, found, err := unstructured.NestedString(crd.Object, "spec", "group")
	if err != nil {
		return schema.GroupVersionKind{}, fmt.Errorf("reading CRD spec.group: %w", err)
	}

	if !found || group == "" {
		return schema.GroupVersionKind{}, fmt.Errorf("CRD %s missing spec.group", crd.GetName())
	}

	kind, found, err := unstructured.NestedString(crd.Object, "spec", "names", "kind")
	if err != nil {
		return schema.GroupVersionKind{}, fmt.Errorf("reading CRD spec.names.kind: %w", err)
	}

	if !found || kind == "" {
		return schema.GroupVersionKind{}, fmt.Errorf("CRD %s missing spec.names.kind", crd.GetName())
	}

	versions, found, err := unstructured.NestedSlice(crd.Object, "spec", "versions")
	if err != nil {
		return schema.GroupVersionKind{}, fmt.Errorf("reading CRD spec.versions: %w", err)
	}

	if !found {
		return schema.GroupVersionKind{}, fmt.Errorf("CRD %s missing spec.versions", crd.GetName())
	}

	for _, v := range versions {
		vm, ok := v.(map[string]any)
		if !ok {
			continue
		}

		served, _, _ := unstructured.NestedBool(vm, "served")
		name, _, _ := unstructured.NestedString(vm, "name")

		if served && name != "" {
			return schema.GroupVersionKind{
				Group:   group,
				Version: name,
				Kind:    kind,
			}, nil
		}
	}

	return schema.GroupVersionKind{}, fmt.Errorf("CRD %s has no served version", crd.GetName())
}
