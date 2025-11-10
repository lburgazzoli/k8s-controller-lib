package ownership_test

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/resources/ownership"

	. "github.com/onsi/gomega"
)

func TestSetOwner(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(scheme)).To(Succeed())

	owner := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "owner",
			Namespace: "default",
			UID:       types.UID("owner-uid"),
		},
	}

	owned := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "owned",
			Namespace: "default",
		},
	}

	err := ownership.SetOwner(scheme, owner, owned)
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(owned.GetOwnerReferences()).To(HaveLen(1))
	g.Expect(owned.GetOwnerReferences()[0].UID).To(Equal(owner.GetUID()))
	g.Expect(owned.GetOwnerReferences()[0].Name).To(Equal("owner"))
}

func TestSetOwner_WithController(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(scheme)).To(Succeed())

	owner := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "owner",
			Namespace: "default",
			UID:       types.UID("owner-uid"),
		},
	}

	owned := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "owned",
			Namespace: "default",
		},
	}

	err := ownership.SetOwner(scheme, owner, owned, ownership.WithController())
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(owned.GetOwnerReferences()).To(HaveLen(1))
	g.Expect(owned.GetOwnerReferences()[0].Controller).ToNot(BeNil())
	g.Expect(*owned.GetOwnerReferences()[0].Controller).To(BeTrue())
}

func TestSetOwner_WithBlockDeletion(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(scheme)).To(Succeed())

	owner := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "owner",
			Namespace: "default",
			UID:       types.UID("owner-uid"),
		},
	}

	owned := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "owned",
			Namespace: "default",
		},
	}

	err := ownership.SetOwner(scheme, owner, owned, ownership.WithBlockDeletion())
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(owned.GetOwnerReferences()).To(HaveLen(1))
	g.Expect(owned.GetOwnerReferences()[0].BlockOwnerDeletion).ToNot(BeNil())
	g.Expect(*owned.GetOwnerReferences()[0].BlockOwnerDeletion).To(BeTrue())
}

func TestSetOwner_WithBothOptions(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(scheme)).To(Succeed())

	owner := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "owner",
			Namespace: "default",
			UID:       types.UID("owner-uid"),
		},
	}

	owned := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "owned",
			Namespace: "default",
		},
	}

	err := ownership.SetOwner(scheme, owner, owned,
		ownership.WithController(),
		ownership.WithBlockDeletion())
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(owned.GetOwnerReferences()).To(HaveLen(1))
	g.Expect(owned.GetOwnerReferences()[0].Controller).ToNot(BeNil())
	g.Expect(*owned.GetOwnerReferences()[0].Controller).To(BeTrue())
	g.Expect(owned.GetOwnerReferences()[0].BlockOwnerDeletion).ToNot(BeNil())
	g.Expect(*owned.GetOwnerReferences()[0].BlockOwnerDeletion).To(BeTrue())
}

func TestSetOwner_NamespacedOwnerDifferentNamespace(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(scheme)).To(Succeed())

	owner := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "owner",
			Namespace: "namespace1",
			UID:       types.UID("owner-uid"),
		},
	}

	owned := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "owned",
			Namespace: "namespace2",
		},
	}

	err := ownership.SetOwner(scheme, owner, owned)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("namespaced owner cannot own resource in different namespace"))
}

func TestSetOwner_ClusterScopedOwner(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(scheme)).To(Succeed())

	owner := &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Namespace",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-namespace",
			UID:  types.UID("namespace-uid"),
		},
	}

	owned := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "owned",
			Namespace: "test-namespace",
		},
	}

	err := ownership.SetOwner(scheme, owner, owned)
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(owned.GetOwnerReferences()).To(HaveLen(1))
	g.Expect(owned.GetOwnerReferences()[0].UID).To(Equal(owner.GetUID()))
	g.Expect(owned.GetOwnerReferences()[0].Name).To(Equal("test-namespace"))
	g.Expect(owned.GetOwnerReferences()[0].Kind).To(Equal("Namespace"))
}

func TestIsOwnedBy(t *testing.T) {
	g := NewWithT(t)

	owner := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			UID: types.UID("owner-uid"),
		},
	}

	owned := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			OwnerReferences: []metav1.OwnerReference{
				{
					UID: types.UID("owner-uid"),
				},
			},
		},
	}

	g.Expect(ownership.IsOwnedBy(owner, owned)).To(BeTrue())
}

func TestIsOwnedBy_NotOwned(t *testing.T) {
	g := NewWithT(t)

	owner := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			UID: types.UID("owner-uid"),
		},
	}

	owned := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			OwnerReferences: []metav1.OwnerReference{
				{
					UID: types.UID("different-uid"),
				},
			},
		},
	}

	g.Expect(ownership.IsOwnedBy(owner, owned)).To(BeFalse())
}

func TestIsController(t *testing.T) {
	g := NewWithT(t)

	owner := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			UID: types.UID("owner-uid"),
		},
	}

	controller := true
	owned := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			OwnerReferences: []metav1.OwnerReference{
				{
					UID:        types.UID("owner-uid"),
					Controller: &controller,
				},
			},
		},
	}

	g.Expect(ownership.IsController(owner, owned)).To(BeTrue())
}

func TestIsController_NotController(t *testing.T) {
	g := NewWithT(t)

	owner := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			UID: types.UID("owner-uid"),
		},
	}

	controller := false
	owned := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			OwnerReferences: []metav1.OwnerReference{
				{
					UID:        types.UID("owner-uid"),
					Controller: &controller,
				},
			},
		},
	}

	g.Expect(ownership.IsController(owner, owned)).To(BeFalse())
}

func TestRemoveOwner(t *testing.T) {
	g := NewWithT(t)

	owner := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			UID: types.UID("owner-uid"),
		},
	}

	owned := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			OwnerReferences: []metav1.OwnerReference{
				{
					UID:  types.UID("owner-uid"),
					Name: "owner",
				},
				{
					UID:  types.UID("other-uid"),
					Name: "other",
				},
			},
		},
	}

	removed := ownership.RemoveOwner(owner, owned)

	g.Expect(removed).To(BeTrue())
	g.Expect(owned.GetOwnerReferences()).To(HaveLen(1))
	g.Expect(owned.GetOwnerReferences()[0].UID).To(Equal(types.UID("other-uid")))
}

func TestRemoveOwner_NotFound(t *testing.T) {
	g := NewWithT(t)

	owner := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			UID: types.UID("owner-uid"),
		},
	}

	owned := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			OwnerReferences: []metav1.OwnerReference{
				{
					UID:  types.UID("different-uid"),
					Name: "different",
				},
			},
		},
	}

	removed := ownership.RemoveOwner(owner, owned)

	g.Expect(removed).To(BeFalse())
	g.Expect(owned.GetOwnerReferences()).To(HaveLen(1))
}

func TestGetOwnerReference(t *testing.T) {
	g := NewWithT(t)

	owner := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			UID: types.UID("owner-uid"),
		},
	}

	owned := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			OwnerReferences: []metav1.OwnerReference{
				{
					UID:  types.UID("owner-uid"),
					Name: "owner",
				},
			},
		},
	}

	ref := ownership.GetOwnerReference(owner, owned)

	g.Expect(ref).ToNot(BeNil())
	g.Expect(ref.UID).To(Equal(types.UID("owner-uid")))
	g.Expect(ref.Name).To(Equal("owner"))
}

func TestGetOwnerReference_NotFound(t *testing.T) {
	g := NewWithT(t)

	owner := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			UID: types.UID("owner-uid"),
		},
	}

	owned := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			OwnerReferences: []metav1.OwnerReference{
				{
					UID:  types.UID("different-uid"),
					Name: "different",
				},
			},
		},
	}

	ref := ownership.GetOwnerReference(owner, owned)

	g.Expect(ref).To(BeNil())
}
