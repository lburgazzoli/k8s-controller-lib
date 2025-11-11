package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/status"
)

// CleanupAppSpec defines the desired state of CleanupApp
type CleanupAppSpec struct {
	// ConfigMapName is the name of the ConfigMap to create
	// This ConfigMap will be created without owner references to demonstrate cleanup
	// +kubebuilder:validation:Required
	ConfigMapName string `json:"configMapName"`

	// Data contains the data to store in the ConfigMap
	// +kubebuilder:validation:Optional
	Data map[string]string `json:"data,omitempty"`
}

// CleanupApp is the Schema for the cleanupapps API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=ca
// +kubebuilder:printcolumn:name="ConfigMap",type=string,JSONPath=`.spec.configMapName`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type CleanupApp struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CleanupAppSpec `json:"spec,omitempty"`
	Status status.Status  `json:"status,omitempty"`
}

// GetStatus implements status.Accessor
func (c *CleanupApp) GetStatus() *status.Status {
	return &c.Status
}

// SetStatus implements status.Accessor
func (c *CleanupApp) SetStatus(st *status.Status) {
	if st != nil {
		c.Status = *st
	}
}

// CleanupAppList contains a list of CleanupApp
// +kubebuilder:object:root=true
type CleanupAppList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CleanupApp `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CleanupApp{}, &CleanupAppList{})
}
