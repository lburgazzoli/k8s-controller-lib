package watch

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
)

const (
	// LabelOwnerName is the label key for owner name tracking.
	LabelOwnerName = "controller-lib.k8s.io/owner-name"
	// LabelOwnerNamespace is the label key for owner namespace tracking.
	LabelOwnerNamespace = "controller-lib.k8s.io/owner-namespace"
)

// EnqueueRequestForOwnerOrLabel creates a handler that maps events to owner reconciliation.
// It first tries to use OwnerReferences (standard Kubernetes approach).
// If no controller owner reference is found, it falls back to using
// controller-lib.k8s.io/owner-name and controller-lib.k8s.io/owner-namespace labels.
//
// This handler works seamlessly with both owned and non-owned objects,
// making it suitable for mixed ownership scenarios.
//
// Example:
//
//	handler := watch.EnqueueRequestForOwnerOrLabel(scheme, &v1.Pod{}, true)
func EnqueueRequestForOwnerOrLabel(
	scheme *runtime.Scheme,
	ownerType client.Object,
	isController bool,
) handler.EventHandler {
	enqueueOwner := func(obj client.Object, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
		if obj == nil {
			return
		}

		// Try owner references first
		if req := getOwnerFromReferences(scheme, ownerType, isController, obj); req != nil {
			q.Add(*req)

			return
		}

		// Fall back to labels
		if req := getOwnerFromLabels(obj); req != nil {
			q.Add(*req)
		}
	}

	return handler.TypedFuncs[client.Object, reconcile.Request]{
		CreateFunc: func(
			_ context.Context,
			evt event.TypedCreateEvent[client.Object],
			q workqueue.TypedRateLimitingInterface[reconcile.Request],
		) {
			enqueueOwner(evt.Object, q)
		},
		UpdateFunc: func(
			_ context.Context,
			evt event.TypedUpdateEvent[client.Object],
			q workqueue.TypedRateLimitingInterface[reconcile.Request],
		) {
			enqueueOwner(evt.ObjectNew, q)
		},
		DeleteFunc: func(
			_ context.Context,
			evt event.TypedDeleteEvent[client.Object],
			q workqueue.TypedRateLimitingInterface[reconcile.Request],
		) {
			enqueueOwner(evt.Object, q)
		},
		GenericFunc: func(
			_ context.Context,
			evt event.TypedGenericEvent[client.Object],
			q workqueue.TypedRateLimitingInterface[reconcile.Request],
		) {
			enqueueOwner(evt.Object, q)
		},
	}
}

// getOwnerFromReferences extracts owner from OwnerReferences.
// Returns nil if no matching owner reference is found.
//
//nolint:revive // isController follows controller-runtime pattern
func getOwnerFromReferences(
	scheme *runtime.Scheme,
	ownerType client.Object,
	isController bool,
	obj client.Object,
) *reconcile.Request {
	// Get owner type GVK
	ownerGVK, err := apiutil.GVKForObject(ownerType, scheme)
	if err != nil {
		return nil
	}

	// Find matching owner reference
	for _, ref := range obj.GetOwnerReferences() {
		refGV, err := schema.ParseGroupVersion(ref.APIVersion)
		if err != nil {
			continue
		}

		// Check if this reference matches our owner type
		if refGV.Group != ownerGVK.Group ||
			ref.Kind != ownerGVK.Kind {
			continue
		}

		// If we only want controller owners, check the Controller field
		if isController && (ref.Controller == nil || !*ref.Controller) {
			continue
		}

		// Found a match
		return &reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      ref.Name,
				Namespace: obj.GetNamespace(),
			},
		}
	}

	return nil
}

// getOwnerFromLabels extracts owner from labels.
// Returns nil if labels are not present or incomplete.
func getOwnerFromLabels(obj client.Object) *reconcile.Request {
	labels := obj.GetLabels()
	if labels == nil {
		return nil
	}

	ownerName := labels[LabelOwnerName]
	if ownerName == "" {
		return nil
	}

	// Default namespace to object's namespace if not specified in label
	ownerNamespace := labels[LabelOwnerNamespace]
	if ownerNamespace == "" {
		ownerNamespace = obj.GetNamespace()
	}

	return &reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      ownerName,
			Namespace: ownerNamespace,
		},
	}
}
