// Copyright 2025
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

package adapter

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	inclusteripamv1alpha2 "sigs.k8s.io/cluster-api-ipam-provider-in-cluster/api/v1alpha2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kcm "github.com/K0rdent/kcm/api/v1beta1"
	"github.com/K0rdent/kcm/internal/utils"
)

const (
	IPAMInClusterAdapterName = "in-cluster"

	InClusterIPPoolKind = "InClusterIPPool"
)

type InClusterAdapter struct{}

func NewInClusterAdapter() *InClusterAdapter {
	return &InClusterAdapter{}
}

func (InClusterAdapter) BindAddress(ctx context.Context, config IPAMConfig, c client.Client) (kcm.ClusterIPAMProviderData, error) {
	ipAddresses := config.ClusterIPAMClaim.Spec.NodeNetwork.IPAddresses
	if len(ipAddresses) == 0 {
		ipAddresses = []string{config.ClusterIPAMClaim.Spec.NodeNetwork.CIDR}
	}

	pool := inclusteripamv1alpha2.InClusterIPPool{
		ObjectMeta: metav1.ObjectMeta{Name: config.ClusterIPAMClaim.Name, Namespace: config.ClusterIPAMClaim.Namespace},
	}

	config.ClusterIPAMClaim.Kind = kcm.ClusterIPAMClaimKind
	config.ClusterIPAMClaim.APIVersion = kcm.GroupVersion.Version
	utils.AddOwnerReference(&pool, config.ClusterIPAMClaim)
	_, err := ctrl.CreateOrUpdate(ctx, c, &pool, func() error {
		pool.Spec = inclusteripamv1alpha2.InClusterIPPoolSpec{
			Addresses: ipAddresses,
		}
		return nil
	})
	if err != nil {
		return kcm.ClusterIPAMProviderData{}, fmt.Errorf("failed to create or update ip pool resource: %w", err)
	}

	poolAPIGroup := inclusteripamv1alpha2.GroupVersion.String()
	poolRef := corev1.TypedLocalObjectReference{
		APIGroup: &poolAPIGroup,
		Kind:     InClusterIPPoolKind,
		Name:     config.ClusterIPAMClaim.Name,
	}

	poolData, err := json.Marshal(poolRef)
	if err != nil {
		return kcm.ClusterIPAMProviderData{}, fmt.Errorf("failed to marshal ip pool data: %w", err)
	}

	return kcm.ClusterIPAMProviderData{
		Name:  ClusterDeploymentConfigKeyName,
		Data:  &apiextensionsv1.JSON{Raw: poolData},
		Ready: pool.Status.Addresses != nil && pool.Status.Addresses.Total > 0,
	}, nil
}
