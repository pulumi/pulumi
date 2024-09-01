// Copyright 2016-2018, Pulumi Corporation.
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

package plugin

import (
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/stretchr/testify/require"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/urn"
	"github.com/stretchr/testify/assert"
)

func TestRevertAlias(t *testing.T) {
	// Example URN string
	urnStr := "urn:pulumi:stack::project::type::name"

	// Revert the URN back into an Alias
	alias, err := revertAlias(urnStr)

	// Assertions
	assert.NoError(t, err)
	assert.Equal(t, urn.URN(urnStr), alias.URN)
}

func TestRevertAliases(t *testing.T) {
	// Example URN strings
	urnStrs := []string{
		"urn:pulumi:stack::project::type::name",
		"urn:pulumi:stack::project::type::name2",
	}

	// Revert the URNs back into Aliases
	aliases, _, err := revertAliases(urnStrs)

	// Assertions
	assert.NoError(t, err)
	assert.Len(t, aliases, len(urnStrs))
	for i, alias := range aliases {
		assert.Equal(t, urn.URN(urnStrs[i]), alias.URN)
	}
}

func TestRevertInvalidAlias(t *testing.T) {
	// Test with an invalid URN string
	invalidAliasStr := "invalid:urn:string"
	_, err := revertAlias(invalidAliasStr)

	// Assertion
	assert.Error(t, err)
}

func TestToProtoAnalyzerInfo(t *testing.T) {

	analyzerInfo := AnalyzerInfo{
		Name:           "validate-stack-test-policy",
		DisplayName:    "stack validation test policyPack",
		Version:        "0.0.1",
		SupportsConfig: true,
		Policies: []AnalyzerPolicyInfo{
			{
				Name:             "dynamic-no-foo-with-value-bar",
				Description:      "Prohibits setting foo to 'bar' on dynamic resources.",
				EnforcementLevel: "remediate",
				ConfigSchema: &AnalyzerPolicyConfigSchema{
					Properties: map[string]JSONSchema{
						"enforcementLevel": map[string]interface{}{
							"enum": []string{
								"advisory",
								"mandatory",
								"remediate",
								"disabled",
							},
							"type": "string",
						},
					},
					Required: nil,
				},
			},
			{
				Name:             "dynamic-no-state-with-value-1",
				Description:      "Prohibits setting state to 1 on dynamic resources.",
				EnforcementLevel: "mandatory",
				ConfigSchema: &AnalyzerPolicyConfigSchema{
					Properties: map[string]JSONSchema{
						"enforcementLevel": map[string]interface{}{
							"enum": []string{
								"advisory",
								"mandatory",
								"remediate",
								"disabled",
							},
							"type": "string",
						},
					},
					Required: nil,
				},
			},
		},
		InitialConfig: map[string]AnalyzerPolicyConfig{
			"policy1": {
				Properties:       map[string]interface{}{},
				EnforcementLevel: apitype.Advisory,
			},
		},
	}

	t.Run("successful conversion", func(t *testing.T) {

		result, err := ToProtoAnalyzerInfo(&analyzerInfo)

		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, analyzerInfo.Name, result.GetName())
	})
}
