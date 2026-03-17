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

// FluxCatalogVersion represents an available Flux version.
type FluxCatalogVersion struct {
	// Version is the upstream application version (e.g. "v2.4.0").
	Version string `json:"version"`
	// ChartVersion is the helm chart version/tag (e.g. "2.14.0").
	// +optional
	ChartVersion string `json:"chartVersion,omitempty"`
}

// FluxCatalogSpec defines the available Flux versions.
type FluxCatalogSpec struct {
	// Available Flux versions.
	Versions []FluxCatalogVersion `json:"versions"`
}

// FluxCatalog is a read-only catalog of available Flux versions.
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
type FluxCatalog struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the available Flux versions
	// +required
	Spec FluxCatalogSpec `json:"spec"`
}

// +kubebuilder:object:root=true

// FluxCatalogList contains a list of FluxCatalog
type FluxCatalogList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FluxCatalog `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FluxCatalog{}, &FluxCatalogList{})
}
