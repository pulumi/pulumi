// Copyright 2016-2018, Pulumi Corporation.
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
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/deploy/providers"
	"github.com/pulumi/pulumi/pkg/resource/plugin"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/workspace"
)

func TestRefresh(t *testing.T) {
	proj := &workspace.Project{Name: "test"}
	target := &Target{Name: "test"}

	newURN := func(t tokens.Type, name string, parent resource.URN) resource.URN {
		var pt tokens.Type
		if parent != "" {
			pt = parent.Type()
		}
		return resource.NewURN(target.Name, proj.Name, pt, t, tokens.QName(name))
	}

	newProviderURN := func(pkg tokens.Package, name string, parent resource.URN) resource.URN {
		return newURN(providers.MakeProviderType(pkg), name, parent)
	}

	componentURN := newURN("component", "component", "")
	resAURN := newURN("pkgA:m:typA", "resA", "")
	resBURN := newURN("pkgA:m:typB", "resB", "")
	resCURN := newURN("pkgC:m:typC", "resC", "")

	providerARef, err := providers.NewReference(newProviderURN("pkgA", "providerA", ""), "id1")
	assert.NoError(t, err)
	providerBRef, err := providers.NewReference(newProviderURN("pkgA", "providerB", componentURN), "id2")
	assert.NoError(t, err)
	providerCRef, err := providers.NewReference(newProviderURN("pkgC", "providerC", ""), "id1")
	assert.NoError(t, err)

	newProviderState := func(ref providers.Reference) *resource.State {
		return &resource.State{
			Type:   ref.URN().Type(),
			URN:    ref.URN(),
			Custom: true,
			ID:     ref.ID(),
			Inputs: resource.PropertyMap{},
		}
	}

	newState := func(urn resource.URN, id resource.ID, provider string) *resource.State {
		custom := id != ""
		return &resource.State{
			Type:     urn.Type(),
			URN:      urn,
			Custom:   custom,
			ID:       id,
			Provider: provider,
			Inputs:   resource.PropertyMap{},
		}
	}

	reads := int32(0)
	noopProvider := &testProvider{
		read: func(resource.URN, resource.ID, resource.PropertyMap) (resource.PropertyMap, error) {
			atomic.AddInt32(&reads, 1)
			return resource.PropertyMap{}, nil
		},
	}

	providerSource := &testProviderSource{
		providers: map[providers.Reference]plugin.Provider{
			providerARef: noopProvider,
			providerBRef: noopProvider,
			providerCRef: noopProvider,
		},
	}

	olds := []*resource.State{
		// One top-level provider from package A
		newProviderState(providerARef),
		// One component resource
		newState(componentURN, "", ""),
		// One nested provider from package A
		newProviderState(providerBRef),
		// One resource referencing provider A
		newState(resAURN, "id1", providerARef.String()),
		// One resource referencing provider B
		newState(resBURN, "id2", providerBRef.String()),
		// A top-level provider from package C
		newProviderState(providerCRef),
		// One resource refernecing provider C
		newState(resCURN, "id3", providerCRef.String()),
	}
	target.Snapshot = &Snapshot{Resources: olds}

	// Create and iterate a source.
	iter, err := NewRefreshSource(nil, proj, target, false).Iterate(Options{}, providerSource)
	assert.NoError(t, err)

	processed := 0
	for {
		event, err := iter.Next()
		assert.NoError(t, err)
		if event == nil {
			break
		}

		processed++
	}
	assert.Equal(t, len(olds), processed)

	expectedRead := 0
	for _, s := range olds {
		if s.Custom && !providers.IsProviderType(s.Type) {
			expectedRead++
		}
	}
	assert.Equal(t, expectedRead, int(reads))
}
