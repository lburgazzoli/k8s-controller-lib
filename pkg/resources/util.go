package resources

import (
	"errors"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

// ToUnstructured converts any object to an unstructured.Unstructured.
//
// This function converts the input object to an unstructured representation
// using JSON marshaling to avoid scheme validation issues. The GroupVersionKind
// is preserved from the original object's TypeMeta if set.
//
// Example:
//
//	cm := &corev1.ConfigMap{
//	    TypeMeta: metav1.TypeMeta{
//	        APIVersion: "v1",
//	        Kind:       "ConfigMap",
//	    },
//	    ...
//	}
//	u, err := ToUnstructured(cm)
//	if err != nil {
//	    // handle error
//	}
func ToUnstructured(
	s *runtime.Scheme,
	in any,
) (*unstructured.Unstructured, error) {
	if in == nil {
		return nil, errors.New("nil object")
	}

	// Use runtime.DefaultUnstructuredConverter for conversion
	// This works without scheme registration when TypeMeta is set
	data, err := runtime.DefaultUnstructuredConverter.ToUnstructured(in)
	if err != nil {
		return nil, fmt.Errorf("unable to convert object %T to unstructured: %w", in, err)
	}

	obj := &unstructured.Unstructured{
		Object: data,
	}

	// Ensure that the object has a GroupVersionKind set
	if err := EnsureGroupVersionKind(s, obj); err != nil {
		return nil, fmt.Errorf("failed to ensure GroupVersionKind: %w", err)
	}

	return obj, nil
}

// ObjectToUnstructured converts a client.Object to an unstructured.Unstructured
// and ensures the GroupVersionKind is set.
//
// This function first ensures the object has a valid GVK using the provided scheme,
// then converts it to an unstructured representation.
//
// Example:
//
//	cm := &corev1.ConfigMap{...}
//	u, err := ObjectToUnstructured(scheme, cm)
//	if err != nil {
//	    // handle error
//	}
func ObjectToUnstructured(
	s *runtime.Scheme,
	obj client.Object,
) (*unstructured.Unstructured, error) {
	// Ensure that the object has a GroupVersionKind set
	if err := EnsureGroupVersionKind(s, obj); err != nil {
		return nil, fmt.Errorf("failed to ensure GroupVersionKind: %w", err)
	}

	// Now, convert the object to unstructured
	u, err := ToUnstructured(s, obj)
	if err != nil {
		return nil, err
	}

	return u, nil
}

// FromUnstructured converts an unstructured.Unstructured to a typed client.Object.
//
// This function converts the unstructured object to the provided typed object and
// ensures the GroupVersionKind is correctly set.
//
// Example:
//
//	var cm corev1.ConfigMap
//	err := FromUnstructured(scheme, u, &cm)
//	if err != nil {
//	    // handle error
//	}
func FromUnstructured(
	s *runtime.Scheme,
	obj *unstructured.Unstructured,
	intoObj client.Object,
) error {
	if obj == nil {
		return errors.New("nil object")
	}

	// Convert the unstructured object to the typed object
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, intoObj)
	if err != nil {
		return fmt.Errorf("unable to convert unstructured object to %T: %w", intoObj, err)
	}

	// Ensure that the GroupVersionKind is correctly set on the target object
	err = EnsureGroupVersionKind(s, intoObj)
	if err != nil {
		return fmt.Errorf("unable to ensure GroupVersionKind: %w", err)
	}

	return nil
}

// GetGroupVersionKindForObject returns the GroupVersionKind for the given object.
//
// If the object already has a GVK set, it returns that. Otherwise, it uses the
// scheme to determine the GVK for the object's type.
//
// Example:
//
//	cm := &corev1.ConfigMap{...}
//	gvk, err := GetGroupVersionKindForObject(scheme, cm)
//	if err != nil {
//	    // handle error
//	}
func GetGroupVersionKindForObject(
	s *runtime.Scheme,
	obj runtime.Object,
) (schema.GroupVersionKind, error) {
	if obj == nil {
		return schema.GroupVersionKind{}, errors.New("nil object")
	}

	if obj.GetObjectKind().GroupVersionKind().Version != "" &&
		obj.GetObjectKind().GroupVersionKind().Kind != "" {
		return obj.GetObjectKind().GroupVersionKind(), nil
	}

	gvk, err := apiutil.GVKForObject(obj, s)
	if err != nil {
		return schema.GroupVersionKind{}, fmt.Errorf("failed to get GVK: %w", err)
	}

	return gvk, nil
}

// EnsureGroupVersionKind ensures that the object has a GroupVersionKind set.
//
// If the object doesn't have a GVK, this function uses the scheme to determine
// and set the appropriate GVK.
//
// Example:
//
//	cm := &corev1.ConfigMap{...}
//	err := EnsureGroupVersionKind(scheme, cm)
//	if err != nil {
//	    // handle error
//	}
func EnsureGroupVersionKind(s *runtime.Scheme, obj client.Object) error {
	gvk, err := GetGroupVersionKindForObject(s, obj)
	if err != nil {
		return err
	}

	obj.GetObjectKind().SetGroupVersionKind(gvk)

	return nil
}

// GvkToUnstructured creates an unstructured object with the given GroupVersionKind.
//
// This is useful when you need to create an empty unstructured object with a specific GVK.
//
// Example:
//
//	gvk := schema.GroupVersionKind{
//	    Group: "",
//	    Version: "v1",
//	    Kind: "ConfigMap",
//	}
//	u := GvkToUnstructured(gvk)
func GvkToUnstructured(gvk schema.GroupVersionKind) *unstructured.Unstructured {
	u := unstructured.Unstructured{}
	u.SetGroupVersionKind(gvk)

	return &u
}

// FormatObjectReference formats an unstructured object reference as "GVK namespace/name".
//
// This is useful for logging and error messages to provide context about which
// object is being referenced without exposing sensitive data.
//
// Example:
//
//	u := &unstructured.Unstructured{...}
//	ref := FormatObjectReference(u)
//	// ref might be: "v1, Kind=ConfigMap default/my-config"
func FormatObjectReference(u *unstructured.Unstructured) string {
	gvk := u.GroupVersionKind().String()
	name := u.GetName()
	ns := u.GetNamespace()
	if ns != "" {
		return gvk + " " + ns + "/" + name
	}
	return gvk + " " + name
}

// NamespacedNameFromObject extracts a types.NamespacedName from a client.Object.
//
// This is a convenience function for getting the namespace and name from an object.
//
// Example:
//
//	cm := &corev1.ConfigMap{...}
//	nn := NamespacedNameFromObject(cm)
//	// nn.Namespace = "default", nn.Name = "my-config"
func NamespacedNameFromObject(obj client.Object) types.NamespacedName {
	return types.NamespacedName{
		Namespace: obj.GetNamespace(),
		Name:      obj.GetName(),
	}
}

// FormatNamespacedName formats a types.NamespacedName as "namespace/name" or just "name"
// for cluster-scoped resources.
//
// Example:
//
//	nn := types.NamespacedName{Namespace: "default", Name: "my-config"}
//	s := FormatNamespacedName(nn)
//	// s = "default/my-config"
//
//	nn2 := types.NamespacedName{Name: "my-cluster-resource"}
//	s2 := FormatNamespacedName(nn2)
//	// s2 = "my-cluster-resource"
func FormatNamespacedName(nn types.NamespacedName) string {
	if nn.Namespace == "" {
		return nn.Name
	}
	return nn.Namespace + "/" + nn.Name
}
