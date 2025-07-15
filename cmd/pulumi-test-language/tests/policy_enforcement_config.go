// Copyright 2024, Pulumi Corporation.
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
	"fmt"

	"github.com/pulumi/pulumi/cmd/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	validate := func(l *L, events []engine.Event, level apitype.EnforcementLevel) {
		var policyViolations []engine.PolicyViolationEventPayload
		for _, event := range events {
			if event.Type == engine.PolicyViolationEvent {
				policyViolations = append(policyViolations, event.Payload().(engine.PolicyViolationEventPayload))
			}
		}

		expectedViolations := []engine.PolicyViolationEventPayload{}
		if level != "" {
			levelIndex := 3
			if level == apitype.Mandatory {
				levelIndex = 1
			}

			expectedViolations = append(expectedViolations, engine.PolicyViolationEventPayload{
				ResourceURN:       "urn:pulumi:test::policy-enforcement-config::simple:index:Resource::res",
				Message:           "<{%reset%}>Verifies property is false\nProperty was true<{%reset%}>\n",
				Color:             "raw",
				PolicyName:        "false",
				PolicyPackName:    "enforcement-config",
				PolicyPackVersion: "3.0.0",
				EnforcementLevel:  level,
				Prefix:            fmt.Sprintf("<{%%fg %d%%}>%s: <{%%reset%%}>", levelIndex, level),
			})
		}

		assert.Len(l, policyViolations, len(expectedViolations), "expected %d policy violations", len(expectedViolations))

		for _, violation := range expectedViolations {
			assert.Contains(l, policyViolations, violation, "expected policy violation %v", violation)
		}
	}

	// This is to test that we can configure policy enforcement via policy config with the special key
	// "enforcementLevel".
	LanguageTests["policy-enforcement-config"] = LanguageTest{
		Providers: []plugin.Provider{&providers.SimpleProvider{}},
		// All these runs share the same source, we're just changing the policy config.
		RunsShareSource: true,
		Runs: []TestRun{
			{
				// First run don't send any config, the policy should be advisory
				PolicyPacks: map[string]map[string]any{
					"enforcement-config": {},
				},
				AssertPreview: func(
					l *L, projectDirectory string, err error, plan *deploy.Plan,
					changes display.ResourceChanges, events []engine.Event,
				) {
					require.NoError(l, err)
					validate(l, events, apitype.Advisory)
				},
				Assert: func(l *L,
					projectDirectory string, err error, snap *deploy.Snapshot,
					changes display.ResourceChanges, events []engine.Event,
				) {
					require.NoError(l, err)
					validate(l, events, apitype.Advisory)
				},
			},
			{
				// Second, use config to set the policy to mandatory
				PolicyPacks: map[string]map[string]any{
					"enforcement-config": {
						"false": map[string]any{
							"enforcementLevel": "mandatory",
						},
					},
				},
				AssertPreview: func(
					l *L, projectDirectory string, err error, plan *deploy.Plan,
					changes display.ResourceChanges, events []engine.Event,
				) {
					require.ErrorContains(l, err, "BAIL: step generator errored")
					validate(l, events, apitype.Mandatory)
				},
				Assert: func(l *L,
					projectDirectory string, err error, snap *deploy.Snapshot,
					changes display.ResourceChanges, events []engine.Event,
				) {
					require.ErrorContains(l, err,
						"BAIL: resource urn:pulumi:test::policy-enforcement-config::simple:index:Resource::res is invalid")
					validate(l, events, apitype.Mandatory)
				},
			},
			{
				// Third, use config to disable the policy
				PolicyPacks: map[string]map[string]any{
					"enforcement-config": {
						"false": "disabled",
					},
				},
				AssertPreview: func(
					l *L, projectDirectory string, err error, plan *deploy.Plan,
					changes display.ResourceChanges, events []engine.Event,
				) {
					require.NoError(l, err)
					validate(l, events, "")
				},
				Assert: func(l *L,
					projectDirectory string, err error, snap *deploy.Snapshot,
					changes display.ResourceChanges, events []engine.Event,
				) {
					require.NoError(l, err)
					validate(l, events, "")
				},
			},
		},
	}
}
