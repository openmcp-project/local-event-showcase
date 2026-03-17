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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// KROCatalogVersion represents an available KRO version.
type KROCatalogVersion struct {
	// Version is the upstream application version (e.g. "v0.8.5").
	Version string `json:"version"`
	// ChartVersion is the helm chart version/tag (e.g. "0.8.5").
	// +optional
	ChartVersion string `json:"chartVersion,omitempty"`
}

// KROCatalogSpec defines the available KRO versions.
type KROCatalogSpec struct {
	// Available KRO versions.
	Versions []KROCatalogVersion `json:"versions"`
}

// KROCatalog is a read-only catalog of available KRO versions.
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
type KROCatalog struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the available KRO versions
	// +required
	Spec KROCatalogSpec `json:"spec"`
}

// +kubebuilder:object:root=true

// KROCatalogList contains a list of KROCatalog
type KROCatalogList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KROCatalog `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KROCatalog{}, &KROCatalogList{})
}
