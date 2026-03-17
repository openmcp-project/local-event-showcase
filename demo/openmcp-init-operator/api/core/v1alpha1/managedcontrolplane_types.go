/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	commonapi "github.com/openmcp-project/openmcp-operator/api/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ManagedControlPlanePhaseProvisioning = "Provisioning"
	ManagedControlPlanePhaseMCPReady     = "MCPReady"
	ManagedControlPlanePhaseReady        = "Ready"
)

// ManagedControlPlaneSpec defines the desired state of a ManagedControlPlane.
type ManagedControlPlaneSpec struct{}

// ManagedControlPlaneStatus defines the observed state of a ManagedControlPlane.
type ManagedControlPlaneStatus struct {
	commonapi.Status `json:",inline"`
}

// ManagedControlPlane is the Schema for the managedcontrolplanes API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:JSONPath=`.status.phase`,name="Phase",type=string
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:metadata:labels="opencp.cloud/cluster=onboarding"
type ManagedControlPlane struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of ManagedControlPlane
	// +optional
	Spec ManagedControlPlaneSpec `json:"spec,omitempty,omitzero"`

	// status defines the observed state of ManagedControlPlane
	// +optional
	Status ManagedControlPlaneStatus `json:"status,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// ManagedControlPlaneList contains a list of ManagedControlPlane
type ManagedControlPlaneList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ManagedControlPlane `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ManagedControlPlane{}, &ManagedControlPlaneList{})
}
