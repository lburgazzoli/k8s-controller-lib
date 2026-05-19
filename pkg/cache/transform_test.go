package cache_test

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	libcache "github.com/lburgazzoli/k8s-controller-lib/pkg/cache"

	. "github.com/onsi/gomega"
)

func TestStripUnusedFields_RemovesManagedFields(t *testing.T) {
	g := NewWithT(t)

	obj := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:          "test",
			ManagedFields: []metav1.ManagedFieldsEntry{{Manager: "kubectl"}},
		},
	}

	result, err := libcache.StripUnusedFields()(obj)

	g.Expect(err).ToNot(HaveOccurred())

	cm := result.(*corev1.ConfigMap)
	g.Expect(cm.ManagedFields).To(BeNil())
}

func TestStripUnusedFields_RemovesLastAppliedAnnotation(t *testing.T) {
	g := NewWithT(t)

	obj := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
			Annotations: map[string]string{
				libcache.AnnotationLastAppliedConfiguration: `{"some":"config"}`,
				"other-annotation":                          "keep",
			},
		},
	}

	result, err := libcache.StripUnusedFields()(obj)

	g.Expect(err).ToNot(HaveOccurred())

	cm := result.(*corev1.ConfigMap)
	g.Expect(cm.Annotations).To(HaveLen(1))
	g.Expect(cm.Annotations).To(HaveKey("other-annotation"))
	g.Expect(cm.Annotations).ToNot(HaveKey(libcache.AnnotationLastAppliedConfiguration))
}

func TestStripUnusedFields_NoAnnotations(t *testing.T) {
	g := NewWithT(t)

	obj := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
	}

	result, err := libcache.StripUnusedFields()(obj)

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result).To(Equal(obj))
}

func TestStripUnusedFields_NonAccessorPassesThrough(t *testing.T) {
	g := NewWithT(t)

	obj := "not-a-k8s-object"

	result, err := libcache.StripUnusedFields()(obj)

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result).To(Equal(obj))
}
