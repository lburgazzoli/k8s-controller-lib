package status

import (
	"errors"
	"fmt"
	"reflect"
)

// Getter interface defines the method to retrieve Status from a resource.
type Getter interface {
	GetStatus() *Status
}

// Setter interface defines the method to set Status on a resource.
type Setter interface {
	SetStatus(status *Status)
}

// Accessor interface combines Getter and Setter for full status management.
type Accessor interface {
	Getter
	Setter
}

// statusAccessor provides a Accessor implementation using reflection.
type statusAccessor struct {
	statusField reflect.Value
}

// GetStatus returns the Status object.
func (a *statusAccessor) GetStatus() *Status {
	if !a.statusField.IsValid() {
		return nil
	}
	return a.statusField.Addr().Interface().(*Status)
}

// SetStatus sets the Status object.
func (a *statusAccessor) SetStatus(status *Status) {
	if a.statusField.IsValid() && a.statusField.CanSet() && status != nil {
		a.statusField.Set(reflect.ValueOf(*status))
	}
}

// NewAccessor extracts a Status accessor from an arbitrary object.
// The object must be a pointer to a struct with a Status field of type Status.
//
// This is inspired by k8s.io/apimachinery/pkg/api/meta.Accessor but for Status objects.
//
// Example:
//
//	type MyResource struct {
//	    metav1.TypeMeta   `json:",inline"`
//	    metav1.ObjectMeta `json:"metadata,omitempty"`
//	    Spec   MyResourceSpec   `json:"spec,omitempty"`
//	    Status status.Status    `json:"status,omitempty"`
//	}
//
//	resource := &MyResource{}
//	statusAccessor, err := status.NewAccessor(resource)
//	if err != nil {
//	    // handle error
//	}
//
//	// Get the status
//	s := statusAccessor.GetStatus()
//	s.ObservedGeneration = 5
//
//	// Or set a new status
//	newStatus := &status.Status{ObservedGeneration: 10}
//	statusAccessor.SetStatus(newStatus)
func NewAccessor(obj any) (Accessor, error) {
	v := reflect.ValueOf(obj)
	if v.Kind() != reflect.Ptr {
		return nil, fmt.Errorf("expected pointer, got %v", v.Kind())
	}

	v = v.Elem()
	if v.Kind() != reflect.Struct {
		return nil, fmt.Errorf("expected struct, got %v", v.Kind())
	}

	statusField := v.FieldByName("Status")
	if !statusField.IsValid() {
		return nil, errors.New("object does not have a Status field")
	}

	// Verify the Status field is of type Status
	statusType := reflect.TypeFor[Status]()
	if statusField.Type() != statusType {
		return nil, fmt.Errorf("status field is not of type status.Status, got %v", statusField.Type())
	}

	return &statusAccessor{statusField: statusField}, nil
}
