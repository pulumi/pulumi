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

package tests

import (
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/pkg/v3/testing/pulumi-test-language/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	expectedPolicyNames := []string{
		"name-type-urn-props",
		"protect",
		"ignore-changes",
		"delete-before-replace",
		"additional-secret-outputs",
		"custom-timeouts",
		"provider",
		"parent",
		"dependencies",
		"property-dependencies",
	}

	collectViolationPolicyNames := func(events []engine.Event) []string {
		var policyNames []string
		for _, event := range events {
			if event.Type == engine.PolicyViolationEvent {
				payload := event.Payload().(engine.PolicyViolationEventPayload)
				policyNames = append(policyNames, payload.PolicyName)
			}
		}
		return policyNames
	}

	assertMetadataViolations := func(l *L, events []engine.Event) {
		policyNames := collectViolationPolicyNames(events)
		require.Len(l, policyNames, len(expectedPolicyNames), "expected one violation from each metadata policy")
		assert.ElementsMatch(l, expectedPolicyNames, policyNames)
	}

	LanguageTests["policy-resource-metadata"] = LanguageTest{
		RunsShareSource: true,
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.SimpleProvider{} },
		},
		Runs: []TestRun{
			{
				PolicyPacks: map[string]map[string]any{
					"resource-metadata": nil,
				},
				AssertPreview: func(l *L, res AssertPreviewArgs) {
					require.NoError(l, res.Err)
					assertMetadataViolations(l, res.Events)
				},
				Assert: func(l *L, res AssertArgs) {
					require.NoError(l, res.Err)
					assertMetadataViolations(l, res.Events)
				},
			},
		},
	}
}
