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

	"github.com/pulumi/pulumi/pkg/v3/secrets/b64"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/promise"
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
	}, b64.NewBase64SecretsManager(), resources, ops, SnapshotMetadata{}, nil, nil)
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

	// makeStep captures the CompletionSource the first caller is handed.
	var firstCTS *promise.CompletionSource[struct{}]
	makeStep := func(cts *promise.CompletionSource[struct{}]) Step {
		firstCTS = cts
		return &ExtensionParameterizeStep{}
	}

	// First call: nothing registered yet -> makeStep runs and we get a step to emit.
	step1, promise1 := d.LookupOrRegisterExtension(provA, extensionA, makeStep)
	require.NotNil(t, step1, "first call should return a step to emit")
	require.NotNil(t, promise1)
	require.NotNil(t, firstCTS, "first call should hand makeStep a CompletionSource")

	// Second call for the same (provider, ref): no step, and makeStep must not run.
	calledAgain := false
	step2, promise2 := d.LookupOrRegisterExtension(provA, extensionA,
		func(*promise.CompletionSource[struct{}]) Step {
			calledAgain = true
			return nil
		})
	require.Nil(t, step2, "duplicate registration must not emit a second step")
	require.False(t, calledAgain, "duplicate registration must not invoke makeStep")
	require.NotNil(t, promise2, "duplicate registration must hand back the in-flight promise")

	// Fulfilling the first caller's CompletionSource resolves the duplicate's promise.
	firstCTS.MustFulfill(struct{}{})
	_, err := promise2.Result(t.Context())
	require.NoError(t, err, "duplicate's promise should resolve after the first caller fulfills")

	// Different ref under the same provider -> separate entry, its own step.
	stepY, _ := d.LookupOrRegisterExtension(provA, extensionB, makeStep)
	require.NotNil(t, stepY, "different ref under same provider must be tracked independently")

	// Same ref under a different provider -> separate entry, its own step.
	stepB, _ := d.LookupOrRegisterExtension(provB, extensionA, makeStep)
	require.NotNil(t, stepB, "same ref under a different provider must be tracked independently")
}
