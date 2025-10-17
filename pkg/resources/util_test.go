package resources_test

import (
	"testing"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/resources"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"

	. "github.com/onsi/gomega"
)

func TestToUnstructured(t *testing.T) {
	g := NewWithT(t)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "default",
		},
		Data: map[string]string{
			"key": "value",
		},
	}

	u, err := resources.ToUnstructured(scheme.Scheme, cm)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(u).ToNot(BeNil())
	g.Expect(u.GetName()).To(Equal("test-cm"))
	g.Expect(u.GetNamespace()).To(Equal("default"))

	data, found, err := unstructured.NestedString(u.Object, "data", "key")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(found).To(BeTrue())
	g.Expect(data).To(Equal("value"))
}

func TestToUnstructuredNilObject(t *testing.T) {
	g := NewWithT(t)

	u, err := resources.ToUnstructured(scheme.Scheme, nil)
	g.Expect(err).To(HaveOccurred())
	g.Expect(u).To(BeNil())
}

func TestObjectToUnstructured(t *testing.T) {
	g := NewWithT(t)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "default",
		},
		Data: map[string]string{
			"key": "value",
		},
	}

	u, err := resources.ToUnstructured(scheme.Scheme, cm)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(u).ToNot(BeNil())
	g.Expect(u.GetName()).To(Equal("test-cm"))
	g.Expect(u.GetNamespace()).To(Equal("default"))
	g.Expect(u.GroupVersionKind()).To(Equal(schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "ConfigMap",
	}))
}

func TestObjectFromUnstructured(t *testing.T) {
	g := NewWithT(t)

	u := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":      "test-cm",
				"namespace": "default",
			},
			"data": map[string]interface{}{
				"key": "value",
			},
		},
	}

	var cm corev1.ConfigMap
	err := resources.FromUnstructured(scheme.Scheme, u, &cm)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(cm.Name).To(Equal("test-cm"))
	g.Expect(cm.Namespace).To(Equal("default"))
	g.Expect(cm.Data).To(HaveKeyWithValue("key", "value"))
	g.Expect(cm.GroupVersionKind()).To(Equal(schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "ConfigMap",
	}))
}

func TestObjectFromUnstructuredNilObject(t *testing.T) {
	g := NewWithT(t)

	var cm corev1.ConfigMap
	err := resources.FromUnstructured(scheme.Scheme, nil, &cm)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("nil object"))
}

func TestGetGroupVersionKindForObject(t *testing.T) {
	g := NewWithT(t)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "default",
		},
	}

	gvk, err := resources.GetGroupVersionKindForObject(scheme.Scheme, cm)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(gvk).To(Equal(schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "ConfigMap",
	}))
}

func TestGetGroupVersionKindForObjectWithGVKSet(t *testing.T) {
	g := NewWithT(t)

	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "default",
		},
	}

	gvk, err := resources.GetGroupVersionKindForObject(scheme.Scheme, cm)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(gvk).To(Equal(schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "ConfigMap",
	}))
}

func TestGetGroupVersionKindForObjectNilObject(t *testing.T) {
	g := NewWithT(t)

	gvk, err := resources.GetGroupVersionKindForObject(scheme.Scheme, nil)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("nil object"))
	g.Expect(gvk).To(Equal(schema.GroupVersionKind{}))
}

func TestEnsureGroupVersionKind(t *testing.T) {
	g := NewWithT(t)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "default",
		},
	}

	err := resources.EnsureGroupVersionKind(scheme.Scheme, cm)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(cm.GroupVersionKind()).To(Equal(schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "ConfigMap",
	}))
}

func TestGvkToUnstructured(t *testing.T) {
	g := NewWithT(t)

	gvk := schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "ConfigMap",
	}

	u := resources.GvkToUnstructured(gvk)
	g.Expect(u).ToNot(BeNil())
	g.Expect(u.GroupVersionKind()).To(Equal(gvk))
}

func TestFormatObjectReference(t *testing.T) {
	t.Run("namespaced object", func(t *testing.T) {
		g := NewWithT(t)

		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "",
			Version: "v1",
			Kind:    "ConfigMap",
		})
		u.SetName("test-cm")
		u.SetNamespace("default")

		ref := resources.FormatObjectReference(u)
		g.Expect(ref).To(Equal("/v1, Kind=ConfigMap default/test-cm"))
	})

	t.Run("cluster-scoped object", func(t *testing.T) {
		g := NewWithT(t)

		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "",
			Version: "v1",
			Kind:    "Namespace",
		})
		u.SetName("test-ns")

		ref := resources.FormatObjectReference(u)
		g.Expect(ref).To(Equal("/v1, Kind=Namespace test-ns"))
	})
}

func TestNamespacedNameFromObject(t *testing.T) {
	g := NewWithT(t)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "default",
		},
	}

	nn := resources.NamespacedNameFromObject(cm)
	g.Expect(nn).To(Equal(types.NamespacedName{
		Namespace: "default",
		Name:      "test-cm",
	}))
}

func TestFormatNamespacedName(t *testing.T) {
	t.Run("namespaced", func(t *testing.T) {
		g := NewWithT(t)

		nn := types.NamespacedName{
			Namespace: "default",
			Name:      "test-cm",
		}

		s := resources.FormatNamespacedName(nn)
		g.Expect(s).To(Equal("default/test-cm"))
	})

	t.Run("cluster-scoped", func(t *testing.T) {
		g := NewWithT(t)

		nn := types.NamespacedName{
			Name: "test-ns",
		}

		s := resources.FormatNamespacedName(nn)
		g.Expect(s).To(Equal("test-ns"))
	})
}

func TestRoundTripConversion(t *testing.T) {
	g := NewWithT(t)

	original := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "default",
			Labels: map[string]string{
				"app": "test",
			},
		},
		Data: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
	}

	// Convert to unstructured
	u, err := resources.ToUnstructured(scheme.Scheme, original)
	g.Expect(err).ToNot(HaveOccurred())

	// Convert back to typed
	var result corev1.ConfigMap
	err = resources.FromUnstructured(scheme.Scheme, u, &result)
	g.Expect(err).ToNot(HaveOccurred())

	// Verify all fields match
	g.Expect(result.Name).To(Equal(original.Name))
	g.Expect(result.Namespace).To(Equal(original.Namespace))
	g.Expect(result.Labels).To(Equal(original.Labels))
	g.Expect(result.Data).To(Equal(original.Data))
}

func TestUnknownGVK(t *testing.T) {
	g := NewWithT(t)

	// Create an unstructured object with an unknown GVK
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "custom.example.com",
		Version: "v1",
		Kind:    "Unknown",
	})
	u.SetName("test")
	u.SetNamespace("default")

	// This should fail when trying to convert to a typed object
	var cm corev1.ConfigMap
	err := resources.FromUnstructured(scheme.Scheme, u, &cm)
	g.Expect(err).Should(HaveOccurred())
}

func TestEnsureGroupVersionKindPreservesExisting(t *testing.T) {
	g := NewWithT(t)

	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "default",
		},
	}

	// Get the initial GVK
	initialGVK := cm.GroupVersionKind()

	// Call EnsureGroupVersionKind
	err := resources.EnsureGroupVersionKind(scheme.Scheme, cm)
	g.Expect(err).ToNot(HaveOccurred())

	// The GVK should be the same
	g.Expect(cm.GroupVersionKind()).To(Equal(initialGVK))
}

func TestObjectFromUnstructuredWithInvalidGVK(t *testing.T) {
	g := NewWithT(t)

	// Create an unstructured object with an unknown GVK
	u := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "unknown.example.com/v1",
			"kind":       "Unknown",
			"metadata": map[string]interface{}{
				"name":      "test",
				"namespace": "default",
			},
		},
	}

	var cm corev1.ConfigMap
	err := resources.FromUnstructured(scheme.Scheme, u, &cm)
	g.Expect(err).To(HaveOccurred())
}

func TestConversionPreservesRuntimeObject(t *testing.T) {
	g := NewWithT(t)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test-cm",
			Namespace:       "default",
			ResourceVersion: "12345",
			UID:             "abc-123",
			Generation:      1,
		},
		Data: map[string]string{
			"key": "value",
		},
	}

	u, err := resources.ToUnstructured(scheme.Scheme, cm)
	g.Expect(err).ToNot(HaveOccurred())

	var result corev1.ConfigMap
	err = resources.FromUnstructured(scheme.Scheme, u, &result)
	g.Expect(err).ToNot(HaveOccurred())

	// Verify metadata is preserved
	g.Expect(result.ResourceVersion).To(Equal(cm.ResourceVersion))
	g.Expect(result.UID).To(Equal(cm.UID))
	g.Expect(result.Generation).To(Equal(cm.Generation))
}

func TestObjectToUnstructuredImplementsRuntimeObject(t *testing.T) {
	g := NewWithT(t)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "default",
		},
	}

	u, err := resources.ToUnstructured(scheme.Scheme, cm)
	g.Expect(err).ToNot(HaveOccurred())

	// Verify it implements runtime.Object
	var _ runtime.Object = u
	g.Expect(u.GetObjectKind()).ToNot(BeNil())
}
