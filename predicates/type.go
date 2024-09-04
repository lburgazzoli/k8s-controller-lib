package predicates

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type Test struct {
	predicate.Funcs
	F func(obj client.Object) bool
}

func (p Test) Create(e event.CreateEvent) bool {
	return p.test(e.Object)
}

func (p Test) Generic(e event.GenericEvent) bool {
	return p.test(e.Object)
}

func (p Test) Delete(e event.DeleteEvent) bool {
	return p.test(e.Object)
}

func (p Test) Update(e event.UpdateEvent) bool {
	return p.test(e.ObjectNew)
}

func (p Test) test(obj client.Object) bool {
	if obj == nil {
		return false
	}

	return p.F(obj)
}

type Compare struct {
	predicate.Funcs
	F func(oldObj client.Object, newObj client.Object) bool
}

func (p Compare) Create(_ event.CreateEvent) bool {
	return false
}

func (p Compare) Generic(_ event.GenericEvent) bool {
	return false
}

func (p Compare) Delete(_ event.DeleteEvent) bool {
	return false
}

func (p Compare) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil {
		return false
	}

	if e.ObjectNew == nil {
		return false
	}

	return p.F(e.ObjectOld, e.ObjectNew)
}
