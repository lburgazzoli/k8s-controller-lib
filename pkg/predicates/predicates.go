package predicates

import (
	"maps"

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
// - Create: Returns false (creation doesn't need reconciliation)
// - Delete: Returns true (deletions should be processed)
// - Update: Returns true if generation changed or either object has zero generation
//
//nolint:gochecknoglobals
var GenerationChanged = predicate.Funcs{
	CreateFunc: func(e event.CreateEvent) bool {
		return false
	},
	DeleteFunc: func(e event.DeleteEvent) bool {
		return true
	},
	UpdateFunc: func(e event.UpdateEvent) bool {
		if e.ObjectOld == nil || e.ObjectNew == nil {
			return false
		}

		// Zero generation means the resource doesn't track generation (e.g., ConfigMap, Secret).
		// Process all updates for such resources to avoid missing changes.
		if e.ObjectNew.GetGeneration() == 0 || e.ObjectOld.GetGeneration() == 0 {
			return true
		}

		return e.ObjectNew.GetGeneration() != e.ObjectOld.GetGeneration()
	},
}

// LabelChanged triggers reconciliation when object labels change.
//
// Behavior:
// - Create: Returns false (creation doesn't need reconciliation)
// - Delete: Returns true (deletions should be processed)
// - Update: Returns true if labels changed
//
//nolint:gochecknoglobals
var LabelChanged = predicate.Funcs{
	CreateFunc: func(e event.CreateEvent) bool {
		return false
	},
	DeleteFunc: func(e event.DeleteEvent) bool {
		return true
	},
	UpdateFunc: func(e event.UpdateEvent) bool {
		if e.ObjectOld == nil || e.ObjectNew == nil {
			return false
		}
		return !maps.Equal(e.ObjectNew.GetLabels(), e.ObjectOld.GetLabels())
	},
}

// AnnotationChanged triggers reconciliation when object annotations change.
//
// Behavior:
// - Create: Returns false (creation doesn't need reconciliation)
// - Delete: Returns true (deletions should be processed)
// - Update: Returns true if annotations changed
//
//nolint:gochecknoglobals
var AnnotationChanged = predicate.Funcs{
	CreateFunc: func(e event.CreateEvent) bool {
		return false
	},
	DeleteFunc: func(e event.DeleteEvent) bool {
		return true
	},
	UpdateFunc: func(e event.UpdateEvent) bool {
		if e.ObjectOld == nil || e.ObjectNew == nil {
			return false
		}
		return !maps.Equal(e.ObjectNew.GetAnnotations(), e.ObjectOld.GetAnnotations())
	},
}

// Default is the recommended predicate for most controllers.
// Triggers reconciliation when generation, labels, or annotations change.
//
// This combines:
// - GenerationChanged: Detects spec changes.
// - LabelChanged: Detects label modifications.
// - AnnotationChanged: Detects annotation modifications.
//
//nolint:gochecknoglobals
var Default = predicate.Or(
	GenerationChanged,
	LabelChanged,
	AnnotationChanged,
)
