package watch

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// ByLabel returns a mapper that enqueues a reconcile request using the
// value of the given label as the object name. Useful for watching
// resources that reference an owner via label rather than ownerReference
// (e.g. cross-namespace resources that cannot use ownerReferences).
func ByLabel(
	labelKey string,
) func(context.Context, *metav1.PartialObjectMetadata) []reconcile.Request {
	return func(_ context.Context, obj *metav1.PartialObjectMetadata) []reconcile.Request {
		name, ok := obj.GetLabels()[labelKey]
		if !ok {
			return nil
		}

		return []reconcile.Request{{
			NamespacedName: types.NamespacedName{Name: name},
		}}
	}
}
