package predicates_test

import (
	"testing"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/predicates"
	"sigs.k8s.io/controller-runtime/pkg/event"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

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
	g := NewWithT(t)

	pred := predicates.GenerationChanged

	t.Run("returns false for create events", func(t *testing.T) {
		obj := &testObject{ObjectMeta: metav1.ObjectMeta{Generation: 1}}

		result := pred.Create(event.CreateEvent{Object: obj})

		g.Expect(result).To(BeFalse())
	})

	t.Run("returns true for delete events", func(t *testing.T) {
		obj := &testObject{ObjectMeta: metav1.ObjectMeta{Generation: 1}}

		result := pred.Delete(event.DeleteEvent{Object: obj})

		g.Expect(result).To(BeTrue())
	})

	t.Run("returns true when generation changes", func(t *testing.T) {
		oldObj := &testObject{ObjectMeta: metav1.ObjectMeta{Generation: 1}}
		newObj := &testObject{ObjectMeta: metav1.ObjectMeta{Generation: 2}}

		result := pred.Update(event.UpdateEvent{
			ObjectOld: oldObj,
			ObjectNew: newObj,
		})

		g.Expect(result).To(BeTrue())
	})

	t.Run("returns false when generation unchanged", func(t *testing.T) {
		oldObj := &testObject{ObjectMeta: metav1.ObjectMeta{Generation: 1}}
		newObj := &testObject{ObjectMeta: metav1.ObjectMeta{Generation: 1}}

		result := pred.Update(event.UpdateEvent{
			ObjectOld: oldObj,
			ObjectNew: newObj,
		})

		g.Expect(result).To(BeFalse())
	})

	t.Run("returns true when new object has zero generation", func(t *testing.T) {
		oldObj := &testObject{ObjectMeta: metav1.ObjectMeta{Generation: 0}}
		newObj := &testObject{ObjectMeta: metav1.ObjectMeta{Generation: 0}}

		result := pred.Update(event.UpdateEvent{
			ObjectOld: oldObj,
			ObjectNew: newObj,
		})

		g.Expect(result).To(BeTrue())
	})

	t.Run("returns true when old object has zero generation", func(t *testing.T) {
		oldObj := &testObject{ObjectMeta: metav1.ObjectMeta{Generation: 0}}
		newObj := &testObject{ObjectMeta: metav1.ObjectMeta{Generation: 1}}

		result := pred.Update(event.UpdateEvent{
			ObjectOld: oldObj,
			ObjectNew: newObj,
		})

		g.Expect(result).To(BeTrue())
	})

	t.Run("returns false when old object is nil", func(t *testing.T) {
		newObj := &testObject{ObjectMeta: metav1.ObjectMeta{Generation: 1}}

		result := pred.Update(event.UpdateEvent{
			ObjectOld: nil,
			ObjectNew: newObj,
		})

		g.Expect(result).To(BeFalse())
	})

	t.Run("returns false when new object is nil", func(t *testing.T) {
		oldObj := &testObject{ObjectMeta: metav1.ObjectMeta{Generation: 1}}

		result := pred.Update(event.UpdateEvent{
			ObjectOld: oldObj,
			ObjectNew: nil,
		})

		g.Expect(result).To(BeFalse())
	})
}

func TestDefault(t *testing.T) {
	g := NewWithT(t)

	g.Expect(predicates.Default).ToNot(BeNil())
}
