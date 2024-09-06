package patch

import (
	"reflect"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
)

func ApplyPatch(source runtime.Object) (*unstructured.Unstructured, error) {
	switch s := source.(type) {
	case *unstructured.Unstructured:
		// Directly take the unstructured object as apply patch
		return s, nil
	default:
		// Otherwise, for typed objects, remove null fields from the apply patch,
		// so that ownership is not taken for non-managed fields.
		// See https://github.com/kubernetes/enhancements/tree/master/keps/sig-api-machinery/2155-clientgo-apply
		sourceJSON, err := json.Marshal(source)
		if err != nil {
			return nil, err
		}
		var positivePatch map[string]interface{}
		err = json.Unmarshal(sourceJSON, &positivePatch)
		if err != nil {
			return nil, err
		}
		removeNilValues(reflect.ValueOf(positivePatch), reflect.Value{})

		return &unstructured.Unstructured{Object: positivePatch}, nil
	}
}

func removeNilValues(v reflect.Value, parent reflect.Value) {
	for v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
		v = v.Elem()
	}
	switch v.Kind() {
	case reflect.Array, reflect.Slice:
		for i := range v.Len() {
			removeNilValues(v.Index(i), v)
		}
	case reflect.Map:
		for _, k := range v.MapKeys() {
			switch c := v.MapIndex(k); {
			case !c.IsValid():
				// Skip keys previously deleted
				continue
			case c.IsNil(), c.Elem().Kind() == reflect.Map && len(c.Elem().MapKeys()) == 0:
				v.SetMapIndex(k, reflect.Value{})
			default:
				removeNilValues(c, v)
			}
		}
		// Back process the parent map in case it has been emptied so that it's deleted as well
		if len(v.MapKeys()) == 0 && parent.Kind() == reflect.Map {
			removeNilValues(parent, reflect.Value{})
		}
	default:
		// do nothing
	}
}
