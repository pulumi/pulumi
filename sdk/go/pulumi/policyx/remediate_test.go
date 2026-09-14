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

package policyx

import (
	"context"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/pulumi/pulumi/sdk/v3/go/property"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

func TestRemediateRunsOnlyAtRemediateLevel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		config          map[string]PolicyConfig
		wantRemediation bool
	}{
		{
			name:            "no config uses the policy level",
			wantRemediation: true,
		},
		{
			name:            "remediate config",
			config:          map[string]PolicyConfig{"fixup": {EnforcementLevel: EnforcementLevelRemediate}},
			wantRemediation: true,
		},
		{
			name:   "advisory config",
			config: map[string]PolicyConfig{"fixup": {EnforcementLevel: EnforcementLevelAdvisory}},
		},
		{
			name:   "mandatory config",
			config: map[string]PolicyConfig{"fixup": {EnforcementLevel: EnforcementLevelMandatory}},
		},
		{
			name:   "disabled config",
			config: map[string]PolicyConfig{"fixup": {EnforcementLevel: EnforcementLevelDisabled}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			policyPack, err := NewPolicyPack("remediate", semver.MustParse("1.0.0"), EnforcementLevelAdvisory, []Policy{
				NewResourceRemediationPolicy("fixup", ResourceRemediationPolicyArgs{
					Description: "Sets value to false",
					RemediateResource: func(context.Context, ResourceRemediationArgs) (*property.Map, error) {
						result := property.NewMap(map[string]property.Value{"value": property.New(false)})
						return &result, nil
					},
				}),
			})
			require.NoError(t, err)
			srv := &analyzerServer{policyPack: policyPack, config: tt.config}
			props, err := structpb.NewStruct(map[string]any{"value": true})
			require.NoError(t, err)

			resp, err := srv.Remediate(t.Context(), &pulumirpc.AnalyzeRequest{
				Type:       "simple:index:Resource",
				Urn:        "urn:pulumi:test::test::simple:index:Resource::res",
				Name:       "res",
				Properties: props,
			})
			require.NoError(t, err)

			if !tt.wantRemediation {
				assert.Empty(t, resp.GetRemediations())
				return
			}
			require.Len(t, resp.GetRemediations(), 1)
			assert.Equal(t, "fixup", resp.GetRemediations()[0].GetPolicyName())
			assert.Equal(t, map[string]any{"value": false}, resp.GetRemediations()[0].GetProperties().AsMap())
		})
	}
}
