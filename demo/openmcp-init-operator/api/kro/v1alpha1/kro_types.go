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
	KROPhaseProvisioning = "Provisioning"
	KROPhaseReady        = "Ready"
)

// KROSpec defines the desired state of a KRO instance.
type KROSpec struct {
	// Version is the upstream application version (e.g. "v0.8.5").
	Version string `json:"version"`
	// ChartVersion is the helm chart version/tag to install (e.g. "0.8.5").
	// +optional
	ChartVersion string `json:"chartVersion,omitempty"`
}

// KROStatus defines the observed state of a KRO instance.
type KROStatus struct {
	commonapi.Status `json:",inline"`
}

// KRO is the Schema for the kros API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:JSONPath=`.spec.version`,name="Version",type=string
// +kubebuilder:printcolumn:JSONPath=`.status.phase`,name="Phase",type=string
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:metadata:labels="opencp.cloud/cluster=onboarding"
type KRO struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of KRO
	// +required
	Spec KROSpec `json:"spec"`

	// status defines the observed state of KRO
	// +optional
	Status KROStatus `json:"status,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// KROList contains a list of KRO
type KROList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KRO `json:"items"`
}

func (k *KRO) GetVersion() string {
	return k.Spec.Version
}

func (k *KRO) GetChartVersion() string {
	return k.Spec.ChartVersion
}

func (k *KRO) SetPhase(phase string) {
	k.Status.Phase = phase
}

func init() {
	SchemeBuilder.Register(&KRO{}, &KROList{})
}
