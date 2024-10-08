package predicates

import (
	"reflect"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/apimachinery/pkg/api/equality"
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

func StatusChanged() predicate.Predicate {
	return Compare{
		F: func(oldObj client.Object, newObj client.Object) bool {
			s1 := reflect.ValueOf(oldObj).Elem().FieldByName("Status")
			if !s1.IsValid() {
				return false
			}

			s2 := reflect.ValueOf(newObj).Elem().FieldByName("Status")
			if !s2.IsValid() {
				return false
			}

			return !equality.Semantic.DeepEqual(s1.Interface(), s2.Interface())
		},
	}
}
