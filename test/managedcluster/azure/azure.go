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

package azure

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"

	"github.com/a8m/envsubst"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/restmapper"

	"github.com/Mirantis/hmc/test/kubeclient"
)

func CreateCredentialSecret(ctx context.Context, kc *kubeclient.KubeClient) error {
	serializer := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	yamlFile, err := os.ReadFile("./config/dev/azure-credentials.yaml")

	if err != nil {
		return fmt.Errorf("failed to read azure credential file: %w", err)
	}

	yamlFile, err = envsubst.Bytes(yamlFile)
	if err != nil {
		return fmt.Errorf("failed to process azure credential file: %w", err)
	}

	c := discovery.NewDiscoveryClientForConfigOrDie(kc.Config)
	groupResources, err := restmapper.GetAPIGroupResources(c)
	if err != nil {
		return fmt.Errorf("failed to fetch group resources: %w", err)
	}

	yamlReader := yamlutil.NewYAMLReader(bufio.NewReader(bytes.NewReader(yamlFile)))
	for {
		yamlDoc, err := yamlReader.Read()

		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed to process azure credential file: %w", err)

		}

		credentialResource := &unstructured.Unstructured{}
		_, _, err = serializer.Decode(yamlDoc, nil, credentialResource)
		if err != nil {
			return fmt.Errorf("failed to deserialize azure credential object: %w", err)
		}

		mapper := restmapper.NewDiscoveryRESTMapper(groupResources)
		mapping, err := mapper.RESTMapping(credentialResource.GroupVersionKind().GroupKind())

		if err != nil {
			return fmt.Errorf("failed to create rest mapper: %w", err)
		}

		dc := kc.GetDynamicClient(schema.GroupVersionResource{
			Group:    credentialResource.GroupVersionKind().Group,
			Version:  credentialResource.GroupVersionKind().Version,
			Resource: mapping.Resource.Resource,
		})

		exists, err := dc.Get(ctx, credentialResource.GetName(), metav1.GetOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to check for existing credential: %w", err)
		}

		if exists == nil {
			if _, err = dc.Create(ctx, credentialResource, metav1.CreateOptions{}); err != nil {
				return fmt.Errorf("failed to create azure credentials: %w", err)
			}
		}
	}

	return nil
}
