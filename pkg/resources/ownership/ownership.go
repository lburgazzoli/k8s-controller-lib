package ownership

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/util"
)

// Options configure owner reference behavior.
type Options struct {
	BlockOwnerDeletion *bool
	Controller         *bool
}

// Option is a functional option for ownership configuration.
type Option = util.Option[Options]

// WithBlockDeletion sets BlockOwnerDeletion to true on the owner reference.
// This prevents the owned object from being deleted until the owner is deleted.
func WithBlockDeletion() Option {
	return util.FunctionalOption[Options](func(opts *Options) {
		t := true
		opts.BlockOwnerDeletion = &t
	})
}

// WithController sets Controller to true on the owner reference.
// Only one owner reference can have Controller=true.
// The controller owner reference enables automatic garbage collection.
func WithController() Option {
	return util.FunctionalOption[Options](func(opts *Options) {
		t := true
		opts.Controller = &t
	})
}

// SetOwner sets an owner reference from owner to owned with the specified options.
// This is a convenience wrapper around controllerutil.SetOwnerReference that allows
// customizing the BlockOwnerDeletion and Controller fields.
//
// Namespace constraints:
// - Cluster-scoped owners (e.g., Namespace, ClusterRole) can own namespaced resources
// - Namespaced owners can only own resources in the same namespace
// - Cross-namespace ownership between namespaced resources is not supported
//
// Example:
//
//	err := ownership.SetOwner(scheme, myResource, deployment,
//	    ownership.WithController(),
//	    ownership.WithBlockDeletion())
func SetOwner(
	scheme *runtime.Scheme,
	owner client.Object,
	owned client.Object,
	opts ...Option,
) error {
	options := &Options{}
	for _, opt := range opts {
		opt.ApplyTo(options)
	}

	// Validate namespace constraints for owner references:
	// - Cluster-scoped owners (empty namespace) can own namespaced resources
	// - Namespaced owners can only own resources in the same namespace
	if owner.GetNamespace() != "" && owner.GetNamespace() != owned.GetNamespace() {
		return fmt.Errorf("namespaced owner cannot own resource in different namespace: owner=%s/%s, owned=%s/%s",
			owner.GetNamespace(), owner.GetName(),
			owned.GetNamespace(), owned.GetName())
	}

	// Use controllerutil to set the base owner reference
	if err := controllerutil.SetOwnerReference(owner, owned, scheme); err != nil {
		return fmt.Errorf("failed to set owner reference: %w", err)
	}

	// Apply custom options to the newly set owner reference
	applyOwnerOptions(owner, owned, options)

	return nil
}

func applyOwnerOptions(owner client.Object, owned client.Object, options *Options) {
	if options.BlockOwnerDeletion == nil && options.Controller == nil {
		return
	}

	ownerRefs := owned.GetOwnerReferences()
	for i := range ownerRefs {
		if ownerRefs[i].UID != owner.GetUID() {
			continue
		}

		if options.BlockOwnerDeletion != nil {
			ownerRefs[i].BlockOwnerDeletion = options.BlockOwnerDeletion
		}
		if options.Controller != nil {
			ownerRefs[i].Controller = options.Controller
		}
	}
	owned.SetOwnerReferences(ownerRefs)
}

// IsOwnedBy checks if the owned object has an owner reference to the specified owner.
// Returns true if an owner reference with matching UID exists.
func IsOwnedBy(owner client.Object, owned client.Object) bool {
	ownerUID := owner.GetUID()
	for _, ref := range owned.GetOwnerReferences() {
		if ref.UID == ownerUID {
			return true
		}
	}

	return false
}

// IsController checks if the owned object has a controller owner reference to the specified owner.
// Returns true if an owner reference with matching UID and Controller=true exists.
func IsController(owner client.Object, owned client.Object) bool {
	ownerUID := owner.GetUID()
	for _, ref := range owned.GetOwnerReferences() {
		if ref.UID == ownerUID && ref.Controller != nil && *ref.Controller {
			return true
		}
	}

	return false
}

// RemoveOwner removes the owner reference from owned that points to owner.
// Returns true if an owner reference was removed, false otherwise.
func RemoveOwner(owner client.Object, owned client.Object) bool {
	ownerUID := owner.GetUID()
	ownerRefs := owned.GetOwnerReferences()

	for i, ref := range ownerRefs {
		if ref.UID == ownerUID {
			// Remove the owner reference by excluding it from the slice
			owned.SetOwnerReferences(append(ownerRefs[:i], ownerRefs[i+1:]...))

			return true
		}
	}

	return false
}

// GetOwnerReference returns the owner reference from owned that points to owner.
// Returns nil if no matching owner reference exists.
func GetOwnerReference(owner client.Object, owned client.Object) *metav1.OwnerReference {
	ownerUID := owner.GetUID()
	for i := range owned.GetOwnerReferences() {
		if owned.GetOwnerReferences()[i].UID == ownerUID {
			ref := owned.GetOwnerReferences()[i]

			return &ref
		}
	}

	return nil
}
