// Copyright 2026, Pulumi Corporation.
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
	"encoding/json"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/require"
)

func TestRunStateMigrationCallbacksRejectsInvalidAccounting(t *testing.T) {
	t.Parallel()

	const (
		rootURN  = resource.URN("urn:pulumi:test::test::pkg:m:Component::component")
		childURN = resource.URN("urn:pulumi:test::test::pkg:m:Resource::child")
		newURN   = resource.URN("urn:pulumi:test::test::pkg:m:Resource::renamed")
		otherURN = resource.URN("urn:pulumi:test::test::pkg:m:Resource::other")
	)
	root := apitype.ResourceV3{URN: rootURN, Type: rootURN.Type()}
	child := apitype.ResourceV3{URN: childURN, Type: childURN.Type(), Custom: true, ID: "child-id"}
	original := []apitype.ResourceV3{root, child}

	tests := []struct {
		name       string
		returned   []apitype.ResourceV3
		successors map[resource.URN]resource.URN
		noState    bool
		want       string
	}{
		{
			name:       "successor source is not prior state",
			returned:   original,
			successors: map[resource.URN]resource.URN{otherURN: childURN},
			want:       "returned successor for resource " + string(otherURN),
		},
		{
			name:       "successor target is not returned",
			returned:   []apitype.ResourceV3{root},
			successors: map[resource.URN]resource.URN{childURN: newURN},
			want:       "is not present in the returned state",
		},
		{
			name:       "successors without state",
			noState:    true,
			successors: map[resource.URN]resource.URN{childURN: newURN},
			want:       "returned successors without returning a new state",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			callback := func(
				context.Context, resource.URN, []byte,
			) ([]byte, map[resource.URN]resource.URN, error) {
				if tt.noState {
					return nil, tt.successors, nil
				}
				state, err := json.Marshal(tt.returned)
				return state, tt.successors, err
			}
			_, err := runStateMigrationCallbacks(
				t.Context(), rootURN, []StateMigrationFunction{callback}, original)
			require.ErrorContains(t, err, tt.want)
		})
	}
}

func TestDecodeStateMigrationResourcesRejectsUnknownFields(t *testing.T) {
	t.Parallel()

	data := []byte(`[{"urn":"urn:pulumi:test::test::pkg:m:Component::component",` +
		`"type":"pkg:m:Component","protected":true}]`)
	_, err := decodeStateMigrationResources(data)
	require.ErrorContains(t, err, `unknown field "protected"`)
}
