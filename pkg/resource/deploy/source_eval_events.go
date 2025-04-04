// Copyright 2025, Pulumi Corporation.
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

package deploy

import (
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type RegisterProviderExtensionEvent interface {
	SourceEvent

	ProviderReference() providers.Reference

	Extension() *workspace.Parameterization

	Done()
}

type registerProviderExtensionEvent struct {
	providerReference providers.Reference
	extension         *workspace.Parameterization
	done              chan bool
}

var _ RegisterProviderExtensionEvent = (*registerProviderExtensionEvent)(nil)

func (g *registerProviderExtensionEvent) event() {}

func (g *registerProviderExtensionEvent) ProviderReference() providers.Reference {
	return g.providerReference
}

func (g *registerProviderExtensionEvent) Extension() *workspace.Parameterization {
	return g.extension
}

func (g *registerProviderExtensionEvent) Done() {
	g.done <- true
}
