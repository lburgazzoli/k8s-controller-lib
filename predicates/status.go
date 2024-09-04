package predicates

import (
	"reflect"

	"k8s.io/apimachinery/pkg/api/equality"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

var _ predicate.Predicate = StatusChanged{}

// StatusChanged implements a generic update predicate function on status change.
type StatusChanged struct {
	predicate.Funcs
}

func (p StatusChanged) Create(_ event.CreateEvent) bool {
	return false
}

func (p StatusChanged) Generic(_ event.GenericEvent) bool {
	return false
}

func (p StatusChanged) Delete(_ event.DeleteEvent) bool {
	return false
}

// Update implements default UpdateEvent filter for validating status change.
func (p StatusChanged) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil {
		return false
	}

	if e.ObjectNew == nil {
		return false
	}

	s1 := reflect.ValueOf(e.ObjectOld).Elem().FieldByName("Status")
	if !s1.IsValid() {
		return false
	}

	s2 := reflect.ValueOf(e.ObjectNew).Elem().FieldByName("Status")
	if !s2.IsValid() {
		return false
	}

	return !equality.Semantic.DeepEqual(s1.Interface(), s2.Interface())
}
