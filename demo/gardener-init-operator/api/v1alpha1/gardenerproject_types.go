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
	GardenerProjectPhaseProvisioning = "Provisioning"
	GardenerProjectPhaseProjectReady = "ProjectReady"
	GardenerProjectPhaseReady        = "Ready"
)

// GardenerProjectSpec defines the desired state of a GardenerProject.
type GardenerProjectSpec struct{}

// GardenerProjectStatus defines the observed state of a GardenerProject.
type GardenerProjectStatus struct {
	commonapi.Status `json:",inline"`

	// ProjectName is the name of the Gardener Project created for this workspace.
	// Set by the operator after creation via generateName.
	// +optional
	ProjectName string `json:"projectName,omitempty"`
}

// GardenerProject is the Schema for the gardenerprojects API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:JSONPath=`.status.phase`,name="Phase",type=string
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:metadata:labels="openmcp.cloud/cluster=onboarding"
type GardenerProject struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of GardenerProject
	// +optional
	Spec GardenerProjectSpec `json:"spec,omitempty,omitzero"`

	// status defines the observed state of GardenerProject
	// +optional
	Status GardenerProjectStatus `json:"status,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// GardenerProjectList contains a list of GardenerProject
type GardenerProjectList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GardenerProject `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GardenerProject{}, &GardenerProjectList{})
}
