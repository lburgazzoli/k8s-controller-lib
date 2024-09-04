package predicates

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

func AnnotationChanged(name string) predicate.Predicate {
	return Compare{
		F: func(oldObj client.Object, newObj client.Object) bool {
			return oldObj.GetAnnotations()[name] != newObj.GetAnnotations()[name]
		},
	}
}

func HasAnnotation(name string) predicate.Predicate {
	return Test{
		F: func(obj client.Object) bool {
			_, ok := obj.GetAnnotations()[name]
			return ok
		},
	}
}

func LabelChanged(name string) predicate.Predicate {
	return Compare{
		F: func(oldObj client.Object, newObj client.Object) bool {
			return oldObj.GetLabels()[name] != newObj.GetLabels()[name]
		},
	}
}

func HasLabel(name string) predicate.Predicate {
	return Test{
		F: func(obj client.Object) bool {
			_, ok := obj.GetLabels()[name]
			return ok
		},
	}
}
