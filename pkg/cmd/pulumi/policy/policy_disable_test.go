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

package policy

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
)

func TestPolicyDisable_JSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		args      []string
		wantGroup string
		wantTag   any
	}{
		{
			name: "no version", args: []string{"acme/p", "--output", "json"},
			wantGroup: "default-policy-group", wantTag: nil,
		},
		{
			name:      "version and group",
			args:      []string{"acme/p", "--version", "1.0.0", "--policy-group", "prod", "--output", "json"},
			wantGroup: "prod", wantTag: "1.0.0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			pack := &backend.MockPolicyPack{
				RefF:     func() backend.PolicyPackReference { return stubPolicyPackRef{"acme", "p"} },
				DisableF: func(context.Context, string, backend.PolicyPackOperation) error { return nil },
			}
			client := stubPolicyGroupClient{}
			out, err := runPolicyPackCmd(t, newPolicyDisableCmdWith(stubPolicyPackDeps(pack, client)), tt.args...)
			require.NoError(t, err)

			var got map[string]any
			require.NoError(t, json.Unmarshal([]byte(out), &got))
			assert.Equal(t, "acme", got["organization"])
			assert.Equal(t, tt.wantGroup, got["policyGroup"])
			assert.Equal(t, "p", got["policyPack"])
			v, ok := got["versionTag"]
			assert.True(t, ok)
			assert.Equal(t, tt.wantTag, v)
		})
	}
}

func TestPolicyDisable_DefaultMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "no version", args: []string{"acme/p"},
			want: "Disabled policy pack acme/p in policy group default-policy-group.\n",
		},
		{
			name: "version and group", args: []string{"acme/p", "--version", "1.0.0", "--policy-group", "prod"},
			want: "Disabled version 1.0.0 of policy pack acme/p in policy group prod.\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			pack := &backend.MockPolicyPack{
				RefF:     func() backend.PolicyPackReference { return stubPolicyPackRef{"acme", "p"} },
				DisableF: func(context.Context, string, backend.PolicyPackOperation) error { return nil },
			}
			out, err := runPolicyPackCmd(t,
				newPolicyDisableCmdWith(stubPolicyPackDeps(pack, stubPolicyGroupClient{})), tt.args...)
			require.NoError(t, err)
			assert.Equal(t, tt.want, out)
		})
	}
}
