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

package management

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Mirantis/hmc/api/v1alpha1"
)

const (
	DefaultName = "hmc"
)

type Opt func(management *v1alpha1.Management)

func NewManagement(opts ...Opt) *v1alpha1.Management {
	p := &v1alpha1.Management{
		ObjectMeta: metav1.ObjectMeta{
			Name: DefaultName,
		},
	}

	for _, opt := range opts {
		opt(p)
	}
	return p
}

func WithName(name string) Opt {
	return func(p *v1alpha1.Management) {
		p.Name = name
	}
}

func WithCoreComponents(core *v1alpha1.Core) Opt {
	return func(p *v1alpha1.Management) {
		p.Spec.Core = core
	}
}

func WithProviders(providers []v1alpha1.Component) Opt {
	return func(p *v1alpha1.Management) {
		p.Spec.Providers = providers
	}
}

func WithAvailableProviders(providers v1alpha1.Providers) Opt {
	return func(p *v1alpha1.Management) {
		p.Status.AvailableProviders = providers
	}
}

func WithComponentsStatus(components map[string]v1alpha1.ComponentStatus) Opt {
	return func(p *v1alpha1.Management) {
		p.Status.Components = components
	}
}