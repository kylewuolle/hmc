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

package unmanagedcluster

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	hmc "github.com/Mirantis/hmc/api/v1alpha1"
)

type Opt func(unmanagedCluster *hmc.UnmanagedCluster)

const (
	DefaultName = "hmc-uc"
)

func NewUnmanagedCluster(opts ...Opt) *hmc.UnmanagedCluster {
	uc := &hmc.UnmanagedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: DefaultName,
		},
		Spec: hmc.UnmanagedClusterSpec{},
	}

	for _, opt := range opts {
		opt(uc)
	}
	return uc
}

func WithNameAndNamespace(name, namespace string) Opt {
	return func(uc *hmc.UnmanagedCluster) {
		uc.Name = name
		uc.Namespace = namespace
	}
}
