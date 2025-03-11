// Copyright 2024
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClusterIPAMSpec defines the desired state of ClusterIPAM
type ClusterIPAMSpec struct {
	// The provider that this claim will be consumed by
	Provider string `json:"provider,omitempty"`
}

// ClusterIPAMStatus defines the observed state of ClusterIPAM
type ClusterIPAMStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// ClusterIPAM is the Schema for the clusteripams API
type ClusterIPAM struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterIPAMSpec   `json:"spec,omitempty"`
	Status ClusterIPAMStatus `json:"status,omitempty"`

	ClusterIPAMClaimRef v1.ObjectReference `json:"clusterIPAMClaimRef,omitempty"`

	NodeIPClaims []v1.TypedLocalObjectReference `json:"nodeIPClaims,omitempty"`

	ClusterIPClaims []v1.TypedLocalObjectReference `json:"clusterIPClaims,omitempty"`

	ExternalIPClaims []v1.TypedLocalObjectReference `json:"ExternalIPClaims,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterIPAMList contains a list of ClusterIPAM
type ClusterIPAMList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterIPAM `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterIPAM{}, &ClusterIPAMList{})
}
