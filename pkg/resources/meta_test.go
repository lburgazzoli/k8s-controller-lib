package resources_test

import (
	"testing"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/resources"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/gomega"
)

func TestHasLabel(t *testing.T) {
	t.Run("returns true when label matches one of the values", func(t *testing.T) {
		g := NewWithT(t)

		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"env": "production",
				},
			},
		}

		g.Expect(resources.HasLabel(cm, "env", "production", "staging")).To(BeTrue())
	})

	t.Run("returns false when label does not match any value", func(t *testing.T) {
		g := NewWithT(t)

		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"env": "production",
				},
			},
		}

		g.Expect(resources.HasLabel(cm, "env", "staging", "development")).To(BeFalse())
	})

	t.Run("returns false when label key does not exist", func(t *testing.T) {
		g := NewWithT(t)

		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"env": "production",
				},
			},
		}

		g.Expect(resources.HasLabel(cm, "tier", "frontend")).To(BeFalse())
	})

	t.Run("returns false when labels map is nil", func(t *testing.T) {
		g := NewWithT(t)

		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{},
		}

		g.Expect(resources.HasLabel(cm, "env", "production")).To(BeFalse())
	})

	t.Run("returns false when object is nil", func(t *testing.T) {
		g := NewWithT(t)

		g.Expect(resources.HasLabel(nil, "env", "production")).To(BeFalse())
	})

	t.Run("returns false when values slice is empty", func(t *testing.T) {
		g := NewWithT(t)

		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"env": "production",
				},
			},
		}

		g.Expect(resources.HasLabel(cm, "env")).To(BeFalse())
	})
}

func TestSetLabels(t *testing.T) {
	t.Run("sets multiple labels on object with no existing labels", func(t *testing.T) {
		g := NewWithT(t)

		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{},
		}

		resources.SetLabels(cm, map[string]string{
			"env":  "production",
			"tier": "frontend",
		})

		g.Expect(cm.Labels).To(HaveKeyWithValue("env", "production"))
		g.Expect(cm.Labels).To(HaveKeyWithValue("tier", "frontend"))
	})

	t.Run("adds labels to object with existing labels", func(t *testing.T) {
		g := NewWithT(t)

		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"app": "myapp",
				},
			},
		}

		resources.SetLabels(cm, map[string]string{
			"env":  "production",
			"tier": "frontend",
		})

		g.Expect(cm.Labels).To(HaveKeyWithValue("app", "myapp"))
		g.Expect(cm.Labels).To(HaveKeyWithValue("env", "production"))
		g.Expect(cm.Labels).To(HaveKeyWithValue("tier", "frontend"))
	})

	t.Run("overwrites existing label values", func(t *testing.T) {
		g := NewWithT(t)

		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"env": "staging",
				},
			},
		}

		resources.SetLabels(cm, map[string]string{
			"env": "production",
		})

		g.Expect(cm.Labels).To(HaveKeyWithValue("env", "production"))
	})

	t.Run("handles nil object gracefully", func(t *testing.T) {
		g := NewWithT(t)

		// Should not panic
		resources.SetLabels(nil, map[string]string{"env": "production"})

		g.Expect(true).To(BeTrue())
	})
}

func TestSetLabel(t *testing.T) {
	t.Run("sets label on object with no existing labels", func(t *testing.T) {
		g := NewWithT(t)

		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{},
		}

		old := resources.SetLabel(cm, "env", "production")

		g.Expect(cm.Labels).To(HaveKeyWithValue("env", "production"))
		g.Expect(old).To(BeEmpty())
	})

	t.Run("adds label to object with existing labels", func(t *testing.T) {
		g := NewWithT(t)

		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"app": "myapp",
				},
			},
		}

		old := resources.SetLabel(cm, "env", "production")

		g.Expect(cm.Labels).To(HaveKeyWithValue("app", "myapp"))
		g.Expect(cm.Labels).To(HaveKeyWithValue("env", "production"))
		g.Expect(old).To(BeEmpty())
	})

	t.Run("returns old value when overwriting existing label", func(t *testing.T) {
		g := NewWithT(t)

		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"env": "staging",
				},
			},
		}

		old := resources.SetLabel(cm, "env", "production")

		g.Expect(cm.Labels).To(HaveKeyWithValue("env", "production"))
		g.Expect(old).To(Equal("staging"))
	})

	t.Run("returns empty string for nil object", func(t *testing.T) {
		g := NewWithT(t)

		old := resources.SetLabel(nil, "env", "production")

		g.Expect(old).To(BeEmpty())
	})
}

func TestRemoveLabel(t *testing.T) {
	t.Run("removes existing label", func(t *testing.T) {
		g := NewWithT(t)

		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"env":  "production",
					"tier": "frontend",
				},
			},
		}

		resources.RemoveLabel(cm, "env")

		g.Expect(cm.Labels).ToNot(HaveKey("env"))
		g.Expect(cm.Labels).To(HaveKeyWithValue("tier", "frontend"))
	})

	t.Run("handles removing non-existent label", func(t *testing.T) {
		g := NewWithT(t)

		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"env": "production",
				},
			},
		}

		resources.RemoveLabel(cm, "tier")

		g.Expect(cm.Labels).To(HaveKeyWithValue("env", "production"))
	})

	t.Run("handles nil labels map", func(t *testing.T) {
		g := NewWithT(t)

		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{},
		}

		// Should not panic
		resources.RemoveLabel(cm, "env")

		g.Expect(true).To(BeTrue())
	})

	t.Run("handles nil object", func(t *testing.T) {
		g := NewWithT(t)

		// Should not panic
		resources.RemoveLabel(nil, "env")

		g.Expect(true).To(BeTrue())
	})
}

func TestGetLabel(t *testing.T) {
	t.Run("returns label value when it exists", func(t *testing.T) {
		g := NewWithT(t)

		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"env": "production",
				},
			},
		}

		g.Expect(resources.GetLabel(cm, "env")).To(Equal("production"))
	})

	t.Run("returns empty string when label does not exist", func(t *testing.T) {
		g := NewWithT(t)

		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"env": "production",
				},
			},
		}

		g.Expect(resources.GetLabel(cm, "tier")).To(BeEmpty())
	})

	t.Run("returns empty string when labels map is nil", func(t *testing.T) {
		g := NewWithT(t)

		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{},
		}

		g.Expect(resources.GetLabel(cm, "env")).To(BeEmpty())
	})

	t.Run("returns empty string when object is nil", func(t *testing.T) {
		g := NewWithT(t)

		g.Expect(resources.GetLabel(nil, "env")).To(BeEmpty())
	})
}

func TestHasAnnotation(t *testing.T) {
	t.Run("returns true when annotation matches one of the values", func(t *testing.T) {
		g := NewWithT(t)

		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					"managed-by": "controller",
				},
			},
		}

		g.Expect(resources.HasAnnotation(cm, "managed-by", "controller", "operator")).To(BeTrue())
	})

	t.Run("returns false when annotation does not match any value", func(t *testing.T) {
		g := NewWithT(t)

		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					"managed-by": "controller",
				},
			},
		}

		g.Expect(resources.HasAnnotation(cm, "managed-by", "operator", "helm")).To(BeFalse())
	})

	t.Run("returns false when annotation key does not exist", func(t *testing.T) {
		g := NewWithT(t)

		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					"managed-by": "controller",
				},
			},
		}

		g.Expect(resources.HasAnnotation(cm, "owner", "team-a")).To(BeFalse())
	})

	t.Run("returns false when annotations map is nil", func(t *testing.T) {
		g := NewWithT(t)

		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{},
		}

		g.Expect(resources.HasAnnotation(cm, "managed-by", "controller")).To(BeFalse())
	})

	t.Run("returns false when object is nil", func(t *testing.T) {
		g := NewWithT(t)

		g.Expect(resources.HasAnnotation(nil, "managed-by", "controller")).To(BeFalse())
	})

	t.Run("returns false when values slice is empty", func(t *testing.T) {
		g := NewWithT(t)

		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					"managed-by": "controller",
				},
			},
		}

		g.Expect(resources.HasAnnotation(cm, "managed-by")).To(BeFalse())
	})
}

func TestSetAnnotations(t *testing.T) {
	t.Run("sets multiple annotations on object with no existing annotations", func(t *testing.T) {
		g := NewWithT(t)

		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{},
		}

		resources.SetAnnotations(cm, map[string]string{
			"managed-by": "controller",
			"version":    "v1.0.0",
		})

		g.Expect(cm.Annotations).To(HaveKeyWithValue("managed-by", "controller"))
		g.Expect(cm.Annotations).To(HaveKeyWithValue("version", "v1.0.0"))
	})

	t.Run("adds annotations to object with existing annotations", func(t *testing.T) {
		g := NewWithT(t)

		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					"app": "myapp",
				},
			},
		}

		resources.SetAnnotations(cm, map[string]string{
			"managed-by": "controller",
			"version":    "v1.0.0",
		})

		g.Expect(cm.Annotations).To(HaveKeyWithValue("app", "myapp"))
		g.Expect(cm.Annotations).To(HaveKeyWithValue("managed-by", "controller"))
		g.Expect(cm.Annotations).To(HaveKeyWithValue("version", "v1.0.0"))
	})

	t.Run("overwrites existing annotation values", func(t *testing.T) {
		g := NewWithT(t)

		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					"version": "v0.9.0",
				},
			},
		}

		resources.SetAnnotations(cm, map[string]string{
			"version": "v1.0.0",
		})

		g.Expect(cm.Annotations).To(HaveKeyWithValue("version", "v1.0.0"))
	})

	t.Run("handles nil object gracefully", func(t *testing.T) {
		g := NewWithT(t)

		// Should not panic
		resources.SetAnnotations(nil, map[string]string{"managed-by": "controller"})

		g.Expect(true).To(BeTrue())
	})
}

func TestSetAnnotation(t *testing.T) {
	t.Run("sets annotation on object with no existing annotations", func(t *testing.T) {
		g := NewWithT(t)

		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{},
		}

		old := resources.SetAnnotation(cm, "managed-by", "controller")

		g.Expect(cm.Annotations).To(HaveKeyWithValue("managed-by", "controller"))
		g.Expect(old).To(BeEmpty())
	})

	t.Run("adds annotation to object with existing annotations", func(t *testing.T) {
		g := NewWithT(t)

		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					"app": "myapp",
				},
			},
		}

		old := resources.SetAnnotation(cm, "managed-by", "controller")

		g.Expect(cm.Annotations).To(HaveKeyWithValue("app", "myapp"))
		g.Expect(cm.Annotations).To(HaveKeyWithValue("managed-by", "controller"))
		g.Expect(old).To(BeEmpty())
	})

	t.Run("returns old value when overwriting existing annotation", func(t *testing.T) {
		g := NewWithT(t)

		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					"version": "v0.9.0",
				},
			},
		}

		old := resources.SetAnnotation(cm, "version", "v1.0.0")

		g.Expect(cm.Annotations).To(HaveKeyWithValue("version", "v1.0.0"))
		g.Expect(old).To(Equal("v0.9.0"))
	})

	t.Run("returns empty string for nil object", func(t *testing.T) {
		g := NewWithT(t)

		old := resources.SetAnnotation(nil, "managed-by", "controller")

		g.Expect(old).To(BeEmpty())
	})
}

func TestRemoveAnnotation(t *testing.T) {
	t.Run("removes existing annotation", func(t *testing.T) {
		g := NewWithT(t)

		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					"managed-by": "controller",
					"version":    "v1.0.0",
				},
			},
		}

		resources.RemoveAnnotation(cm, "managed-by")

		g.Expect(cm.Annotations).ToNot(HaveKey("managed-by"))
		g.Expect(cm.Annotations).To(HaveKeyWithValue("version", "v1.0.0"))
	})

	t.Run("handles removing non-existent annotation", func(t *testing.T) {
		g := NewWithT(t)

		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					"managed-by": "controller",
				},
			},
		}

		resources.RemoveAnnotation(cm, "version")

		g.Expect(cm.Annotations).To(HaveKeyWithValue("managed-by", "controller"))
	})

	t.Run("handles nil annotations map", func(t *testing.T) {
		g := NewWithT(t)

		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{},
		}

		// Should not panic
		resources.RemoveAnnotation(cm, "managed-by")

		g.Expect(true).To(BeTrue())
	})

	t.Run("handles nil object", func(t *testing.T) {
		g := NewWithT(t)

		// Should not panic
		resources.RemoveAnnotation(nil, "managed-by")

		g.Expect(true).To(BeTrue())
	})
}

func TestGetAnnotation(t *testing.T) {
	t.Run("returns annotation value when it exists", func(t *testing.T) {
		g := NewWithT(t)

		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					"managed-by": "controller",
				},
			},
		}

		g.Expect(resources.GetAnnotation(cm, "managed-by")).To(Equal("controller"))
	})

	t.Run("returns empty string when annotation does not exist", func(t *testing.T) {
		g := NewWithT(t)

		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					"managed-by": "controller",
				},
			},
		}

		g.Expect(resources.GetAnnotation(cm, "version")).To(BeEmpty())
	})

	t.Run("returns empty string when annotations map is nil", func(t *testing.T) {
		g := NewWithT(t)

		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{},
		}

		g.Expect(resources.GetAnnotation(cm, "managed-by")).To(BeEmpty())
	})

	t.Run("returns empty string when object is nil", func(t *testing.T) {
		g := NewWithT(t)

		g.Expect(resources.GetAnnotation(nil, "managed-by")).To(BeEmpty())
	})
}
