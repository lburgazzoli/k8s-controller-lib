package resources

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func NewUnstructured(group string, version string, kind string) *unstructured.Unstructured {
	u := unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Kind:    kind,
		Group:   group,
		Version: version,
	})

	return &u
}

func ConvertToUnstructured(s *runtime.Scheme, obj runtime.Object) (*unstructured.Unstructured, error) {
	switch ot := obj.(type) {
	case *unstructured.Unstructured:
		return ot, nil
	default:
		var err error
		var u unstructured.Unstructured

		u.Object, err = runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
		if err != nil {
			return nil, fmt.Errorf("failed to convert to unstructured: %w", err)
		}

		gvk := u.GroupVersionKind()
		if gvk.Group == "" || gvk.Kind == "" {
			gvks, _, err := s.ObjectKinds(obj)
			if err != nil {
				return nil, fmt.Errorf("failed to convert to unstructured - unable to get GVK %w", err)
			}

			v, k := gvks[0].ToAPIVersionAndKind()

			u.SetAPIVersion(v)
			u.SetKind(k)
		}

		return &u, nil
	}
}

func Decode(decoder runtime.Decoder, content []byte) ([]unstructured.Unstructured, error) {
	results := make([]unstructured.Unstructured, 0)

	r := bytes.NewReader(content)
	yd := yaml.NewDecoder(r)

	for {
		var out map[string]interface{}

		err := yd.Decode(&out)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return nil, fmt.Errorf("unable to decode resource: %w", err)
		}

		if len(out) == 0 {
			continue
		}

		if out["Kind"] == "" {
			continue
		}

		encoded, err := yaml.Marshal(out)
		if err != nil {
			return nil, fmt.Errorf("unable to marshal resource: %w", err)
		}

		var obj unstructured.Unstructured

		if _, _, err = decoder.Decode(encoded, nil, &obj); err != nil {
			if runtime.IsMissingKind(err) {
				continue
			}

			return nil, fmt.Errorf("unable to decode resource: %w", err)
		}

		results = append(results, obj)
	}

	return results, nil
}
