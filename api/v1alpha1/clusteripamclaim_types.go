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

const ClusterIPAMClaimFinalizer = "k0rdent.mirantis.com/cluster-ipamclaim"

type NodeIPPoolSpec struct {
	v1.TypedLocalObjectReference `json:",inline"`
	NodeCount                    int `json:"nodeCount"`
}

// ClusterIPAMClaimSpec defines the desired state of ClusterIPAMClaim
type ClusterIPAMClaimSpec struct {
	// The provider that this claim will be consumed by
	Provider string `json:"provider,omitempty"`

	NodeIPPool NodeIPPoolSpec `json:"nodeIPPool,omitempty"`

	ClusterIPPool v1.TypedLocalObjectReference `json:"clusterIPPool,omitempty"`

	ExternalIPPool v1.TypedLocalObjectReference `json:"externalIPPool,omitempty"`
}

// ClusterIPAMClaimStatus defines the observed state of ClusterIPAMClaim
type ClusterIPAMClaimStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// ClusterIPAMClaim is the Schema for the clusteripamclaims API
type ClusterIPAMClaim struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterIPAMClaimSpec   `json:"spec,omitempty"`
	Status ClusterIPAMClaimStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterIPAMClaimList contains a list of ClusterIPAMClaim
type ClusterIPAMClaimList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterIPAMClaim `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterIPAMClaim{}, &ClusterIPAMClaimList{})
}
