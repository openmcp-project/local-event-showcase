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

// CrossplaneCatalogVersion represents an available Crossplane version.
type CrossplaneCatalogVersion struct {
	// Version string (e.g. "v1.20.1").
	Version string `json:"version"`
}

// CrossplaneCatalogProvider represents an available Crossplane provider with its versions.
type CrossplaneCatalogProvider struct {
	// Name of the provider (e.g. "provider-kubernetes").
	Name string `json:"name"`

	// Available versions for this provider.
	Versions []string `json:"versions"`
}

// CrossplaneCatalogSpec defines the available Crossplane versions and providers.
type CrossplaneCatalogSpec struct {
	// Available Crossplane versions.
	Versions []CrossplaneCatalogVersion `json:"versions"`

	// Available Crossplane providers.
	Providers []CrossplaneCatalogProvider `json:"providers"`
}

// CrossplaneCatalog is a read-only catalog of available Crossplane versions and providers.
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
type CrossplaneCatalog struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the available Crossplane versions and providers
	// +required
	Spec CrossplaneCatalogSpec `json:"spec"`
}

// +kubebuilder:object:root=true

// CrossplaneCatalogList contains a list of CrossplaneCatalog
type CrossplaneCatalogList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CrossplaneCatalog `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CrossplaneCatalog{}, &CrossplaneCatalogList{})
}
