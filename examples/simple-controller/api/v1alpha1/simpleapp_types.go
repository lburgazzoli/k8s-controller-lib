package v1alpha1

import (
	"github.com/lburgazzoli/k8s-controller-lib/pkg/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SimpleAppSpec defines the desired state of SimpleApp
type SimpleAppSpec struct {
	// Replicas is the number of desired replicas
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=0
	Replicas int32 `json:"replicas,omitempty"`

	// Image is the container image to run
	// +kubebuilder:validation:Required
	Image string `json:"image"`

	// Port is the container port to expose
	// +kubebuilder:default=8080
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port,omitempty"`
}

// SimpleApp is the Schema for the simpleapps API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=sa
// +kubebuilder:printcolumn:name="Image",type=string,JSONPath=`.spec.image`
// +kubebuilder:printcolumn:name="Replicas",type=integer,JSONPath=`.spec.replicas`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type SimpleApp struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SimpleAppSpec `json:"spec,omitempty"`
	Status status.Status `json:"status,omitempty"`
}

// GetStatus implements status.Accessor
func (s *SimpleApp) GetStatus() *status.Status {
	return &s.Status
}

// SetStatus implements status.Accessor
func (s *SimpleApp) SetStatus(st *status.Status) {
	if st != nil {
		s.Status = *st
	}
}

// SimpleAppList contains a list of SimpleApp
// +kubebuilder:object:root=true
type SimpleAppList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SimpleApp `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SimpleApp{}, &SimpleAppList{})
}
