package status

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen=true

// Status represents the common status fields for Kubernetes custom resources.
// This struct implements the conditions.Accessor interface and follows
// Kubernetes API conventions for status reporting.
type Status struct {
	// ObservedGeneration reflects the generation of the most recently observed resource.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions represent the latest available observations of the resource's state.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// GetConditions returns the conditions slice (implements conditions.Accessor).
func (s *Status) GetConditions() []metav1.Condition {
	return s.Conditions
}

// SetConditions sets the conditions slice (implements conditions.Accessor).
func (s *Status) SetConditions(conditions []metav1.Condition) {
	s.Conditions = conditions
}
