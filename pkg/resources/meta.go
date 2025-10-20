package resources

import (
	"slices"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

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

func SetLabels(obj client.Object, values map[string]string) {
	if obj == nil {
		return
	}

	target := obj.GetLabels()
	if target == nil {
		target = make(map[string]string)
	}

	for k, v := range values {
		target[k] = v
	}

	obj.SetLabels(target)
}

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

func SetAnnotations(obj client.Object, values map[string]string) {
	if obj == nil {
		return
	}

	target := obj.GetAnnotations()
	if target == nil {
		target = make(map[string]string)
	}

	for k, v := range values {
		target[k] = v
	}

	obj.SetAnnotations(target)
}

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
