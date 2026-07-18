// Copyright 2018, Pulumi Corporation.
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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/pkg/v3/secrets/b64"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	sdkproviders "github.com/pulumi/pulumi/sdk/v3/go/common/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/version"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

func newResource(name string) *pkgresource.State {
	ty := tokens.Type("test")
	return &pkgresource.State{
		Type:    ty,
		URN:     resource.NewURN(tokens.QName("teststack"), tokens.PackageName("pkg"), ty, ty, name),
		Inputs:  property.Map{},
		Outputs: property.Map{},
	}
}

func newSnapshot(resources []*pkgresource.State, ops []pkgresource.Operation) *Snapshot {
	return NewSnapshot(Manifest{
		Time:    time.Now(),
		Version: version.Version,
		Plugins: nil,
	}, b64.NewBase64SecretsManager(), resources, ops, SnapshotMetadata{}, nil, nil)
}

func TestPendingOperationsDeployment(t *testing.T) {
	t.Parallel()

	resourceA := newResource("a")
	resourceB := newResource("b")
	snap := newSnapshot([]*pkgresource.State{
		resourceA,
	}, []pkgresource.Operation{
		{
			Type:     pkgresource.OperationTypeCreating,
			Resource: resourceB,
		},
	})

	_, err := NewDeployment(&plugin.Context{}, &Options{}, nil, &Target{}, snap, nil, NewNullSource("test"), nil, nil)
	require.NoError(t, err)
}

func TestGlobUrn(t *testing.T) {
	t.Parallel()

	globs := []struct {
		input      string
		expected   []resource.URN
		unexpected []resource.URN
	}{
		{
			input: "**",
			expected: []resource.URN{
				"urn:pulumi:stack::test::typ$aws:resource::aname",
				"urn:pulumi:stack::test::typ$aws:resource::bar",
				"urn:pulumi:stack::test::typ$azure:resource::bar",
			},
		},
		{
			input: "urn:pulumi:stack::test::typ*:resource::bar",
			expected: []resource.URN{
				"urn:pulumi:stack::test::typ$aws:resource::bar",
				"urn:pulumi:stack::test::typ$azure:resource::bar",
			},
			unexpected: []resource.URN{
				"urn:pulumi:stack::test::ty:resource::bar",
				"urn:pulumi:stack::test::type:resource::foobar",
			},
		},
		{
			input:      "**:aname",
			expected:   []resource.URN{"urn:pulumi:stack::test::typ$aws:resource::aname"},
			unexpected: []resource.URN{"urn:pulumi:stack::test::typ$aws:resource::somename"},
		},
		{
			input: "*:*:stack::test::typ$aws:resource::*",
			expected: []resource.URN{
				"urn:pulumi:stack::test::typ$aws:resource::aname",
				"urn:pulumi:stack::test::typ$aws:resource::bar",
			},
			unexpected: []resource.URN{
				"urn:pulumi:stack::test::typ$azure:resource::aname",
			},
		},
		{
			input:    "stack::test::typ$aws:resource::none",
			expected: []resource.URN{"stack::test::typ$aws:resource::none"},
			unexpected: []resource.URN{
				"stack::test::typ$aws:resource::nonee",
			},
		},
	}
	for _, tt := range globs {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			targets := NewUrnTargets([]string{tt.input})
			for _, urn := range tt.expected {
				assert.True(t, targets.Contains(urn))
			}
		})
	}
}

func makeProviderRef(t *testing.T, name string) sdkproviders.Reference {
	t.Helper()
	providerURN := resource.URN("urn:pulumi:stack::project::pulumi:providers:" + name + "::default")
	ref, err := sdkproviders.NewReference(providerURN, resource.ID("id-"+name))
	require.NoError(t, err)
	return ref
}

func TestLookupOrRegisterExtension(t *testing.T) {
	t.Parallel()

	d := &Deployment{
		extensions: map[sdkproviders.Reference][]inFlightExtension{},
	}
	k8sProvider := makeProviderRef(t, "k8s")
	azureProvider := makeProviderRef(t, "azure")
	extA := apitype.ExtensionRef("extension-a")
	extB := apitype.ExtensionRef("extension-b")

	// Registering an unseen (provider, ref) makes this caller the owner: no promise to
	// wait on, and a CompletionSource it is responsible for fulfilling.
	ownerWait, ownerSource := d.LookupOrRegisterExtension(k8sProvider, extA)
	require.Nil(t, ownerWait, "registering an unseen extension has no in-flight promise")
	require.NotNil(t, ownerSource, "registering an unseen extension yields a CompletionSource to fulfill")

	// Registering the same pair again makes this caller a waiter: the in-flight promise,
	// and no CompletionSource of its own.
	waiterWait, waiterSource := d.LookupOrRegisterExtension(k8sProvider, extA)
	require.NotNil(t, waiterWait, "a duplicate registration returns the in-flight promise to wait on")
	require.Nil(t, waiterSource, "a duplicate registration does not mint a second CompletionSource")

	// The waiter's promise resolves once the owner fulfills its CompletionSource.
	ownerSource.MustFulfill(struct{}{})
	_, err := waiterWait.Result(t.Context())
	require.NoError(t, err, "the waiter's promise resolves once the owner fulfills")

	otherRefWait, otherRefSource := d.LookupOrRegisterExtension(k8sProvider, extB)
	require.Nil(t, otherRefWait)
	require.NotNil(t, otherRefSource, "a different ref under the same provider registers independently")

	otherProviderWait, otherProviderSource := d.LookupOrRegisterExtension(azureProvider, extA)
	require.Nil(t, otherProviderWait)
	require.NotNil(t, otherProviderSource, "the same ref under a different provider registers independently")
}
