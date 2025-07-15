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
	"github.com/pulumi/pulumi/cmd/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	validate := func(l *L, events []engine.Event, value string) {
		var policyViolations []engine.PolicyViolationEventPayload
		for _, event := range events {
			if event.Type == engine.PolicyViolationEvent {
				policyViolations = append(policyViolations, event.Payload().(engine.PolicyViolationEventPayload))
			}
		}

		expectedViolations := []engine.PolicyViolationEventPayload{
			{
				ResourceURN:       "urn:pulumi:test::policy-config::simple:index:Resource::res",
				Message:           "<{%reset%}>Verifies properties\nProperty was " + value + "<{%reset%}>\n",
				Color:             "raw",
				PolicyName:        "allowed",
				PolicyPackName:    "config",
				PolicyPackVersion: "2.0.0",
				EnforcementLevel:  "mandatory",
				Prefix:            "<{%fg 1%}>mandatory: <{%reset%}>",
			},
		}

		assert.Len(l, policyViolations, len(expectedViolations), "expected %d policy violations", len(expectedViolations))

		for _, violation := range expectedViolations {
			assert.Contains(l, policyViolations, violation, "expected policy violation %v", violation)
		}
	}

	LanguageTests["policy-config"] = LanguageTest{
		Providers: []plugin.Provider{&providers.SimpleProvider{}},
		Runs: []TestRun{
			{
				PolicyPacks: map[string]map[string]any{
					"config": {
						"allowed": map[string]any{
							"value": false,
						},
					},
				},
				AssertPreview: func(
					l *L, projectDirectory string, err error, plan *deploy.Plan,
					changes display.ResourceChanges, events []engine.Event,
				) {
					require.ErrorContains(l, err, "BAIL: step generator errored")
					validate(l, events, "true")
				},
				Assert: func(l *L,
					projectDirectory string, err error, snap *deploy.Snapshot,
					changes display.ResourceChanges, events []engine.Event,
				) {
					require.ErrorContains(l, err,
						"BAIL: resource urn:pulumi:test::policy-config::simple:index:Resource::res is invalid")
					validate(l, events, "true")
				},
			},
			{
				PolicyPacks: map[string]map[string]any{
					"config": {
						"allowed": map[string]any{
							"value": true,
						},
					},
				},
				AssertPreview: func(
					l *L, projectDirectory string, err error, plan *deploy.Plan,
					changes display.ResourceChanges, events []engine.Event,
				) {
					require.ErrorContains(l, err, "BAIL: step generator errored")
					validate(l, events, "false")
				},
				Assert: func(l *L,
					projectDirectory string, err error, snap *deploy.Snapshot,
					changes display.ResourceChanges, events []engine.Event,
				) {
					require.ErrorContains(l, err,
						"BAIL: resource urn:pulumi:test::policy-config::simple:index:Resource::res is invalid")
					validate(l, events, "false")
				},
			},
		},
	}
}
