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
	kcmv1 "github.com/K0rdent/kcm/api/v1beta1"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	corev1 "k8s.io/api/core/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"slices"
)

type AWSAdapter struct{}

func NewAWSAdapter() *AWSAdapter {
	return &AWSAdapter{}
}

type AWSNetworkInterfaceInfo struct {
	NetworkInterfaceId string `json:"networkInterfaceId,omitempty"`
	IP                 string `json:"ip,omitempty"`
}
type AWSAdapterData struct {
	SubnetId          string                    `json:"subnetId,omitempty"`
	NetworkInterfaces []AWSNetworkInterfaceInfo `json:"networkInterfaces,omitempty"`
}

func (AWSAdapter) BindAddress(ctx context.Context, config IPAMConfig, c client.Client) (kcmv1.ClusterIPAMProviderData, error) {
	const subnetId = "subnet-04aa20824fd313eb3"
	clusterDeployment := kcmv1.ClusterDeployment{}
	err := c.Get(ctx, client.ObjectKey{
		Name:      config.ClusterIPAMClaim.Spec.Cluster,
		Namespace: config.ClusterIPAMClaim.Namespace,
	}, &clusterDeployment)
	if err != nil {
		return kcmv1.ClusterIPAMProviderData{}, fmt.Errorf("failed to fetch ClusterDeployment: %s: %w", config.ClusterIPAMClaim.Spec.Cluster, err)
	}

	clusterIPAM := kcmv1.ClusterIPAM{}
	err = c.Get(ctx, client.ObjectKey{
		Name:      config.ClusterIPAMClaim.Name,
		Namespace: config.ClusterIPAMClaim.Namespace,
	}, &clusterIPAM)

	if client.IgnoreNotFound(err) != nil {
		return kcmv1.ClusterIPAMProviderData{}, fmt.Errorf("failed to fetch clusterIPAM: %s: %w", config.ClusterIPAMClaim.Name, err)
	}

	interfacesToCreate := config.ClusterIPAMClaim.Spec.NodeNetwork.IPAddresses

	adapterData := AWSAdapterData{}

	if clusterIPAM.Status.ProviderData != nil {
		for _, providerData := range clusterIPAM.Status.ProviderData {
			err = json.Unmarshal(providerData.Data.Raw, &adapterData)
			if err != nil {
				return kcmv1.ClusterIPAMProviderData{}, fmt.Errorf("failed to fetch unmarshal json: %w", err)
			}

			for _, createdInterfaces := range adapterData.NetworkInterfaces {
				if slices.Contains(interfacesToCreate, createdInterfaces.IP) {
					index := slices.Index(interfacesToCreate, createdInterfaces.IP)
					interfacesToCreate = slices.Delete(interfacesToCreate, index, index+1)
				}
			}
		}
	}

	cred := &kcmv1.Credential{}
	err = c.Get(ctx, client.ObjectKey{
		Name:      clusterDeployment.Spec.Credential,
		Namespace: clusterDeployment.Namespace,
	}, cred)

	if err != nil {
		return kcmv1.ClusterIPAMProviderData{}, fmt.Errorf("failed to fetch Credential: %w", err)
	}

	awsIdentity := &unstructured.Unstructured{}
	awsIdentity.SetAPIVersion(cred.Spec.IdentityRef.APIVersion)
	awsIdentity.SetKind(cred.Spec.IdentityRef.Kind)
	awsIdentity.SetName(cred.Spec.IdentityRef.Name)
	awsIdentity.SetNamespace(cred.Spec.IdentityRef.Namespace)
	if err := c.Get(ctx, client.ObjectKey{
		Name:      cred.Spec.IdentityRef.Name,
		Namespace: cred.Spec.IdentityRef.Namespace,
	}, awsIdentity); err != nil {
		return kcmv1.ClusterIPAMProviderData{}, fmt.Errorf("failed to fetch credential identity reference: %w", err)
	}

	secretRef, found, err := unstructured.NestedString(awsIdentity.Object, "spec", "secretRef")
	if err != nil {
		return kcmv1.ClusterIPAMProviderData{}, err
	}

	if !found {
		return kcmv1.ClusterIPAMProviderData{}, fmt.Errorf("missing spec.secretRef")
	}

	secret := &corev1.Secret{}
	if err := c.Get(ctx, client.ObjectKey{Name: secretRef, Namespace: clusterDeployment.Namespace}, secret); err != nil {
		return kcmv1.ClusterIPAMProviderData{}, fmt.Errorf("failed to fetch secret: %w", err)
	}

	accessKeyID := string(secret.Data["AccessKeyID"])
	secretAccessKey := string(secret.Data["SecretAccessKey"])
	sessionToken := string(secret.Data["SessionToken"])

	awsConfig, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion("us-west-1"),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, sessionToken)))
	if err != nil {
		return kcmv1.ClusterIPAMProviderData{}, fmt.Errorf("failed to load aws credentials: %w", err)
	}

	svc := ec2.NewFromConfig(awsConfig)

	for _, ipAddress := range interfacesToCreate {
		networkInterfaceReq := ec2.CreateNetworkInterfaceInput{
			PrivateIpAddress: aws.String(ipAddress),
			SubnetId:         aws.String(subnetId), // todo
		}

		output, err := svc.CreateNetworkInterface(ctx, &networkInterfaceReq)
		if err != nil {
			return kcmv1.ClusterIPAMProviderData{}, fmt.Errorf("failed to create network interface: %w", err)
		}

		adapterData.SubnetId = subnetId
		adapterData.NetworkInterfaces = append(adapterData.NetworkInterfaces, AWSNetworkInterfaceInfo{
			NetworkInterfaceId: *output.NetworkInterface.NetworkInterfaceId,
			IP:                 ipAddress,
		})
	}

	providerData := kcmv1.ClusterIPAMProviderData{}

	var jsonData []byte
	jsonData, err = json.Marshal(adapterData)
	if err != nil {
		return kcmv1.ClusterIPAMProviderData{}, fmt.Errorf("failed to create marshal json: %w", err)
	}
	providerData.Name = ClusterDeploymentConfigKeyName
	providerData.Ready = true
	providerData.Data = &apiextv1.JSON{Raw: jsonData}
	return providerData, nil

}
