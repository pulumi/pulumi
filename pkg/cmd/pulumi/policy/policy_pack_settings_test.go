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
	"encoding/json"
	"testing"

	cloudapitype "github.com/pulumi/pulumi-cloud-sdk/go/apitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

func TestApplyConfigChanges(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		config  string
		changes packSettingChanges
		want    string
	}{
		{
			name:   "no changes keeps config",
			config: `{"s3":{"enforcementLevel":"advisory","limit":3}}`,
			want:   `{"s3":{"enforcementLevel":"advisory","limit":3}}`,
		},
		{
			name:    "pack level on empty config",
			config:  `null`,
			changes: packSettingChanges{packLevel: apitype.Mandatory},
			want:    `{"all":{"enforcementLevel":"mandatory"}}`,
		},
		{
			name:    "pack level replaces string all and clears overrides",
			config:  `{"all":"advisory","s3":"disabled","iam":{"enforcementLevel":"advisory","roles":["a"]},"ec2":{"x":1}}`,
			changes: packSettingChanges{packLevel: apitype.Mandatory},
			want:    `{"all":{"enforcementLevel":"mandatory"},"iam":{"roles":["a"]},"ec2":{"x":1}}`,
		},
		{
			name:   "policy levels apply after pack level",
			config: `{"s3":{"enforcementLevel":"disabled","limit":3}}`,
			changes: packSettingChanges{
				packLevel:    apitype.Advisory,
				policyLevels: []policyLevel{{"s3", apitype.Mandatory}, {"iam", apitype.Remediate}},
			},
			want: `{"all":{"enforcementLevel":"advisory"},"s3":{"enforcementLevel":"mandatory","limit":3},` +
				`"iam":{"enforcementLevel":"remediate"}}`,
		},
		{
			name:    "policy level overrides string value",
			config:  `{"s3":"disabled","iam":{"roles":["a"]}}`,
			changes: packSettingChanges{policyLevels: []policyLevel{{"s3", apitype.Mandatory}}},
			want:    `{"s3":{"enforcementLevel":"mandatory"},"iam":{"roles":["a"]}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var config map[string]*json.RawMessage
			require.NoError(t, json.Unmarshal([]byte(tt.config), &config))
			before, err := json.Marshal(config)
			require.NoError(t, err)

			got, err := applyConfigChanges(config, tt.changes)
			require.NoError(t, err)
			gotJSON, err := json.Marshal(got)
			require.NoError(t, err)
			assert.JSONEq(t, tt.want, string(gotJSON))

			after, err := json.Marshal(config)
			require.NoError(t, err)
			assert.Equal(t, string(before), string(after), "input must not be modified")
		})
	}
}

func TestApplyConfigChanges_RejectsMalformedValue(t *testing.T) {
	t.Parallel()

	config := map[string]*json.RawMessage{"s3": new(json.RawMessage(`[1]`))}
	_, err := applyConfigChanges(config, packSettingChanges{packLevel: apitype.Mandatory})
	assert.EqualError(t, err, `reading configuration for policy "s3": expected an object or string, got [1]`)
}

func TestApplyEnvironmentChanges(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		envs    []string
		changes packSettingChanges
		want    []string
	}{
		{"nothing", nil, packSettingChanges{}, nil},
		{"add to empty", nil, packSettingChanges{addEnvironments: []string{"a/b"}}, []string{"a/b"}},
		{
			"add existing is no-op",
			[]string{"a/b@1"},
			packSettingChanges{addEnvironments: []string{"a/b@1"}},
			[]string{"a/b@1"},
		},
		{
			"add other version replaces in place",
			[]string{"a/b@1", "c/d"},
			packSettingChanges{addEnvironments: []string{"a/b@2"}},
			[]string{"a/b@2", "c/d"},
		},
		{
			"remove missing is no-op",
			[]string{"a/b"},
			packSettingChanges{removeEnvironments: []string{"x/y"}},
			[]string{"a/b"},
		},
		{
			"remove without version removes any version",
			[]string{"a/b@1", "c/d"},
			packSettingChanges{removeEnvironments: []string{"a/b"}},
			[]string{"c/d"},
		},
		{
			"remove with version matches exactly",
			[]string{"a/b@1"},
			packSettingChanges{removeEnvironments: []string{"a/b@2"}},
			[]string{"a/b@1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			before := append([]string(nil), tt.envs...)
			assert.Equal(t, tt.want, applyEnvironmentChanges(tt.envs, tt.changes))
			assert.Equal(t, before, tt.envs)
		})
	}
}

func TestDropUnknownPolicyConfig(t *testing.T) {
	t.Parallel()

	config := map[string]*json.RawMessage{
		"all":  new(json.RawMessage(`{"enforcementLevel":"mandatory"}`)),
		"kept": new(json.RawMessage(`{}`)),
		"zeta": new(json.RawMessage(`{}`)),
		"beta": new(json.RawMessage(`{}`)),
	}
	got, dropped := dropUnknownPolicyConfig(config, []cloudapitype.AppPolicy{{Name: "kept"}, {Name: "new"}})
	assert.Equal(t, []string{"beta", "zeta"}, dropped)
	assert.ElementsMatch(t, []string{"all", "kept"}, keysOf(got))
	require.Len(t, config, 4)
}

func keysOf(m map[string]*json.RawMessage) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func TestMergePackSettings_NotEnabledNoChangesMakesNoCalls(t *testing.T) {
	t.Parallel()

	c := &fakePolicyEnableClient{}
	got, err := mergePackSettings(t.Context(), c, "acme", "p", packMergeInput{})
	require.NoError(t, err)
	assert.Nil(t, got.config)
	assert.Nil(t, got.environments)
	assert.Nil(t, got.droppedConfigKeys)
	assert.Zero(t, c.listCalls)
	assert.Empty(t, c.packGets)
}

func TestPackVersionTag(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "1.2.0", packVersionTag(cloudapitype.AppPolicyPackMetadata{Version: 3, VersionTag: "1.2.0"}))
	assert.Equal(t, "3", packVersionTag(cloudapitype.AppPolicyPackMetadata{Version: 3}))
	assert.Equal(t, "", packVersionTag(cloudapitype.AppPolicyPackMetadata{}))
}
