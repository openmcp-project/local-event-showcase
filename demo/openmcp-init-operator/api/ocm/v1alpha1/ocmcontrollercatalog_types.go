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

// OCMControllerCatalogVersion represents an available OCM Controller version.
type OCMControllerCatalogVersion struct {
	// Version is the upstream application version (e.g. "v0.29.0").
	Version string `json:"version"`
	// ChartVersion is the helm chart version/tag (e.g. "0.0.0-6205a8a").
	// +optional
	ChartVersion string `json:"chartVersion,omitempty"`
}

// OCMControllerCatalogSpec defines the available OCM Controller versions.
type OCMControllerCatalogSpec struct {
	// Available OCM Controller versions.
	Versions []OCMControllerCatalogVersion `json:"versions"`
}

// OCMControllerCatalog is a read-only catalog of available OCM Controller versions.
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
type OCMControllerCatalog struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the available OCM Controller versions
	// +required
	Spec OCMControllerCatalogSpec `json:"spec"`
}

// +kubebuilder:object:root=true

// OCMControllerCatalogList contains a list of OCMControllerCatalog
type OCMControllerCatalogList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OCMControllerCatalog `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OCMControllerCatalog{}, &OCMControllerCatalogList{})
}
