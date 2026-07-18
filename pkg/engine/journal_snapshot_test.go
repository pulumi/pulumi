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

package engine

import (
	"testing"

	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/require"
)

func TestMustWrite(t *testing.T) {
	t.Parallel()

	defaultURN := resource.URN("urn:pulumi:test::stack::pulumi:providers:aws::default_4_42_0")

	cases := []struct {
		name      string
		old       *pkgresource.State
		new       *pkgresource.State
		mustWrite bool
	}{
		{
			name:      "changed URN",
			old:       &pkgresource.State{URN: defaultURN},
			new:       &pkgresource.State{URN: "urn:pulumi:test::stack::pulumi:providers:aws::default_4_42_1"},
			mustWrite: true,
		},
		{
			name:      "type changed",
			old:       &pkgresource.State{URN: defaultURN, Type: "pulumi:providers:aws"},
			new:       &pkgresource.State{URN: defaultURN, Type: "pulumi:providers:awsx"},
			mustWrite: true,
		},
		{
			name:      "custom changed",
			old:       &pkgresource.State{URN: defaultURN, Type: "pulumi:providers:aws", ID: "a", Custom: true},
			new:       &pkgresource.State{URN: defaultURN, Type: "pulumi:providers:aws", ID: "", Custom: false},
			mustWrite: true,
		},
		{
			name:      "custom timeout changed",
			old:       &pkgresource.State{URN: defaultURN, CustomTimeouts: resource.CustomTimeouts{Create: 600}},
			new:       &pkgresource.State{URN: defaultURN, CustomTimeouts: resource.CustomTimeouts{Create: 900}},
			mustWrite: true,
		},
		{
			name:      "RetainOnDelete changed",
			old:       &pkgresource.State{URN: defaultURN, RetainOnDelete: false},
			new:       &pkgresource.State{URN: defaultURN, RetainOnDelete: true},
			mustWrite: true,
		},
		{
			name: "Provider changed",
			old: &pkgresource.State{
				URN:      defaultURN,
				Provider: "urn:pulumi:test::stack::pulumi:providers:aws::default_4_42_0::provider-id-1",
			},
			new: &pkgresource.State{
				URN:      defaultURN,
				Provider: "urn:pulumi:test::stack::pulumi:providers:aws::default_4_42_0::provider-id-2",
			},
			mustWrite: true,
		},
		{
			name:      "Parent changed",
			old:       &pkgresource.State{URN: defaultURN, Parent: "urn:pulumi:test::stack::pulumi:pulumi:Stack::parent1"},
			new:       &pkgresource.State{URN: defaultURN, Parent: "urn:pulumi:test::stack::pulumi:pulumi:Stack::parent2"},
			mustWrite: true,
		},
		{
			name: "DeletedWith changed",
			old: &pkgresource.State{
				URN:         defaultURN,
				DeletedWith: "urn:pulumi:test::stack::pulumi:pulumi:Stack::resource1",
			},
			new: &pkgresource.State{
				URN:         defaultURN,
				DeletedWith: "urn:pulumi:test::stack::pulumi:pulumi:Stack::resource2",
			},
			mustWrite: true,
		},
		{
			name: "ReplaceWith changed - different length",
			old: &pkgresource.State{
				URN:         defaultURN,
				ReplaceWith: []resource.URN{"urn:pulumi:test::stack::pulumi:pulumi:Stack::resource1"},
			},
			new:       &pkgresource.State{URN: defaultURN, ReplaceWith: []resource.URN{}},
			mustWrite: true,
		},
		{
			name: "ReplaceWith changed - different content",
			old: &pkgresource.State{URN: defaultURN, ReplaceWith: []resource.URN{
				"urn:pulumi:test::stack::pulumi:pulumi:Stack::resource1",
			}},
			new: &pkgresource.State{URN: defaultURN, ReplaceWith: []resource.URN{
				"urn:pulumi:test::stack::pulumi:pulumi:Stack::resource2",
			}},
			mustWrite: true,
		},
		{
			name:      "Protect changed",
			old:       &pkgresource.State{URN: defaultURN, Protect: false},
			new:       &pkgresource.State{URN: defaultURN, Protect: true},
			mustWrite: true,
		},
		{
			name: "Inputs changed",
			old: &pkgresource.State{
				URN:    defaultURN,
				Inputs: property.NewMap(map[string]property.Value{"key": property.New("value1")}),
			},
			new: &pkgresource.State{
				URN:    defaultURN,
				Inputs: property.NewMap(map[string]property.Value{"key": property.New("value2")}),
			},
			mustWrite: true,
		},
		{
			name: "Outputs changed",
			old: &pkgresource.State{
				URN:     defaultURN,
				Outputs: property.NewMap(map[string]property.Value{"key": property.New("value1")}),
			},
			new: &pkgresource.State{
				URN:     defaultURN,
				Outputs: property.NewMap(map[string]property.Value{"key": property.New("value2")}),
			},
			mustWrite: true,
		},
		{
			name: "Dependencies changed - added dependency",
			old:  &pkgresource.State{URN: defaultURN, Dependencies: []resource.URN{}},
			new: &pkgresource.State{URN: defaultURN, Dependencies: []resource.URN{
				"urn:pulumi:test::stack::pulumi:pulumi:Stack::resource1",
			}},
			mustWrite: true,
		},
		{
			name: "Dependencies changed - removed dependency",
			old: &pkgresource.State{URN: defaultURN, Dependencies: []resource.URN{
				"urn:pulumi:test::stack::pulumi:pulumi:Stack::resource1",
			}},
			new:       &pkgresource.State{URN: defaultURN, Dependencies: []resource.URN{}},
			mustWrite: true,
		},
		{
			name: "Dependencies changed - different order (should not trigger write)",
			old: &pkgresource.State{URN: defaultURN, Dependencies: []resource.URN{
				"urn:pulumi:test::stack::pulumi:pulumi:Stack::resource1",
				"urn:pulumi:test::stack::pulumi:pulumi:Stack::resource2",
			}},
			new: &pkgresource.State{URN: defaultURN, Dependencies: []resource.URN{
				"urn:pulumi:test::stack::pulumi:pulumi:Stack::resource2",
				"urn:pulumi:test::stack::pulumi:pulumi:Stack::resource1",
			}},
			mustWrite: false,
		},
		{
			name: "PropertyDependencies changed - added property",
			old: &pkgresource.State{
				URN:                  defaultURN,
				PropertyDependencies: map[resource.PropertyKey][]resource.URN{},
			},
			new: &pkgresource.State{
				URN: defaultURN,
				PropertyDependencies: map[resource.PropertyKey][]resource.URN{
					"prop1": {"urn:pulumi:test::stack::pulumi:pulumi:Stack::resource1"},
				},
			},
			mustWrite: true,
		},
		{
			name: "PropertyDependencies changed - removed property",
			old: &pkgresource.State{
				URN: defaultURN,
				PropertyDependencies: map[resource.PropertyKey][]resource.URN{
					"prop1": {"urn:pulumi:test::stack::pulumi:pulumi:Stack::resource1"},
				},
			},
			new: &pkgresource.State{
				URN:                  defaultURN,
				PropertyDependencies: map[resource.PropertyKey][]resource.URN{},
			},
			mustWrite: true,
		},
		{
			name: "PropertyDependencies changed - missing property",
			old: &pkgresource.State{
				URN: defaultURN,
				PropertyDependencies: map[resource.PropertyKey][]resource.URN{
					"prop1": {"urn:pulumi:test::stack::pulumi:pulumi:Stack::resource1"},
				},
			},
			new: &pkgresource.State{
				URN: defaultURN,
				PropertyDependencies: map[resource.PropertyKey][]resource.URN{
					"prop2": {"urn:pulumi:test::stack::pulumi:pulumi:Stack::resource1"},
				},
			},
			mustWrite: true,
		},
		{
			name: "PropertyDependencies changed - different dependencies for same property",
			old: &pkgresource.State{
				URN: defaultURN,
				PropertyDependencies: map[resource.PropertyKey][]resource.URN{
					"prop1": {"urn:pulumi:test::stack::pulumi:pulumi:Stack::resource1"},
					"prop2": {"urn:pulumi:test::stack::pulumi:pulumi:Stack::resource1"},
				},
			},
			new: &pkgresource.State{
				URN: defaultURN,
				PropertyDependencies: map[resource.PropertyKey][]resource.URN{
					"prop1": {"urn:pulumi:test::stack::pulumi:pulumi:Stack::resource2"},
					"prop2": {"urn:pulumi:test::stack::pulumi:pulumi:Stack::resource1"},
				},
			},
			mustWrite: true,
		},
		{
			name: "ResourceHooks changed",
			old: &pkgresource.State{URN: defaultURN, ResourceHooks: map[resource.HookType][]string{
				resource.BeforeCreate: {"hook1"},
			}},
			new: &pkgresource.State{URN: defaultURN, ResourceHooks: map[resource.HookType][]string{
				resource.BeforeCreate: {"hook2"},
			}},
			mustWrite: true,
		},
		{
			name:      "SourcePosition changed - should not write",
			old:       &pkgresource.State{URN: defaultURN, SourcePosition: "pos1"},
			new:       &pkgresource.State{URN: defaultURN, SourcePosition: "pos2"},
			mustWrite: false,
		},
		{
			name: "nothing changed",
			old: &pkgresource.State{
				URN:          defaultURN,
				Type:         "pulumi:providers:aws",
				Inputs:       property.NewMap(map[string]property.Value{"key": property.New("value")}),
				Outputs:      property.NewMap(map[string]property.Value{"key": property.New("value")}),
				Dependencies: []resource.URN{"urn:pulumi:test::stack::pulumi:pulumi:Stack::resource1"},
				PropertyDependencies: map[resource.PropertyKey][]resource.URN{
					"prop1": {"urn:pulumi:test::stack::pulumi:pulumi:Stack::resource1"},
				},
				ResourceHooks:  map[resource.HookType][]string{resource.BeforeCreate: {"hook1"}},
				SourcePosition: "pos1",
			},
			new: &pkgresource.State{
				URN:          defaultURN,
				Type:         "pulumi:providers:aws",
				Inputs:       property.NewMap(map[string]property.Value{"key": property.New("value")}),
				Outputs:      property.NewMap(map[string]property.Value{"key": property.New("value")}),
				Dependencies: []resource.URN{"urn:pulumi:test::stack::pulumi:pulumi:Stack::resource1"},
				PropertyDependencies: map[resource.PropertyKey][]resource.URN{
					"prop1": {"urn:pulumi:test::stack::pulumi:pulumi:Stack::resource1"},
				},
				ResourceHooks:  map[resource.HookType][]string{resource.BeforeCreate: {"hook1"}},
				SourcePosition: "pos1",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ssm := sameSnapshotMutation{}
			step := deploy.NewSameStep(nil, nil, tc.old, tc.new)
			result := ssm.mustWrite(step)
			require.Equal(t, tc.mustWrite, result)
		})
	}
}
