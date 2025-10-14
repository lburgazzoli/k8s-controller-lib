package resources

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Apply patches a Kubernetes object using server-side apply.
//
// This function converts the input object to an unstructured type, removes fields
// that are not required for patching (managedFields, resourceVersion, status),
// applies the patch, and updates the input object with the patched result.
//
// The input object must have a GroupVersionKind set (via TypeMeta).
//
// Parameters:
//   - ctx: The context for the client operation
//   - cli: The Kubernetes client interface used to perform the patch operation
//   - in: The Kubernetes object to be applied
//   - opts: Optional client patch options (e.g., client.FieldOwner, client.ForceOwnership)
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
func Apply(
	ctx context.Context,
	cli client.Client,
	in client.Object,
	opts ...client.PatchOption,
) error {
	u, err := ToUnstructured(cli.Scheme(), in)
	if err != nil {
		return fmt.Errorf("failed to convert resource to unstructured: %w", err)
	}

	// safe copy
	u = u.DeepCopy()

	// remove not required fields
	unstructured.RemoveNestedField(u.Object, "metadata", "managedFields")
	unstructured.RemoveNestedField(u.Object, "metadata", "resourceVersion")
	unstructured.RemoveNestedField(u.Object, "status")

	err = cli.Patch(ctx, u, client.Apply, opts...)
	if err != nil {
		// Include GVK and namespace/name for debugging context without logging sensitive object data
		return fmt.Errorf("unable to patch %s: %w", FormatObjectReference(u), err)
	}

	/// Write back the modified object so callers can access the patched object.
	err = FromUnstructured(cli.Scheme(), u, in)
	if err != nil {
		return fmt.Errorf("failed to write modified object: %w", err)
	}

	return nil
}

// ApplyStatus patches the status subresource of a Kubernetes object using server-side apply.
//
// This function converts the input object to an unstructured type, removes fields that are
// not required for patching (i.e. managedFields and resourceVersion), applies the patch to
// the status subresource, and updates the input object with the patched result.
//
// The function handles the case where the resource doesn't exist (NotFound error) by treating
// it as a success condition and returning nil.
//
// The input object must have a GroupVersionKind set (via TypeMeta).
//
// Parameters:
//   - ctx: The context for the client operation
//   - cli: The Kubernetes client interface used to perform the patch operation
//   - in: The Kubernetes object whose status should be applied
//   - opts: Optional client subresource patch options
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
	opts ...client.SubResourcePatchOption,
) error {
	u, err := ToUnstructured(cli.Scheme(), in)
	if err != nil {
		return fmt.Errorf("failed to convert resource to unstructured: %w", err)
	}

	// safe copy
	u = u.DeepCopy()

	// remove not required fields
	unstructured.RemoveNestedField(u.Object, "metadata", "managedFields")
	unstructured.RemoveNestedField(u.Object, "metadata", "resourceVersion")

	err = cli.Status().Patch(ctx, u, client.Apply, opts...)
	switch {
	case k8serr.IsNotFound(err):
		// Resource doesn't exist, treat as success
		return nil
	case err != nil:
		// Include GVK and namespace/name for debugging context without logging sensitive object data
		return fmt.Errorf("unable to patch %s status: %w", FormatObjectReference(u), err)
	}

	// Write back the modified object so callers can access the patched object.
	err = FromUnstructured(cli.Scheme(), u, in)
	if err != nil {
		return fmt.Errorf("failed to write modified object: %w", err)
	}

	return nil
}
