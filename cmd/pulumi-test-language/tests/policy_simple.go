// Copyright 2025, Pulumi Corporation.
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
	LanguageTests["policy-simple"] = LanguageTest{
		Providers: []plugin.Provider{&providers.SimpleProvider{}},
		Runs: []TestRun{
			{
				PolicyPacks: map[string]map[string]any{
					"simple": nil,
				},
				AssertPreview: func(
					l *L, projectDirectory string, err error, plan *deploy.Plan,
					changes display.ResourceChanges, events []engine.Event,
				) {
					require.ErrorContains(l, err, "BAIL: step generator errored")

					var policyViolations []engine.PolicyViolationEventPayload
					for _, event := range events {
						if event.Type == engine.PolicyViolationEvent {
							policyViolations = append(policyViolations, event.Payload().(engine.PolicyViolationEventPayload))
						}
					}

					expectedViolations := []engine.PolicyViolationEventPayload{
						{
							ResourceURN:       "urn:pulumi:test::policy-simple::simple:index:Resource::res1",
							Message:           "<{%reset%}>Verifies properties are true\nThis is a test warning<{%reset%}>\n",
							Color:             "raw",
							PolicyName:        "truthiness",
							PolicyPackName:    "simple",
							PolicyPackVersion: "1.0.0",
							EnforcementLevel:  "advisory",
							Prefix:            "<{%fg 3%}>advisory: <{%reset%}>",
						},
						{
							ResourceURN:       "urn:pulumi:test::policy-simple::simple:index:Resource::res2",
							Message:           "<{%reset%}>Verifies properties are false\nThis is a test error<{%reset%}>\n",
							Color:             "raw",
							PolicyName:        "falsiness",
							PolicyPackName:    "simple",
							PolicyPackVersion: "1.0.0",
							EnforcementLevel:  "mandatory",
							Prefix:            "<{%fg 1%}>mandatory: <{%reset%}>",
						},
					}

					assert.Len(l, policyViolations, len(expectedViolations), "expected %d policy violations", len(expectedViolations))

					for _, violation := range expectedViolations {
						assert.Contains(l, policyViolations, violation, "expected policy violation %v", violation)
					}
				},
				Assert: func(l *L,
					projectDirectory string, err error, snap *deploy.Snapshot,
					changes display.ResourceChanges, events []engine.Event,
				) {
					require.ErrorContains(l, err,
						"BAIL: resource urn:pulumi:test::policy-simple::simple:index:Resource::res2 is invalid")

					var policyViolations []engine.PolicyViolationEventPayload
					for _, event := range events {
						if event.Type == engine.PolicyViolationEvent {
							policyViolations = append(policyViolations, event.Payload().(engine.PolicyViolationEventPayload))
						}
					}

					expectedViolations := []engine.PolicyViolationEventPayload{
						{
							ResourceURN:       "urn:pulumi:test::policy-simple::simple:index:Resource::res1",
							Message:           "<{%reset%}>Verifies properties are true\nThis is a test warning<{%reset%}>\n",
							Color:             "raw",
							PolicyName:        "truthiness",
							PolicyPackName:    "simple",
							PolicyPackVersion: "1.0.0",
							EnforcementLevel:  "advisory",
							Prefix:            "<{%fg 3%}>advisory: <{%reset%}>",
						},
						{
							ResourceURN:       "urn:pulumi:test::policy-simple::simple:index:Resource::res2",
							Message:           "<{%reset%}>Verifies properties are false\nThis is a test error<{%reset%}>\n",
							Color:             "raw",
							PolicyName:        "falsiness",
							PolicyPackName:    "simple",
							PolicyPackVersion: "1.0.0",
							EnforcementLevel:  "mandatory",
							Prefix:            "<{%fg 1%}>mandatory: <{%reset%}>",
						},
					}

					assert.Len(l, policyViolations, len(expectedViolations), "expected %d policy violations", len(expectedViolations))

					for _, violation := range expectedViolations {
						assert.Contains(l, policyViolations, violation, "expected policy violation %v", violation)
					}
				},
			},
		},
	}
}
