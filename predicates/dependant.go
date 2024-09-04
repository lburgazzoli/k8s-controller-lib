package predicates

import (
	"reflect"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

var _ predicate.Predicate = DependentPredicate{}

type DependentPredicateOption func(*DependentPredicate) *DependentPredicate

func WithWatchDeleted(val bool) DependentPredicateOption {
	return func(in *DependentPredicate) *DependentPredicate {
		in.WatchDelete = val
		return in
	}
}

func WithWatchUpdate(val bool) DependentPredicateOption {
	return func(in *DependentPredicate) *DependentPredicate {
		in.WatchUpdate = val
		return in
	}
}

func WithWatchStatus(val bool) DependentPredicateOption {
	return func(in *DependentPredicate) *DependentPredicate {
		in.WatchStatus = val
		return in
	}
}

type DependentPredicate struct {
	WatchDelete bool
	WatchUpdate bool
	WatchStatus bool

	predicate.Funcs
}

func (p DependentPredicate) Create(_ event.CreateEvent) bool {
	return false
}

func (p DependentPredicate) Generic(_ event.GenericEvent) bool {
	return false
}

func (p DependentPredicate) Delete(e event.DeleteEvent) bool {
	return p.WatchDelete
}

func (p DependentPredicate) Update(e event.UpdateEvent) bool {
	if !p.WatchUpdate {
		return false
	}

	if e.ObjectOld.GetResourceVersion() == e.ObjectNew.GetResourceVersion() {
		return false
	}

	oldObj, ok := e.ObjectOld.(*unstructured.Unstructured)
	if !ok {
		return false
	}

	newObj, ok := e.ObjectNew.(*unstructured.Unstructured)
	if !ok {
		return false
	}

	oldObj = oldObj.DeepCopy()
	newObj = newObj.DeepCopy()

	if !p.WatchStatus {
		// Update filters out events that change only the dependent resource
		// status. It is not typical for the controller of a primary
		// resource to write to the status of one its dependent resources.
		delete(oldObj.Object, "status")
		delete(newObj.Object, "status")
	}

	// Reset field not meaningful for comparison
	oldObj.SetResourceVersion("")
	newObj.SetResourceVersion("")
	oldObj.SetManagedFields(nil)
	newObj.SetManagedFields(nil)

	return !reflect.DeepEqual(oldObj.Object, newObj.Object)
}
