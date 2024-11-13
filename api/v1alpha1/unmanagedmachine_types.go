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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// UnmanagedMachineSpec defines the desired state of UnmanagedMachine
type UnmanagedMachineSpec struct {
	ProviderID   string `json:"providerID,omitempty"`
	ClusterName  string `json:"clusterName,omitempty"`
	ControlPlane bool   `json:"controlPlane,omitempty"`
}

// UnmanagedMachineStatus defines the observed state of UnmanagedMachine
type UnmanagedMachineStatus struct {
	// Flag indicating whether the machine is in the ready state or not
	// +kubebuilder:default:=false
	Ready bool `json:"ready,omitempty"`
	// Conditions contains details for the current state of the ManagedCluster
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="Machine ready status"
// +kubebuilder:metadata:labels=cluster.x-k8s.io/v1beta1=v1alpha1

// UnmanagedMachine is the Schema for the unmanagedmachines API
type UnmanagedMachine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UnmanagedMachineSpec   `json:"spec,omitempty"`
	Status UnmanagedMachineStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// UnmanagedMachineList contains a list of UnmanagedMachine
type UnmanagedMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []UnmanagedMachine `json:"items"`
}

func init() {
	SchemeBuilder.Register(&UnmanagedMachine{}, &UnmanagedMachineList{})
}

func (in *UnmanagedMachine) GetConditions() *[]metav1.Condition {
	return &in.Status.Conditions
}
