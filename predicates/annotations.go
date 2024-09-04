//nolint:dupl
package predicates

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

var _ predicate.Predicate = AnnotationChanged{}
var _ predicate.Predicate = HasAnnotation{}

type AnnotationChanged struct {
	predicate.Funcs
	Name string
}

func (p AnnotationChanged) Create(_ event.CreateEvent) bool {
	return false
}

func (p AnnotationChanged) Generic(_ event.GenericEvent) bool {
	return false
}

func (p AnnotationChanged) Delete(_ event.DeleteEvent) bool {
	return false
}

func (p AnnotationChanged) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil {
		return false
	}

	if e.ObjectNew == nil {
		return false
	}

	return e.ObjectOld.GetAnnotations()[p.Name] != e.ObjectNew.GetAnnotations()[p.Name]
}

type HasAnnotation struct {
	predicate.Funcs
	Name string
}

func (p HasAnnotation) Create(e event.CreateEvent) bool {
	return p.test(e.Object)
}

func (p HasAnnotation) Generic(e event.GenericEvent) bool {
	return p.test(e.Object)
}

func (p HasAnnotation) Delete(e event.DeleteEvent) bool {
	return p.test(e.Object)
}

func (p HasAnnotation) Update(e event.UpdateEvent) bool {
	return p.test(e.ObjectNew)
}

func (p HasAnnotation) test(obj client.Object) bool {
	if obj == nil {
		return false
	}

	_, ok := obj.GetAnnotations()[p.Name]

	return ok
}
