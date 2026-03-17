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
	OCMControllerPhaseProvisioning = "Provisioning"
	OCMControllerPhaseReady        = "Ready"
)

// OCMControllerSpec defines the desired state of an OCM Controller instance.
type OCMControllerSpec struct {
	// Version is the upstream application version (e.g. "v0.29.0").
	Version string `json:"version"`
	// ChartVersion is the helm chart version/tag to install (e.g. "0.0.0-6205a8a").
	// +optional
	ChartVersion string `json:"chartVersion,omitempty"`
}

// OCMControllerStatus defines the observed state of an OCM Controller instance.
type OCMControllerStatus struct {
	commonapi.Status `json:",inline"`
}

// OCMController is the Schema for the ocmcontrollers API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:JSONPath=`.spec.version`,name="Version",type=string
// +kubebuilder:printcolumn:JSONPath=`.status.phase`,name="Phase",type=string
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:metadata:labels="openmcp.cloud/cluster=onboarding"
type OCMController struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of the OCM Controller
	// +required
	Spec OCMControllerSpec `json:"spec"`

	// status defines the observed state of the OCM Controller
	// +optional
	Status OCMControllerStatus `json:"status,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// OCMControllerList contains a list of OCMController
type OCMControllerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OCMController `json:"items"`
}

func (o *OCMController) GetVersion() string {
	return o.Spec.Version
}

func (o *OCMController) GetChartVersion() string {
	return o.Spec.ChartVersion
}

func (o *OCMController) SetPhase(phase string) {
	o.Status.Phase = phase
}

func init() {
	SchemeBuilder.Register(&OCMController{}, &OCMControllerList{})
}
