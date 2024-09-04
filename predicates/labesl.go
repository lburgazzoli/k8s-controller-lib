package predicates

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

var _ predicate.Predicate = AnnotationChanged{}
var _ predicate.Predicate = HasLabel{}

type LabelChanged struct {
	predicate.Funcs
	Name string
}

func (p LabelChanged) Create(_ event.CreateEvent) bool {
	return false
}

func (p LabelChanged) Generic(_ event.GenericEvent) bool {
	return false
}

func (p LabelChanged) Delete(_ event.DeleteEvent) bool {
	return false
}

func (p LabelChanged) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil {
		return false
	}

	if e.ObjectNew == nil {
		return false
	}

	return e.ObjectOld.GetLabels()[p.Name] != e.ObjectNew.GetLabels()[p.Name]
}

type HasLabel struct {
	predicate.Funcs
	Name string
}

func (p HasLabel) Create(_ event.CreateEvent) bool {
	return false
}

func (p HasLabel) Generic(_ event.GenericEvent) bool {
	return false
}

func (p HasLabel) Delete(e event.DeleteEvent) bool {
	return p.test(e.Object)
}

func (p HasLabel) Update(e event.UpdateEvent) bool {
	return p.test(e.ObjectNew)
}

func (p HasLabel) test(obj client.Object) bool {
	if obj == nil {
		return false
	}

	_, ok := obj.GetLabels()[p.Name]

	return ok
}
