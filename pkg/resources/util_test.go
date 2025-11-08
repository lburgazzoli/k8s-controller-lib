package resources_test

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/resources"
	"github.com/lburgazzoli/k8s-controller-lib/pkg/resources/gvks"

	. "github.com/onsi/gomega"
)

func TestToUnstructured(t *testing.T) {
	t.Run("convert ConfigMap to Unstructured", func(t *testing.T) {
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
	})
}

func TestToUnstructuredNilObject(t *testing.T) {
	t.Run("convert nil object", func(t *testing.T) {
		g := NewWithT(t)

		u, err := resources.ToUnstructured(scheme.Scheme, nil)
		g.Expect(err).To(HaveOccurred())
		g.Expect(u).To(BeNil())
	})
}

func TestObjectToUnstructured(t *testing.T) {
	t.Run("convert ConfigMap to Unstructured with GVK", func(t *testing.T) {
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
		g.Expect(u.GroupVersionKind()).To(Equal(gvks.ConfigMap))
	})
}

func TestObjectFromUnstructured(t *testing.T) {
	t.Run("convert Unstructured to ConfigMap", func(t *testing.T) {
		g := NewWithT(t)

		u := &unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]any{
					"name":      "test-cm",
					"namespace": "default",
				},
				"data": map[string]any{
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
		g.Expect(cm.GroupVersionKind()).To(Equal(gvks.ConfigMap))
	})
}

func TestObjectFromUnstructuredNilObject(t *testing.T) {
	t.Run("convert nil Unstructured fails", func(t *testing.T) {
		g := NewWithT(t)

		var cm corev1.ConfigMap
		err := resources.FromUnstructured(scheme.Scheme, nil, &cm)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("nil object"))
	})
}

func TestGetGroupVersionKindForObject(t *testing.T) {
	t.Run("get GVK for ConfigMap", func(t *testing.T) {
		g := NewWithT(t)

		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cm",
				Namespace: "default",
			},
		}

		gvk, err := resources.GetGroupVersionKindForObject(scheme.Scheme, cm)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(gvk).To(Equal(gvks.ConfigMap))
	})
}

func TestGetGroupVersionKindForObjectWithGVKSet(t *testing.T) {
	t.Run("get GVK for ConfigMap with TypeMeta", func(t *testing.T) {
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
		g.Expect(gvk).To(Equal(gvks.ConfigMap))
	})
}

func TestGetGroupVersionKindForObjectNilObject(t *testing.T) {
	t.Run("get GVK for nil object fails", func(t *testing.T) {
		g := NewWithT(t)

		gvk, err := resources.GetGroupVersionKindForObject(scheme.Scheme, nil)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("nil object"))
		g.Expect(gvk).To(Equal(schema.GroupVersionKind{}))
	})
}

func TestEnsureGroupVersionKind(t *testing.T) {
	t.Run("set GVK for ConfigMap", func(t *testing.T) {
		g := NewWithT(t)

		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cm",
				Namespace: "default",
			},
		}

		err := resources.EnsureGroupVersionKind(scheme.Scheme, cm)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(cm.GroupVersionKind()).To(Equal(gvks.ConfigMap))
	})
}

func TestGvkToUnstructured(t *testing.T) {
	t.Run("create Unstructured from ConfigMap GVK", func(t *testing.T) {
		g := NewWithT(t)

		u := resources.GvkToUnstructured(gvks.ConfigMap)
		g.Expect(u).ToNot(BeNil())
		g.Expect(u.GroupVersionKind()).To(Equal(gvks.ConfigMap))
	})
}

func TestFormatObjectReference(t *testing.T) {
	t.Run("namespaced object", func(t *testing.T) {
		g := NewWithT(t)

		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(gvks.ConfigMap)
		u.SetName("test-cm")
		u.SetNamespace("default")

		ref := resources.FormatObjectReference(u)
		g.Expect(ref).To(Equal("/v1, Kind=ConfigMap default/test-cm"))
	})

	t.Run("cluster-scoped object", func(t *testing.T) {
		g := NewWithT(t)

		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(gvks.Namespace)
		u.SetName("test-ns")

		ref := resources.FormatObjectReference(u)
		g.Expect(ref).To(Equal("/v1, Kind=Namespace test-ns"))
	})
}

func TestNamespacedNameFromObject(t *testing.T) {
	t.Run("extract NamespacedName from ConfigMap", func(t *testing.T) {
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
	})
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
	t.Run("unstructured round-trip preserves ConfigMap metadata", func(t *testing.T) {
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
	})
}

func TestUnknownGVK(t *testing.T) {
	t.Run("convert unknown GVK fails", func(t *testing.T) {
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
	})
}

func TestEnsureGroupVersionKindPreservesExisting(t *testing.T) {
	t.Run("EnsureGroupVersionKind preserves existing TypeMeta", func(t *testing.T) {
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
	})
}

func TestObjectFromUnstructuredWithInvalidGVK(t *testing.T) {
	t.Run("convert Unstructured with invalid GVK fails", func(t *testing.T) {
		g := NewWithT(t)

		// Create an unstructured object with an unknown GVK
		u := &unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": "unknown.example.com/v1",
				"kind":       "Unknown",
				"metadata": map[string]any{
					"name":      "test",
					"namespace": "default",
				},
			},
		}

		var cm corev1.ConfigMap
		err := resources.FromUnstructured(scheme.Scheme, u, &cm)
		g.Expect(err).To(HaveOccurred())
	})
}

func TestConversionPreservesRuntimeObject(t *testing.T) {
	t.Run("Unstructured conversion preserves runtime object metadata", func(t *testing.T) {
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
	})
}

func TestObjectToUnstructuredImplementsRuntimeObject(t *testing.T) {
	t.Run("Unstructured implements runtime.Object interface", func(t *testing.T) {
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
	})
}

func TestToPartialObjectMetadata(t *testing.T) {
	t.Run("convert ConfigMap to PartialObjectMetadata", func(t *testing.T) {
		g := NewWithT(t)

		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "test-cm",
				Namespace:       "default",
				UID:             "test-uid",
				ResourceVersion: "123",
				Generation:      5,
				Labels: map[string]string{
					"app": "myapp",
				},
				Annotations: map[string]string{
					"note": "test",
				},
				Finalizers: []string{"finalizer1"},
			},
			Data: map[string]string{
				"key": "value",
			},
		}

		partial, err := resources.ToPartialObjectMetadata(scheme.Scheme, cm)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(partial).ToNot(BeNil())

		// Verify metadata was copied
		g.Expect(partial.GetName()).To(Equal("test-cm"))
		g.Expect(partial.GetNamespace()).To(Equal("default"))
		g.Expect(partial.GetUID()).To(Equal(types.UID("test-uid")))
		g.Expect(partial.GetResourceVersion()).To(Equal("123"))
		g.Expect(partial.GetGeneration()).To(Equal(int64(5)))
		g.Expect(partial.GetLabels()).To(Equal(map[string]string{"app": "myapp"}))
		g.Expect(partial.GetAnnotations()).To(Equal(map[string]string{"note": "test"}))
		g.Expect(partial.GetFinalizers()).To(Equal([]string{"finalizer1"}))

		// Verify GVK is set
		gvk := partial.GetObjectKind().GroupVersionKind()
		g.Expect(gvk.Group).To(Equal(""))
		g.Expect(gvk.Version).To(Equal("v1"))
		g.Expect(gvk.Kind).To(Equal("ConfigMap"))
	})
}

func TestToPartialObjectMetadataFromPartial(t *testing.T) {
	t.Run("convert PartialObjectMetadata creates deep copy", func(t *testing.T) {
		g := NewWithT(t)

		original := &metav1.PartialObjectMetadata{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cm",
				Namespace: "default",
				Labels: map[string]string{
					"app": "myapp",
				},
			},
		}

		partial, err := resources.ToPartialObjectMetadata(scheme.Scheme, original)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(partial).ToNot(BeNil())

		// Verify it's a different instance (deep copy)
		g.Expect(partial).ToNot(BeIdenticalTo(original))

		// Verify metadata was copied
		g.Expect(partial.GetName()).To(Equal("test-cm"))
		g.Expect(partial.GetNamespace()).To(Equal("default"))
		g.Expect(partial.GetLabels()).To(Equal(map[string]string{"app": "myapp"}))

		// Modify the copy and ensure original is unchanged
		partial.Labels["new"] = "label"
		g.Expect(original.GetLabels()).ToNot(HaveKey("new"))
	})
}

func TestToPartialObjectMetadataNilObject(t *testing.T) {
	t.Run("convert nil object to PartialObjectMetadata fails", func(t *testing.T) {
		g := NewWithT(t)

		partial, err := resources.ToPartialObjectMetadata(scheme.Scheme, nil)
		g.Expect(err).To(HaveOccurred())
		g.Expect(partial).To(BeNil())
	})
}

func TestIsPartialObjectMetadata(t *testing.T) {
	t.Run("identify valid PartialObjectMetadata", func(t *testing.T) {
		g := NewWithT(t)

		partial := &metav1.PartialObjectMetadata{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cm",
				Namespace: "default",
			},
		}

		g.Expect(resources.IsPartialObjectMetadata(partial)).To(BeTrue())
	})
}

func TestIsPartialObjectMetadataRegularObject(t *testing.T) {
	t.Run("identify regular object as not PartialObjectMetadata", func(t *testing.T) {
		g := NewWithT(t)

		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cm",
				Namespace: "default",
			},
		}

		g.Expect(resources.IsPartialObjectMetadata(cm)).To(BeFalse())
	})
}

func TestIsPartialObjectMetadataNil(t *testing.T) {
	t.Run("identify nil object as not PartialObjectMetadata", func(t *testing.T) {
		g := NewWithT(t)

		g.Expect(resources.IsPartialObjectMetadata(nil)).To(BeFalse())
	})
}
