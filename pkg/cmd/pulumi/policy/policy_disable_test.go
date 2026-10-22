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
	"bytes"
	"context"
	"encoding/json"
	"testing"

	cloudapitype "github.com/pulumi/pulumi-cloud-sdk/go/apitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
)

func TestPolicyDisable_JSONReadsBackGroup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		args      []string
		wantGroup string
		wantTag   string
	}{
		{name: "no version", args: []string{"acme/p", "--output", "json"}, wantGroup: "default-policy-group"},
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
				RefF: func() backend.PolicyPackReference { return stubPolicyPackRef{"acme", "p"} },
				DisableF: func(_ context.Context, group string, op backend.PolicyPackOperation) error {
					assert.Equal(t, tt.wantTag, *op.VersionTag)
					return nil
				},
			}
			client := stubPolicyGroupClient{get: func(_ context.Context, org, group string) (cloudapitype.PolicyGroup, error) {
				assert.Equal(t, "acme", org)
				assert.Equal(t, tt.wantGroup, group)
				return cloudapitype.PolicyGroup{Name: group}, nil
			}}
			out, err := runPolicyPackCmd(t, newPolicyDisableCmdWith(stubPolicyPackDeps(pack, client)), tt.args...)
			require.NoError(t, err)

			var got cloudapitype.PolicyGroup
			require.NoError(t, json.Unmarshal([]byte(out), &got))
			assert.Equal(t, tt.wantGroup, got.Name)
			assert.Empty(t, got.AppliedPolicyPacks)
		})
	}
}

func TestPolicyDisable_DefaultRendersGroup(t *testing.T) {
	t.Parallel()

	pack := &backend.MockPolicyPack{
		RefF:     func() backend.PolicyPackReference { return stubPolicyPackRef{"acme", "p"} },
		DisableF: func(context.Context, string, backend.PolicyPackOperation) error { return nil },
	}
	group := cloudapitype.PolicyGroup{Name: "default-policy-group"}
	client := stubPolicyGroupClient{get: func(context.Context, string, string) (cloudapitype.PolicyGroup, error) {
		return group, nil
	}}
	out, err := runPolicyPackCmd(t, newPolicyDisableCmdWith(stubPolicyPackDeps(pack, client)), "acme/p")
	require.NoError(t, err)

	var want bytes.Buffer
	require.NoError(t, renderPolicyGroupGetText(&want, group))
	assert.Equal(t, want.String(), out)
}
