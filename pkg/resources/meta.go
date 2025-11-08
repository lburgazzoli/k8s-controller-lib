package resources

import (
	"maps"
	"slices"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// HasLabel returns true if the object has a label with the given key and its value matches one of the provided values.
func HasLabel(obj client.Object, k string, values ...string) bool {
	if obj == nil {
		return false
	}

	target := obj.GetLabels()
	if target == nil {
		return false
	}

	val, found := target[k]
	if !found {
		return false
	}

	return slices.Contains(values, val)
}

// SetLabels copies all key-value pairs from values into the object's labels.
func SetLabels(obj client.Object, values map[string]string) {
	if obj == nil {
		return
	}

	target := obj.GetLabels()
	if target == nil {
		target = make(map[string]string)
	}

	maps.Copy(target, values)

	obj.SetLabels(target)
}

// SetLabel sets a label with the given key and value on the object and returns the previous value.
func SetLabel(obj client.Object, k string, v string) string {
	if obj == nil {
		return ""
	}

	target := obj.GetLabels()
	if target == nil {
		target = make(map[string]string)
	}

	old := target[k]
	target[k] = v

	obj.SetLabels(target)

	return old
}

// RemoveLabel removes the label with the given key from the object.
func RemoveLabel(obj client.Object, k string) {
	if obj == nil {
		return
	}

	target := obj.GetLabels()
	if target == nil {
		return
	}

	delete(target, k)

	obj.SetLabels(target)
}

// GetLabel returns the value of the label with the given key, or an empty string if not found.
func GetLabel(obj client.Object, k string) string {
	if obj == nil {
		return ""
	}

	target := obj.GetLabels()
	if target == nil {
		return ""
	}

	return target[k]
}

// HasAnnotation returns true if the object has an annotation with the given key
// and its value matches one of the provided values.
func HasAnnotation(obj client.Object, k string, values ...string) bool {
	if obj == nil {
		return false
	}

	target := obj.GetAnnotations()
	if target == nil {
		return false
	}

	val, found := target[k]
	if !found {
		return false
	}

	return slices.Contains(values, val)
}

// SetAnnotations copies all key-value pairs from values into the object's annotations.
func SetAnnotations(obj client.Object, values map[string]string) {
	if obj == nil {
		return
	}

	target := obj.GetAnnotations()
	if target == nil {
		target = make(map[string]string)
	}

	maps.Copy(target, values)

	obj.SetAnnotations(target)
}

// SetAnnotation sets an annotation with the given key and value on the object and returns the previous value.
func SetAnnotation(obj client.Object, k string, v string) string {
	if obj == nil {
		return ""
	}

	target := obj.GetAnnotations()
	if target == nil {
		target = make(map[string]string)
	}

	old := target[k]
	target[k] = v

	obj.SetAnnotations(target)

	return old
}

// RemoveAnnotation removes the annotation with the given key from the object.
func RemoveAnnotation(obj client.Object, k string) {
	if obj == nil {
		return
	}

	target := obj.GetAnnotations()
	if target == nil {
		return
	}

	delete(target, k)

	obj.SetAnnotations(target)
}

// GetAnnotation returns the value of the annotation with the given key, or an empty string if not found.
func GetAnnotation(obj client.Object, k string) string {
	if obj == nil {
		return ""
	}

	target := obj.GetAnnotations()
	if target == nil {
		return ""
	}

	return target[k]
}
