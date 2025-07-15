// Copyright 2018-2024, Pulumi Corporation.
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

	"github.com/pulumi/pulumi/pkg/v3/secrets/b64"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	}, b64.NewBase64SecretsManager(), resources, ops, SnapshotMetadata{})
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

	_, err := NewDeployment(&plugin.Context{}, &Options{}, nil, &Target{}, snap, nil, NewNullSource("test"), nil, nil, nil)
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
		tt := tt
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			targets := NewUrnTargets([]string{tt.input})
			for _, urn := range tt.expected {
				assert.True(t, targets.Contains(urn))
			}
		})
	}
}
