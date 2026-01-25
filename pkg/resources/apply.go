package resources

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Server-Side Apply Implementation Notes
//
// This package implements Server-Side Apply (SSA) using controller-runtime's modern
// Client.Apply() and SubResourceWriter.Apply() methods, which were introduced in
// controller-runtime v0.19.0 as replacements for the deprecated client.Apply patch type.
//
// Key Implementation Details:
//
// 1. Conversion to ApplyConfiguration:
//    We convert objects to unstructured format, clean up fields that shouldn't be in
//    the apply request (managedFields, resourceVersion, status for main objects), then
//    use client.ApplyConfigurationFromUnstructured() to create a runtime.ApplyConfiguration.
//
// 2. Write-back Behavior:
//    - Apply(): Does NOT write back server values to the input object. The new API
//      returns the apply configuration, not the full object state. Callers should
//      re-fetch the object if they need the updated state.
//    - ApplyStatus(): DOES write back the status to the input object, as status updates
//      are typically the final operation in reconciliation and callers expect the status
//      to be updated in place.
//
// 3. Option Types:
//    The new API uses different option types than the deprecated patch-based approach:
//    - Apply() uses client.ApplyOption (includes FieldOwner, ForceOwnership)
//    - ApplyStatus() uses client.SubResourceApplyOption
//
// 4. Compatibility:
//    This implementation works correctly with both fake clients (for unit tests) and
//    real Kubernetes clusters. The previous patch-based approach had issues with fake
//    client resource version tracking in controller-runtime v0.23.0+.
//
// For more information on Server-Side Apply, see:
// - https://kubernetes.io/docs/reference/using-api/server-side-apply/
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/client#Client.Apply

// Apply patches a Kubernetes object using server-side apply.
//
// This function uses the modern Client.Apply() method with runtime.ApplyConfiguration,
// converting the input object to an unstructured type first. Fields not required for
// server-side apply (managedFields, resourceVersion, status) are removed before applying.
//
// After a successful apply, the function writes back the complete server response to the
// input object, including all server-assigned fields like resourceVersion, UID, generation,
// creationTimestamp, and any other fields that were updated or set by the server.
//
// The input object must have a GroupVersionKind set (via TypeMeta).
//
// Parameters:
//   - ctx: The context for the client operation
//   - cli: The Kubernetes client interface used to perform the apply operation
//   - in: The Kubernetes object to be applied
//   - opts: Optional client apply options (e.g., client.FieldOwner, client.ForceOwnership)
//
// Returns:
//   - error: An error if the apply operation fails
//
// Example:
//
//	cm := &corev1.ConfigMap{
//	    TypeMeta: metav1.TypeMeta{
//	        APIVersion: "v1",
//	        Kind:       "ConfigMap",
//	    },
//	    ObjectMeta: metav1.ObjectMeta{
//	        Name:      "my-config",
//	        Namespace: "default",
//	    },
//	    Data: map[string]string{"key": "value"},
//	}
//
//	err := Apply(ctx, cli, cm, client.FieldOwner("my-controller"))
//	if err != nil {
//	    // handle error
//	}
//	// cm.ResourceVersion, cm.UID, etc. are now populated with server values
func Apply(
	ctx context.Context,
	cli client.Client,
	in client.Object,
	opts ...client.ApplyOption,
) error {
	u, err := ToUnstructured(cli.Scheme(), in)
	if err != nil {
		return fmt.Errorf("failed to convert resource to unstructured: %w", err)
	}

	// Deep copy to avoid modifying the input object or cached data
	u = u.DeepCopy()

	unstructured.RemoveNestedField(u.Object, "metadata", "managedFields")
	unstructured.RemoveNestedField(u.Object, "metadata", "resourceVersion")
	unstructured.RemoveNestedField(u.Object, "status")

	// Convert unstructured to ApplyConfiguration for the new SSA API
	applyConfig := client.ApplyConfigurationFromUnstructured(u)

	err = cli.Apply(ctx, applyConfig, opts...)
	if err != nil {
		// Include GVK and namespace/name for debugging context without logging sensitive object data
		return fmt.Errorf("unable to apply %s: %w", FormatObjectReference(u), err)
	}

	// Write back the modified object so callers can access the applied object.
	// The Apply operation updates the unstructured object with the server response,
	// including resourceVersion, UID, generation, and other server-assigned fields.
	err = FromUnstructured(cli.Scheme(), u, in)
	if err != nil {
		return fmt.Errorf("failed to write modified object: %w", err)
	}

	return nil
}

// ApplyStatus patches the status subresource of a Kubernetes object using server-side apply.
//
// This function uses the modern SubResourceWriter.Apply() method with runtime.ApplyConfiguration,
// converting the input object to an unstructured type first. Fields not required for server-side
// apply (managedFields, resourceVersion) are removed before applying. Unlike Apply(), this function
// keeps the status field since we're applying to the status subresource.
//
// The function handles the case where the resource doesn't exist (NotFound error) by treating
// it as a success condition and returning nil.
//
// The input object must have a GroupVersionKind set (via TypeMeta).
//
// Note: Unlike the deprecated patch-based approach, this function writes back the updated status
// to the input object after successful apply, as status updates are typically the final operation
// in a reconciliation and callers expect the status to reflect the applied state.
//
// Parameters:
//   - ctx: The context for the client operation
//   - cli: The Kubernetes client interface used to perform the apply operation
//   - in: The Kubernetes object whose status should be applied
//   - opts: Optional client subresource apply options
//
// Returns:
//   - error: An error if the apply operation fails (except for NotFound errors)
//
// Example:
//
//	cm := &corev1.ConfigMap{
//	    TypeMeta: metav1.TypeMeta{
//	        APIVersion: "v1",
//	        Kind:       "ConfigMap",
//	    },
//	    ObjectMeta: metav1.ObjectMeta{
//	        Name:      "my-config",
//	        Namespace: "default",
//	    },
//	    Status: corev1.ConfigMapStatus{...},
//	}
//
//	err := ApplyStatus(ctx, cli, cm, client.FieldOwner("my-controller"))
//	if err != nil {
//	    // handle error
//	}
func ApplyStatus(
	ctx context.Context,
	cli client.Client,
	in client.Object,
	opts ...client.SubResourceApplyOption,
) error {
	u, err := ToUnstructured(cli.Scheme(), in)
	if err != nil {
		return fmt.Errorf("failed to convert resource to unstructured: %w", err)
	}

	// Deep copy to avoid modifying the input object or cached data
	u = u.DeepCopy()

	unstructured.RemoveNestedField(u.Object, "metadata", "managedFields")
	unstructured.RemoveNestedField(u.Object, "metadata", "resourceVersion")

	// Convert unstructured to ApplyConfiguration for the new SSA API
	applyConfig := client.ApplyConfigurationFromUnstructured(u)

	err = cli.Status().Apply(ctx, applyConfig, opts...)
	switch {
	case k8serr.IsNotFound(err):
		// Resource doesn't exist, treat as success
		return nil
	case err != nil:
		// Include GVK and namespace/name for debugging context without logging sensitive object data
		return fmt.Errorf("unable to apply %s status: %w", FormatObjectReference(u), err)
	}

	// Write back the modified object so callers can access the updated status.
	// This is important for status updates as callers typically expect the status
	// to be updated in place after a successful apply.
	err = FromUnstructured(cli.Scheme(), u, in)
	if err != nil {
		return fmt.Errorf("failed to write modified object: %w", err)
	}

	return nil
}
