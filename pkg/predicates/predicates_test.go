package predicates_test

import (
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/predicates"

	. "github.com/onsi/gomega"
)

type testObject struct {
	metav1.TypeMeta
	metav1.ObjectMeta
}

func (t *testObject) DeepCopyObject() runtime.Object {
	return &testObject{
		TypeMeta:   t.TypeMeta,
		ObjectMeta: *t.DeepCopy(),
	}
}

func (t *testObject) GetObjectKind() schema.ObjectKind {
	return t
}

func TestGenerationChanged(t *testing.T) {
	pred := predicates.GenerationChanged()

	t.Run("returns false for create events", func(t *testing.T) {
		g := NewWithT(t)
		obj := &testObject{ObjectMeta: metav1.ObjectMeta{Generation: 1}}

		result := pred.Create(event.CreateEvent{Object: obj})

		g.Expect(result).To(BeFalse())
	})

	t.Run("returns false for delete events", func(t *testing.T) {
		g := NewWithT(t)
		obj := &testObject{ObjectMeta: metav1.ObjectMeta{Generation: 1}}

		result := pred.Delete(event.DeleteEvent{Object: obj})

		g.Expect(result).To(BeFalse())
	})

	t.Run("returns true when generation changes", func(t *testing.T) {
		g := NewWithT(t)
		oldObj := &testObject{ObjectMeta: metav1.ObjectMeta{Generation: 1}}
		newObj := &testObject{ObjectMeta: metav1.ObjectMeta{Generation: 2}}

		result := pred.Update(event.UpdateEvent{
			ObjectOld: oldObj,
			ObjectNew: newObj,
		})

		g.Expect(result).To(BeTrue())
	})

	t.Run("returns false when generation unchanged", func(t *testing.T) {
		g := NewWithT(t)
		oldObj := &testObject{ObjectMeta: metav1.ObjectMeta{Generation: 1}}
		newObj := &testObject{ObjectMeta: metav1.ObjectMeta{Generation: 1}}

		result := pred.Update(event.UpdateEvent{
			ObjectOld: oldObj,
			ObjectNew: newObj,
		})

		g.Expect(result).To(BeFalse())
	})

	t.Run("returns true when new object has zero generation", func(t *testing.T) {
		g := NewWithT(t)
		oldObj := &testObject{ObjectMeta: metav1.ObjectMeta{Generation: 0}}
		newObj := &testObject{ObjectMeta: metav1.ObjectMeta{Generation: 0}}

		result := pred.Update(event.UpdateEvent{
			ObjectOld: oldObj,
			ObjectNew: newObj,
		})

		g.Expect(result).To(BeTrue())
	})

	t.Run("returns true when old object has zero generation", func(t *testing.T) {
		g := NewWithT(t)
		oldObj := &testObject{ObjectMeta: metav1.ObjectMeta{Generation: 0}}
		newObj := &testObject{ObjectMeta: metav1.ObjectMeta{Generation: 1}}

		result := pred.Update(event.UpdateEvent{
			ObjectOld: oldObj,
			ObjectNew: newObj,
		})

		g.Expect(result).To(BeTrue())
	})

	t.Run("returns true when old object is nil", func(t *testing.T) {
		g := NewWithT(t)
		newObj := &testObject{ObjectMeta: metav1.ObjectMeta{Generation: 1}}

		result := pred.Update(event.UpdateEvent{
			ObjectOld: nil,
			ObjectNew: newObj,
		})

		g.Expect(result).To(BeTrue())
	})

	t.Run("returns true when new object is nil", func(t *testing.T) {
		g := NewWithT(t)
		oldObj := &testObject{ObjectMeta: metav1.ObjectMeta{Generation: 1}}

		result := pred.Update(event.UpdateEvent{
			ObjectOld: oldObj,
			ObjectNew: nil,
		})

		g.Expect(result).To(BeTrue())
	})
}

func TestCreated(t *testing.T) {
	pred := predicates.Created()

	t.Run("returns true for create events", func(t *testing.T) {
		g := NewWithT(t)
		obj := &testObject{ObjectMeta: metav1.ObjectMeta{Generation: 1}}

		result := pred.Create(event.CreateEvent{Object: obj})

		g.Expect(result).To(BeTrue())
	})

	t.Run("returns false for delete events", func(t *testing.T) {
		g := NewWithT(t)
		obj := &testObject{ObjectMeta: metav1.ObjectMeta{Generation: 1}}

		result := pred.Delete(event.DeleteEvent{Object: obj})

		g.Expect(result).To(BeFalse())
	})

	t.Run("returns false for update events", func(t *testing.T) {
		g := NewWithT(t)
		oldObj := &testObject{ObjectMeta: metav1.ObjectMeta{Generation: 1}}
		newObj := &testObject{ObjectMeta: metav1.ObjectMeta{Generation: 2}}

		result := pred.Update(event.UpdateEvent{
			ObjectOld: oldObj,
			ObjectNew: newObj,
		})

		g.Expect(result).To(BeFalse())
	})
}

func TestDeleted(t *testing.T) {
	pred := predicates.Deleted()

	t.Run("returns false for create events", func(t *testing.T) {
		g := NewWithT(t)
		obj := &testObject{ObjectMeta: metav1.ObjectMeta{Generation: 1}}

		result := pred.Create(event.CreateEvent{Object: obj})

		g.Expect(result).To(BeFalse())
	})

	t.Run("returns true for delete events", func(t *testing.T) {
		g := NewWithT(t)
		obj := &testObject{ObjectMeta: metav1.ObjectMeta{Generation: 1}}

		result := pred.Delete(event.DeleteEvent{Object: obj})

		g.Expect(result).To(BeTrue())
	})

	t.Run("returns false for update events", func(t *testing.T) {
		g := NewWithT(t)
		oldObj := &testObject{ObjectMeta: metav1.ObjectMeta{Generation: 1}}
		newObj := &testObject{ObjectMeta: metav1.ObjectMeta{Generation: 2}}

		result := pred.Update(event.UpdateEvent{
			ObjectOld: oldObj,
			ObjectNew: newObj,
		})

		g.Expect(result).To(BeFalse())
	})
}

func TestUpdated(t *testing.T) {
	// Custom comparison function that returns true if name changed
	pred := predicates.Updated(func(oldObj, newObj client.Object) bool {
		return oldObj.GetName() != newObj.GetName()
	})

	t.Run("returns false for create events", func(t *testing.T) {
		g := NewWithT(t)
		obj := &testObject{ObjectMeta: metav1.ObjectMeta{Name: "test"}}

		result := pred.Create(event.CreateEvent{Object: obj})

		g.Expect(result).To(BeFalse())
	})

	t.Run("returns false for delete events", func(t *testing.T) {
		g := NewWithT(t)
		obj := &testObject{ObjectMeta: metav1.ObjectMeta{Name: "test"}}

		result := pred.Delete(event.DeleteEvent{Object: obj})

		g.Expect(result).To(BeFalse())
	})

	t.Run("returns true when comparison function returns true", func(t *testing.T) {
		g := NewWithT(t)
		oldObj := &testObject{ObjectMeta: metav1.ObjectMeta{Name: "old"}}
		newObj := &testObject{ObjectMeta: metav1.ObjectMeta{Name: "new"}}

		result := pred.Update(event.UpdateEvent{
			ObjectOld: oldObj,
			ObjectNew: newObj,
		})

		g.Expect(result).To(BeTrue())
	})

	t.Run("returns false when comparison function returns false", func(t *testing.T) {
		g := NewWithT(t)
		oldObj := &testObject{ObjectMeta: metav1.ObjectMeta{Name: "same"}}
		newObj := &testObject{ObjectMeta: metav1.ObjectMeta{Name: "same"}}

		result := pred.Update(event.UpdateEvent{
			ObjectOld: oldObj,
			ObjectNew: newObj,
		})

		g.Expect(result).To(BeFalse())
	})

	t.Run("returns true when old object is nil", func(t *testing.T) {
		g := NewWithT(t)
		newObj := &testObject{ObjectMeta: metav1.ObjectMeta{Name: "test"}}

		result := pred.Update(event.UpdateEvent{
			ObjectOld: nil,
			ObjectNew: newObj,
		})

		g.Expect(result).To(BeTrue())
	})

	t.Run("returns true when new object is nil", func(t *testing.T) {
		g := NewWithT(t)
		oldObj := &testObject{ObjectMeta: metav1.ObjectMeta{Name: "test"}}

		result := pred.Update(event.UpdateEvent{
			ObjectOld: oldObj,
			ObjectNew: nil,
		})

		g.Expect(result).To(BeTrue())
	})

	t.Run("returns false when both objects are nil", func(t *testing.T) {
		g := NewWithT(t)
		result := pred.Update(event.UpdateEvent{
			ObjectOld: nil,
			ObjectNew: nil,
		})

		g.Expect(result).To(BeFalse())
	})
}

func TestResourceVersionChanged(t *testing.T) {
	pred := predicates.ResourceVersionChanged()

	t.Run("returns false for create events", func(t *testing.T) {
		g := NewWithT(t)
		obj := &testObject{ObjectMeta: metav1.ObjectMeta{ResourceVersion: "1"}}

		result := pred.Create(event.CreateEvent{Object: obj})

		g.Expect(result).To(BeFalse())
	})

	t.Run("returns false for delete events", func(t *testing.T) {
		g := NewWithT(t)
		obj := &testObject{ObjectMeta: metav1.ObjectMeta{ResourceVersion: "1"}}

		result := pred.Delete(event.DeleteEvent{Object: obj})

		g.Expect(result).To(BeFalse())
	})

	t.Run("returns true when resource version changes", func(t *testing.T) {
		g := NewWithT(t)
		oldObj := &testObject{ObjectMeta: metav1.ObjectMeta{ResourceVersion: "1"}}
		newObj := &testObject{ObjectMeta: metav1.ObjectMeta{ResourceVersion: "2"}}

		result := pred.Update(event.UpdateEvent{
			ObjectOld: oldObj,
			ObjectNew: newObj,
		})

		g.Expect(result).To(BeTrue())
	})

	t.Run("returns false when resource version unchanged", func(t *testing.T) {
		g := NewWithT(t)
		oldObj := &testObject{ObjectMeta: metav1.ObjectMeta{ResourceVersion: "1"}}
		newObj := &testObject{ObjectMeta: metav1.ObjectMeta{ResourceVersion: "1"}}

		result := pred.Update(event.UpdateEvent{
			ObjectOld: oldObj,
			ObjectNew: newObj,
		})

		g.Expect(result).To(BeFalse())
	})
}

func TestAndOr(t *testing.T) {
	t.Run("Or returns true if any predicate is true", func(t *testing.T) {
		g := NewWithT(t)
		pred := predicates.Or(
			predicates.Created(),
			predicates.Deleted(),
		)

		obj := &testObject{ObjectMeta: metav1.ObjectMeta{Name: "test"}}

		createResult := pred.Create(event.CreateEvent{Object: obj})
		g.Expect(createResult).To(BeTrue())

		deleteResult := pred.Delete(event.DeleteEvent{Object: obj})
		g.Expect(deleteResult).To(BeTrue())
	})

	t.Run("And returns true only if all predicates are true", func(t *testing.T) {
		g := NewWithT(t)
		pred := predicates.And(
			predicates.Updated(func(_, _ client.Object) bool {
				return true // always true
			}),
			predicates.GenerationChanged(),
		)

		oldObj := &testObject{ObjectMeta: metav1.ObjectMeta{Generation: 1}}
		newObj := &testObject{ObjectMeta: metav1.ObjectMeta{Generation: 2}}

		result := pred.Update(event.UpdateEvent{
			ObjectOld: oldObj,
			ObjectNew: newObj,
		})

		g.Expect(result).To(BeTrue())
	})
}

func TestDefault(t *testing.T) {
	g := NewWithT(t)

	g.Expect(predicates.Default()).ToNot(BeNil())
}
