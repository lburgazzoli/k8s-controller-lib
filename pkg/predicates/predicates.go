package predicates

import (
	"maps"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// GenerationChanged triggers reconciliation when the object's generation changes.
// This predicate handles objects where generation might be zero (e.g., ConfigMaps, Secrets),
// treating them as always changed to ensure events are processed.
//
// For resources with generation support, only spec changes trigger reconciliation.
// Status-only updates are ignored unless the resource doesn't use subresource status.
//
// Behavior:
// - Create: Returns false
// - Delete: Returns false (use Deleted() or Default() to handle deletions)
// - Update: Returns true if generation changed or either object has zero generation.
func GenerationChanged() predicate.Predicate {
	return Updated(func(oldObj client.Object, newObj client.Object) bool {
		// Zero generation means the resource doesn't track generation (e.g., ConfigMap, Secret).
		// Process all updates for such resources to avoid missing changes.
		if newObj.GetGeneration() == 0 || oldObj.GetGeneration() == 0 {
			return true
		}

		return newObj.GetGeneration() != oldObj.GetGeneration()
	})
}

// LabelChanged triggers reconciliation when object labels change.
//
// Behavior:
// - Create: Returns false
// - Delete: Returns false (use Deleted() or Default() to handle deletions)
// - Update: Returns true if labels changed.
func LabelChanged() predicate.Predicate {
	return Updated(func(oldObj client.Object, newObj client.Object) bool {
		return !maps.Equal(newObj.GetLabels(), oldObj.GetLabels())
	})
}

// AnnotationChanged triggers reconciliation when object annotations change.
//
// Behavior:
// - Create: Returns false
// - Delete: Returns false (use Deleted() or Default() to handle deletions)
// - Update: Returns true if annotations changed.
func AnnotationChanged() predicate.Predicate {
	return Updated(func(oldObj client.Object, newObj client.Object) bool {
		return !maps.Equal(newObj.GetAnnotations(), oldObj.GetAnnotations())
	})
}

// ResourceVersionChanged triggers reconciliation when the object's resource version changes.
//
// Behavior:
// - Create: Returns false
// - Delete: Returns false (use Deleted() or Default() to handle deletions)
// - Update: Returns true if resource version changed.
func ResourceVersionChanged() predicate.Predicate {
	return Updated(func(oldObj client.Object, newObj client.Object) bool {
		return newObj.GetResourceVersion() != oldObj.GetResourceVersion()
	})
}

// Updated returns a predicate that triggers only on Update events where the comparison function returns
// true or when exactly one of the objects is nil.
//
// Nil handling:
// - Both nil: returns false (invalid state)
// - One nil: returns true (trigger reconciliation for safety)
// - Neither nil: calls comparison function
//
// The comparison function receives the old and new objects and should return true if reconciliation is needed.
func Updated(compare func(oldObj client.Object, newObj client.Object) bool) predicate.Predicate {
	return predicate.Funcs{
		GenericFunc: func(_ event.GenericEvent) bool {
			return false
		},
		CreateFunc: func(_ event.CreateEvent) bool {
			return false
		},
		DeleteFunc: func(_ event.DeleteEvent) bool {
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			switch {
			case e.ObjectOld == nil && e.ObjectNew == nil:
				return false
			case e.ObjectOld == nil || e.ObjectNew == nil:
				return true
			default:
				return compare(e.ObjectOld, e.ObjectNew)
			}
		},
	}
}

// Created returns a predicate that triggers only on Create events.
func Created() predicate.Predicate {
	return predicate.Funcs{
		GenericFunc: func(_ event.GenericEvent) bool {
			return false
		},
		CreateFunc: func(_ event.CreateEvent) bool {
			return true
		},
		DeleteFunc: func(_ event.DeleteEvent) bool {
			return false
		},
		UpdateFunc: func(_ event.UpdateEvent) bool {
			return false
		},
	}
}

// Deleted returns a predicate that triggers only on Delete events.
func Deleted() predicate.Predicate {
	return predicate.Funcs{
		GenericFunc: func(_ event.GenericEvent) bool {
			return false
		},
		CreateFunc: func(_ event.CreateEvent) bool {
			return false
		},
		DeleteFunc: func(_ event.DeleteEvent) bool {
			return true
		},
		UpdateFunc: func(_ event.UpdateEvent) bool {
			return false
		},
	}
}

// And returns a predicate that returns true if all provided predicates return true.
// This is an alias for predicate.And for convenience.
func And(predicates ...predicate.Predicate) predicate.Predicate {
	return predicate.And(predicates...)
}

// Or returns a predicate that returns true if any of the provided predicates return true.
// This is an alias for predicate.Or for convenience.
func Or(predicates ...predicate.Predicate) predicate.Predicate {
	return predicate.Or(predicates...)
}

// Default is the recommended predicate for most controllers.
// Triggers reconciliation when generation, labels, or annotations change, or on deletion.
//
// This combines:
// - GenerationChanged: Detects spec changes.
// - LabelChanged: Detects label modifications.
// - AnnotationChanged: Detects annotation modifications.
// - Deleted: Processes deletion events.
func Default() predicate.Predicate {
	return Or(
		GenerationChanged(),
		LabelChanged(),
		AnnotationChanged(),
		Deleted(),
	)
}
