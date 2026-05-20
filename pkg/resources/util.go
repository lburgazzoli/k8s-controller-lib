package resources

import (
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	if obj, ok := in.(client.Object); ok {
		if err := EnsureGroupVersionKind(s, obj); err != nil {
			return nil, fmt.Errorf("failed to ensure GroupVersionKind: %w", err)
		}
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

// FromUnstructured converts an unstructured.Unstructured to a typed client.Object.
//
// This function converts the unstructured object to the provided typed object.
// If both objects have a GVK set, it validates that they match before conversion
// to prevent incompatible type conversions.
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
	inObj *unstructured.Unstructured,
	intoObj client.Object,
) error {
	if inObj == nil {
		return errors.New("nil object")
	}

	// Get GVKs for both objects if available
	inGVK, inErr := GetGroupVersionKindForObject(s, inObj)
	intoGVK, intoErr := GetGroupVersionKindForObject(s, intoObj)

	// If both objects have a valid GVK, verify they match
	if inErr == nil && intoErr == nil && inGVK != intoGVK {
		return fmt.Errorf(
			"incompatible types: cannot convert %s to %s",
			inGVK.String(),
			intoGVK.String(),
		)
	}

	// Convert the unstructured object to the typed object
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(inObj.Object, intoObj); err != nil {
		return fmt.Errorf("unable to convert unstructured object to %T: %w", intoObj, err)
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
func FormatObjectReference(u client.Object) string {
	gvk := u.GetObjectKind().GroupVersionKind().String()
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

// IsPartialObjectMetadata checks if the given object is a PartialObjectMetadata.
// Returns true if obj is *metav1.PartialObjectMetadata, false otherwise.
//
// This is useful for determining if an object contains only metadata or the full object.
//
// Example:
//
//	if resources.IsPartialObjectMetadata(obj) {
//	    // Handle metadata-only object
//	}
func IsPartialObjectMetadata(obj client.Object) bool {
	if obj == nil {
		return false
	}
	_, ok := obj.(*metav1.PartialObjectMetadata)

	return ok
}

// ToPartialObjectMetadata converts any client.Object to a PartialObjectMetadata.
//
// This function always returns a NEW instance, never the original object:
// - If the input is already a *metav1.PartialObjectMetadata, it returns a deep copy
// - If the input is another type, it creates a new PartialObjectMetadata with copied metadata
//
// PartialObjectMetadata contains only the object's metadata (name, namespace, labels,
// annotations, generation, resource version, owner references, finalizers, etc.) without
// spec or status fields. This is useful for efficient watching and caching when you only
// need metadata.
//
// The GroupVersionKind is preserved from the original object or determined from the scheme.
//
// Example:
//
//	cm := &corev1.ConfigMap{
//	    ObjectMeta: metav1.ObjectMeta{
//	        Name:      "my-config",
//	        Namespace: "default",
//	        Labels:    map[string]string{"app": "myapp"},
//	    },
//	}
//	partial, err := ToPartialObjectMetadata(scheme, cm)
//	if err != nil {
//	    // handle error
//	}
//	// partial contains only metadata, no spec/data fields
func ToPartialObjectMetadata(
	s *runtime.Scheme,
	obj client.Object,
) (*metav1.PartialObjectMetadata, error) {
	if obj == nil {
		return nil, errors.New("nil object")
	}

	// If already PartialObjectMetadata, return a deep copy
	if pom, ok := obj.(*metav1.PartialObjectMetadata); ok {
		return pom.DeepCopy(), nil
	}

	// Ensure the object has a GVK set
	if err := EnsureGroupVersionKind(s, obj); err != nil {
		return nil, fmt.Errorf("failed to ensure GroupVersionKind: %w", err)
	}

	// Create new PartialObjectMetadata with copied metadata
	partial := &metav1.PartialObjectMetadata{
		ObjectMeta: metav1.ObjectMeta{
			Name:                       obj.GetName(),
			Namespace:                  obj.GetNamespace(),
			UID:                        obj.GetUID(),
			ResourceVersion:            obj.GetResourceVersion(),
			Generation:                 obj.GetGeneration(),
			CreationTimestamp:          obj.GetCreationTimestamp(),
			DeletionTimestamp:          obj.GetDeletionTimestamp(),
			DeletionGracePeriodSeconds: obj.GetDeletionGracePeriodSeconds(),
			Labels:                     maps.Clone(obj.GetLabels()),
			Annotations:                maps.Clone(obj.GetAnnotations()),
			OwnerReferences:            slices.Clone(obj.GetOwnerReferences()),
			Finalizers:                 slices.Clone(obj.GetFinalizers()),
			ManagedFields:              DeepCopySlice(obj.GetManagedFields()),
		},
	}

	partial.SetGroupVersionKind(obj.GetObjectKind().GroupVersionKind())

	return partial, nil
}

// ConvertList converts a slice of unstructured objects to a slice of typed objects.
// When T is *unstructured.Unstructured or client.Object, a fast path avoids
// conversion. For typed targets (e.g. *appsv1.Deployment), scheme.Convert
// handles the conversion.
func ConvertList[T client.Object](s *runtime.Scheme, objects []unstructured.Unstructured) ([]T, error) {
	result := make([]T, 0, len(objects))

	for i := range objects {
		if obj, ok := any(&objects[i]).(T); ok {
			result = append(result, obj)

			continue
		}

		out, ok := reflect.New(reflect.TypeFor[T]().Elem()).Interface().(T)
		if !ok {
			return nil, fmt.Errorf("cannot convert to target type for %s", FormatObjectReference(&objects[i]))
		}
		if err := s.Convert(&objects[i], out, nil); err != nil {
			return nil, fmt.Errorf("converting %s: %w", FormatObjectReference(&objects[i]), err)
		}

		result = append(result, out)
	}

	return result, nil
}
