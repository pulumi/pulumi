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
	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
)

//nolint:paralleltest // t.Setenv
func TestPolicyRemove_JSON(t *testing.T) {
	t.Setenv("PULUMI_SKIP_CONFIRMATIONS", "")

	tests := []struct {
		name    string
		version string
		wantTag any
	}{
		{name: "all versions", version: "all", wantTag: nil},
		{name: "one version", version: "1.0.0", wantTag: "1.0.0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pack := &backend.MockPolicyPack{
				RefF: func() backend.PolicyPackReference { return stubPolicyPackRef{"acme", "p"} },
				RemoveF: func(_ context.Context, op backend.PolicyPackOperation) error {
					if tt.wantTag == nil {
						assert.Nil(t, op.VersionTag)
					} else {
						require.NotNil(t, op.VersionTag)
						assert.Equal(t, tt.wantTag, *op.VersionTag)
					}
					return nil
				},
			}
			out, err := runPolicyPackCmd(t,
				newPolicyRemoveCmd(newTestWsAndLm(newTestBackend(pack))),
				"acme/p", tt.version, "--yes", "--output", "json")
			require.NoError(t, err)

			var got map[string]any
			require.NoError(t, json.Unmarshal([]byte(out), &got))
			assert.Equal(t, "acme", got["organizationName"])
			assert.Equal(t, "p", got["name"])
			v, ok := got["versionTag"]
			assert.True(t, ok)
			assert.Equal(t, tt.wantTag, v)
		})
	}
}

//nolint:paralleltest // t.Setenv
func TestPolicyRemove_DefaultMessage(t *testing.T) {
	t.Setenv("PULUMI_SKIP_CONFIRMATIONS", "")

	tests := []struct {
		name    string
		version string
		want    string
	}{
		{name: "all versions", version: "all", want: "Removed policy pack acme/p.\n"},
		{name: "one version", version: "1.0.0", want: "Removed version 1.0.0 of policy pack acme/p.\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pack := &backend.MockPolicyPack{
				RefF:    func() backend.PolicyPackReference { return stubPolicyPackRef{"acme", "p"} },
				RemoveF: func(context.Context, backend.PolicyPackOperation) error { return nil },
			}
			out, err := runPolicyPackCmd(t,
				newPolicyRemoveCmd(newTestWsAndLm(newTestBackend(pack))), "acme/p", tt.version, "--yes")
			require.NoError(t, err)
			assert.Equal(t, tt.want, out)
		})
	}
}

func TestPolicyRemove_NonInteractiveWithoutYesErrors(t *testing.T) {
	t.Setenv("PULUMI_SKIP_CONFIRMATIONS", "")

	removed := false
	pack := &backend.MockPolicyPack{
		RefF: func() backend.PolicyPackReference { return stubPolicyPackRef{"acme", "p"} },
		RemoveF: func(context.Context, backend.PolicyPackOperation) error {
			removed = true
			return nil
		},
	}
	out, err := runPolicyPackCmdSplit(t,
		newPolicyRemoveCmd(newTestWsAndLm(newTestBackend(pack))),
		"acme/p", "all", "--output", "json")
	require.ErrorIs(t, err, backenderr.ErrNonInteractiveRequiresYes)
	assert.False(t, removed)
	assert.Empty(t, out)
}
