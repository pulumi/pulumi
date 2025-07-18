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
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	getPolicyRemediations := func(l *L, events []engine.Event) []engine.PolicyRemediationEventPayload {
		var policyViolations []engine.PolicyRemediationEventPayload
		for _, event := range events {
			if event.Type == engine.PolicyRemediationEvent {
				policyViolations = append(policyViolations, event.Payload().(engine.PolicyRemediationEventPayload))
			}
		}
		return policyViolations
	}

	verify := func(l *L, events []engine.Event) {
		policyRemediations := getPolicyRemediations(l, events)

		expectedRemediations := []engine.PolicyRemediationEventPayload{
			{
				ResourceURN:       "urn:pulumi:test::policy-remediate::simple:index:Resource::res",
				Color:             "raw",
				PolicyName:        "fixup",
				PolicyPackName:    "remediate",
				PolicyPackVersion: "3.0.0",
				Before: resource.PropertyMap{
					"value": resource.NewPropertyValue(true),
				},
				After: resource.PropertyMap{
					"value": resource.NewPropertyValue(false),
				},
			},
		}

		require.Len(l, policyRemediations, len(expectedRemediations),
			"expected %d policy remediations", len(expectedRemediations))

		for _, remediations := range expectedRemediations {
			assert.Contains(l, policyRemediations, remediations, "expected policy remediation %v", remediations)
		}
	}

	// This is to test that we can run a remediation policy, we run a program that makes a simple resource,
	// and then using policy config we remediate it in various ways.
	LanguageTests["policy-remediate"] = LanguageTest{
		Providers: []plugin.Provider{&providers.SimpleProvider{}},
		// All these runs share the same source, we're just changing the policy config.
		RunsShareSource: true,
		Runs: []TestRun{
			{
				// First run change the value to false, this should trigger a remediation.
				PolicyPacks: map[string]map[string]any{
					"remediate": {
						"fixup": map[string]any{
							"value": false,
						},
					},
				},
				AssertPreview: func(
					l *L, projectDirectory string, err error, plan *deploy.Plan,
					changes display.ResourceChanges, events []engine.Event,
				) {
					require.NoError(l, err)
					verify(l, events)
				},
				Assert: func(l *L,
					projectDirectory string, err error, snap *deploy.Snapshot,
					changes display.ResourceChanges, events []engine.Event,
				) {
					require.NoError(l, err)
					verify(l, events)
				},
			},
			{
				// Next run change the value to true, this should not trigger a remediation because the value
				// is already true.
				PolicyPacks: map[string]map[string]any{
					"remediate": {
						"fixup": map[string]any{
							"value": true,
						},
					},
				},
				AssertPreview: func(
					l *L, projectDirectory string, err error, plan *deploy.Plan,
					changes display.ResourceChanges, events []engine.Event,
				) {
					require.NoError(l, err)
					policyRemediations := getPolicyRemediations(l, events)
					assert.Empty(l, policyRemediations, "expected no policy remediations")
				},
				Assert: func(l *L,
					projectDirectory string, err error, snap *deploy.Snapshot,
					changes display.ResourceChanges, events []engine.Event,
				) {
					require.NoError(l, err)
					policyRemediations := getPolicyRemediations(l, events)
					assert.Empty(l, policyRemediations, "expected no policy remediations")
				},
			},
			{
				// Next run change the value to false, but disable the remediation policy so it doesn't
				// actually run.
				PolicyPacks: map[string]map[string]any{
					"remediate": {
						"fixup": map[string]any{
							"value":            true,
							"enforcementLevel": "disabled",
						},
					},
				},
				AssertPreview: func(
					l *L, projectDirectory string, err error, plan *deploy.Plan,
					changes display.ResourceChanges, events []engine.Event,
				) {
					require.NoError(l, err)
					policyRemediations := getPolicyRemediations(l, events)
					assert.Empty(l, policyRemediations, "expected no policy remediations")
				},
				Assert: func(l *L,
					projectDirectory string, err error, snap *deploy.Snapshot,
					changes display.ResourceChanges, events []engine.Event,
				) {
					require.NoError(l, err)
					policyRemediations := getPolicyRemediations(l, events)
					assert.Empty(l, policyRemediations, "expected no policy remediations")
				},
			},
		},
	}
}
