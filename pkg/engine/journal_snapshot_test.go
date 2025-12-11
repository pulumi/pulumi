// Copyright 2025, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package engine

import (
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/require"
)

func TestMustWrite(t *testing.T) {
	defaultURN := resource.URN("urn:pulumi:test::stack::pulumi:providers:aws::default_4_42_0")

	cases := []struct {
		name      string
		old       *resource.State
		new       *resource.State
		mustWrite bool
	}{
		{
			name:      "changed URN",
			old:       &resource.State{URN: defaultURN},
			new:       &resource.State{URN: "urn:pulumi:test::stack::pulumi:providers:aws::default_4_42_1"},
			mustWrite: true,
		},
		{
			name:      "type changed",
			old:       &resource.State{URN: defaultURN, Type: "pulumi:providers:aws"},
			new:       &resource.State{URN: defaultURN, Type: "pulumi:providers:awsx"},
			mustWrite: true,
		},
		{
			name:      "custom changed",
			old:       &resource.State{URN: defaultURN, Type: "pulumi:providers:aws", ID: "a", Custom: true},
			new:       &resource.State{URN: defaultURN, Type: "pulumi:providers:aws", ID: "", Custom: false},
			mustWrite: true,
		},
		{
			name:      "custom timeout changed",
			old:       &resource.State{URN: defaultURN, CustomTimeouts: resource.CustomTimeouts{Create: 600}},
			new:       &resource.State{URN: defaultURN, CustomTimeouts: resource.CustomTimeouts{Create: 900}},
			mustWrite: true,
		},
		{
			name:      "RetainOnDelete changed",
			old:       &resource.State{URN: defaultURN, RetainOnDelete: false},
			new:       &resource.State{URN: defaultURN, RetainOnDelete: true},
			mustWrite: true,
		},
		{
			name:      "Provider changed",
			old:       &resource.State{URN: defaultURN, Provider: "urn:pulumi:test::stack::pulumi:providers:aws::default_4_42_0::provider-id-1"},
			new:       &resource.State{URN: defaultURN, Provider: "urn:pulumi:test::stack::pulumi:providers:aws::default_4_42_0::provider-id-2"},
			mustWrite: true,
		},
		{
			name:      "Parent changed",
			old:       &resource.State{URN: defaultURN, Parent: "urn:pulumi:test::stack::pulumi:pulumi:Stack::parent1"},
			new:       &resource.State{URN: defaultURN, Parent: "urn:pulumi:test::stack::pulumi:pulumi:Stack::parent2"},
			mustWrite: true,
		},
		{
			name:      "DeletedWith changed",
			old:       &resource.State{URN: defaultURN, DeletedWith: "urn:pulumi:test::stack::pulumi:pulumi:Stack::resource1"},
			new:       &resource.State{URN: defaultURN, DeletedWith: "urn:pulumi:test::stack::pulumi:pulumi:Stack::resource2"},
			mustWrite: true,
		},
		{
			name:      "ReplaceWith changed - different length",
			old:       &resource.State{URN: defaultURN, ReplaceWith: []resource.URN{"urn:pulumi:test::stack::pulumi:pulumi:Stack::resource1"}},
			new:       &resource.State{URN: defaultURN, ReplaceWith: []resource.URN{}},
			mustWrite: true,
		},
		{
			name: "ReplaceWith changed - different content",
			old: &resource.State{URN: defaultURN, ReplaceWith: []resource.URN{
				"urn:pulumi:test::stack::pulumi:pulumi:Stack::resource1",
			}},
			new: &resource.State{URN: defaultURN, ReplaceWith: []resource.URN{
				"urn:pulumi:test::stack::pulumi:pulumi:Stack::resource2",
			}},
			mustWrite: true,
		},
		{
			name:      "Protect changed",
			old:       &resource.State{URN: defaultURN, Protect: false},
			new:       &resource.State{URN: defaultURN, Protect: true},
			mustWrite: true,
		},
		{
			name:      "Inputs changed",
			old:       &resource.State{URN: defaultURN, Inputs: resource.PropertyMap{"key": resource.NewStringProperty("value1")}},
			new:       &resource.State{URN: defaultURN, Inputs: resource.PropertyMap{"key": resource.NewStringProperty("value2")}},
			mustWrite: true,
		},
		{
			name:      "Outputs changed",
			old:       &resource.State{URN: defaultURN, Outputs: resource.PropertyMap{"key": resource.NewStringProperty("value1")}},
			new:       &resource.State{URN: defaultURN, Outputs: resource.PropertyMap{"key": resource.NewStringProperty("value2")}},
			mustWrite: true,
		},
		{
			name: "Dependencies changed - added dependency",
			old:  &resource.State{URN: defaultURN, Dependencies: []resource.URN{}},
			new: &resource.State{URN: defaultURN, Dependencies: []resource.URN{
				"urn:pulumi:test::stack::pulumi:pulumi:Stack::resource1",
			}},
			mustWrite: true,
		},
		{
			name: "Dependencies changed - removed dependency",
			old: &resource.State{URN: defaultURN, Dependencies: []resource.URN{
				"urn:pulumi:test::stack::pulumi:pulumi:Stack::resource1",
			}},
			new:       &resource.State{URN: defaultURN, Dependencies: []resource.URN{}},
			mustWrite: true,
		},
		{
			name: "Dependencies changed - different order (should not trigger write)",
			old: &resource.State{URN: defaultURN, Dependencies: []resource.URN{
				"urn:pulumi:test::stack::pulumi:pulumi:Stack::resource1",
				"urn:pulumi:test::stack::pulumi:pulumi:Stack::resource2",
			}},
			new: &resource.State{URN: defaultURN, Dependencies: []resource.URN{
				"urn:pulumi:test::stack::pulumi:pulumi:Stack::resource2",
				"urn:pulumi:test::stack::pulumi:pulumi:Stack::resource1",
			}},
			mustWrite: false,
		},
		{
			name: "PropertyDependencies changed - added property",
			old:  &resource.State{URN: defaultURN, PropertyDependencies: map[resource.PropertyKey][]resource.URN{}},
			new: &resource.State{URN: defaultURN, PropertyDependencies: map[resource.PropertyKey][]resource.URN{
				"prop1": {"urn:pulumi:test::stack::pulumi:pulumi:Stack::resource1"},
			}},
			mustWrite: true,
		},
		{
			name: "PropertyDependencies changed - removed property",
			old: &resource.State{URN: defaultURN, PropertyDependencies: map[resource.PropertyKey][]resource.URN{
				"prop1": {"urn:pulumi:test::stack::pulumi:pulumi:Stack::resource1"},
			}},
			new:       &resource.State{URN: defaultURN, PropertyDependencies: map[resource.PropertyKey][]resource.URN{}},
			mustWrite: true,
		},
		{
			name: "PropertyDependencies changed - different dependencies for same property",
			old: &resource.State{URN: defaultURN, PropertyDependencies: map[resource.PropertyKey][]resource.URN{
				"prop1": {"urn:pulumi:test::stack::pulumi:pulumi:Stack::resource1"},
				"prop2": {"urn:pulumi:test::stack::pulumi:pulumi:Stack::resource1"},
			}},
			new: &resource.State{URN: defaultURN, PropertyDependencies: map[resource.PropertyKey][]resource.URN{
				"prop1": {"urn:pulumi:test::stack::pulumi:pulumi:Stack::resource2"},
				"prop2": {"urn:pulumi:test::stack::pulumi:pulumi:Stack::resource1"},
			}},
			mustWrite: true,
		},
		{
			name: "ResourceHooks changed",
			old: &resource.State{URN: defaultURN, ResourceHooks: map[resource.HookType][]string{
				resource.BeforeCreate: {"hook1"},
			}},
			new: &resource.State{URN: defaultURN, ResourceHooks: map[resource.HookType][]string{
				resource.BeforeCreate: {"hook2"},
			}},
			mustWrite: true,
		},
		{
			name:      "SourcePosition changed - should not write",
			old:       &resource.State{URN: defaultURN, SourcePosition: "pos1"},
			new:       &resource.State{URN: defaultURN, SourcePosition: "pos2"},
			mustWrite: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ssm := sameSnapshotMutation{}
			step := deploy.NewSameStep(nil, nil, tc.old, tc.new)
			result := ssm.mustWrite(step)
			require.Equal(t, tc.mustWrite, result)
		})
	}
}
