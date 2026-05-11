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
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/secrets/b64"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	sdkproviders "github.com/pulumi/pulumi/sdk/v3/go/common/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/version"
)

func newResource(name string) *resource.State {
	ty := tokens.Type("test")
	return &resource.State{
		Type:    ty,
		URN:     resource.NewURN(tokens.QName("teststack"), tokens.PackageName("pkg"), ty, ty, name),
		Inputs:  make(resource.PropertyMap),
		Outputs: make(resource.PropertyMap),
	}
}

func newSnapshot(resources []*resource.State, ops []resource.Operation) *Snapshot {
	return NewSnapshot(Manifest{
		Time:    time.Now(),
		Version: version.Version,
		Plugins: nil,
	}, b64.NewBase64SecretsManager(), resources, ops, SnapshotMetadata{}, nil)
}

func TestPendingOperationsDeployment(t *testing.T) {
	t.Parallel()

	resourceA := newResource("a")
	resourceB := newResource("b")
	snap := newSnapshot([]*resource.State{
		resourceA,
	}, []resource.Operation{
		{
			Type:     resource.OperationTypeCreating,
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
	provA := makeProviderRef(t, "k8s")
	provB := makeProviderRef(t, "azure")
	extensionA := apitype.ExtensionRef("extension-a")
	extensionB := apitype.ExtensionRef("extension-b")

	// First call: nothing registered yet -> we get a CompletionSource to fulfill.
	existing1, created1 := d.LookupOrRegisterExtension(provA, extensionA)
	require.Nil(t, existing1, "first call should not return an existing promise")
	require.NotNil(t, created1, "first call should return a fresh CompletionSource")

	// Second call for the same (provider, ref): we get the existing promise to wait on.
	// The CompletionSource is nil because we are NOT the one doing the work.
	existing2, created2 := d.LookupOrRegisterExtension(provA, extensionA)
	require.Nil(t, created2, "duplicate registration must not mint a second CompletionSource")
	require.NotNil(t, existing2, "duplicate registration must hand back the in-flight promise")

	// Verify the returned promise is the one tied to our original CompletionSource:
	// fulfilling created should make existing2 resolve.
	created1.MustFulfill(struct{}{})
	_, err := existing2.Result(context.Background())
	require.NoError(t, err, "existing promise should resolve after the first caller fulfills")

	// Different ref under the same provider -> separate entry, new CompletionSource.
	existingY, createdY := d.LookupOrRegisterExtension(provA, extensionB)
	require.Nil(t, existingY)
	require.NotNil(t, createdY, "different ref under same provider must be tracked independently")

	// Same ref under a different provider -> separate entry, new CompletionSource.
	existingB, createdB := d.LookupOrRegisterExtension(provB, extensionA)
	require.Nil(t, existingB)
	require.NotNil(t, createdB, "same ref under a different provider must be tracked independently")
}
