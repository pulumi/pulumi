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
	"github.com/stretchr/testify/require"
)

func init() {
	getDiagnostics := func(l *L, events []engine.Event) []engine.DiagEventPayload {
		var diagnostics []engine.DiagEventPayload
		for _, event := range events {
			if event.Type == engine.DiagEvent {
				diagnostics = append(diagnostics, event.Payload().(engine.DiagEventPayload))
			}
		}
		return diagnostics
	}
	getPolicyViolationEvents := func(l *L, events []engine.Event) []engine.PolicyViolationEventPayload {
		var policyViolations []engine.PolicyViolationEventPayload
		for _, event := range events {
			if event.Type == engine.PolicyViolationEvent {
				policyViolations = append(policyViolations, event.Payload().(engine.PolicyViolationEventPayload))
			}
		}
		return policyViolations
	}

	// This is to test that policy packs can return config schemas that the engine validates config against. The policy
	// pack should return a config schema that defines a required bool field "value", and a string list field "names" that
	// must contain at least one item.
	LanguageTests["policy-config-schema"] = LanguageTest{
		Providers: []plugin.Provider{&providers.SimpleProvider{}},
		// All these runs share the same source, we're just changing the policy config.
		RunsShareSource: true,
		Runs: []TestRun{
			{
				// First run don't send the names config, it should fail to validate.
				PolicyPacks: map[string]map[string]any{
					"config-schema": {
						"validator": map[string]any{
							"value": false,
						},
					},
				},
				AssertPreview: func(
					l *L, projectDirectory string, err error, plan *deploy.Plan,
					changes display.ResourceChanges, events []engine.Event,
				) {
					require.ErrorContains(l, err, "validating policy config")
					diags := getDiagnostics(l, events)
					require.Len(l, diags, 1)
					require.Equal(l,
						"<{%reset%}>validating policy config: config-schema 3.0.0  validator: names is required<{%reset%}>\n",
						diags[0].Message)
				},
				Assert: func(l *L,
					projectDirectory string, err error, snap *deploy.Snapshot,
					changes display.ResourceChanges, events []engine.Event,
				) {
					require.ErrorContains(l, err, "validating policy config")
					diags := getDiagnostics(l, events)
					require.Len(l, diags, 1)
					require.Equal(l,
						"<{%reset%}>validating policy config: config-schema 3.0.0  validator: names is required<{%reset%}>\n",
						diags[0].Message)
				},
			},
			{
				// Second run send the names config but it's empty, it should still fail to validate.
				PolicyPacks: map[string]map[string]any{
					"config-schema": {
						"validator": map[string]any{
							"value": false,
							"names": []string{},
						},
					},
				},
				AssertPreview: func(
					l *L, projectDirectory string, err error, plan *deploy.Plan,
					changes display.ResourceChanges, events []engine.Event,
				) {
					require.ErrorContains(l, err, "validating policy config")
					diags := getDiagnostics(l, events)
					require.Len(l, diags, 1)
					require.Equal(l,
						"<{%reset%}>validating policy config: config-schema 3.0.0  validator:"+
							" names: Array must have at least 1 items<{%reset%}>\n",
						diags[0].Message)
				},
				Assert: func(l *L,
					projectDirectory string, err error, snap *deploy.Snapshot,
					changes display.ResourceChanges, events []engine.Event,
				) {
					require.ErrorContains(l, err, "validating policy config")
					diags := getDiagnostics(l, events)
					require.Len(l, diags, 1)
					require.Equal(l,
						"<{%reset%}>validating policy config: config-schema 3.0.0  validator:"+
							" names: Array must have at least 1 items<{%reset%}>\n",
						diags[0].Message)
				},
			},
			{
				// Run with a valid config, this should pass validation.
				PolicyPacks: map[string]map[string]any{
					"config-schema": {
						"validator": map[string]any{
							"value": false,
							"names": []string{"resN"},
						},
					},
				},
				AssertPreview: func(
					l *L, projectDirectory string, err error, plan *deploy.Plan,
					changes display.ResourceChanges, events []engine.Event,
				) {
					require.NoError(l, err)
					policyViolations := getPolicyViolationEvents(l, events)
					require.Empty(l, policyViolations, "expected no policy violations")
				},
				Assert: func(l *L,
					projectDirectory string, err error, snap *deploy.Snapshot,
					changes display.ResourceChanges, events []engine.Event,
				) {
					require.NoError(l, err)
					policyViolations := getPolicyViolationEvents(l, events)
					require.Empty(l, policyViolations, "expected no policy violations")
				},
			},
			{
				// Run again against the other resource, this should warn about validation.
				PolicyPacks: map[string]map[string]any{
					"config-schema": {
						"validator": map[string]any{
							"value": false,
							"names": []string{"resY"},
						},
					},
				},
				AssertPreview: func(
					l *L, projectDirectory string, err error, plan *deploy.Plan,
					changes display.ResourceChanges, events []engine.Event,
				) {
					require.NoError(l, err)
					policyViolations := getPolicyViolationEvents(l, events)
					require.Len(l, policyViolations, 1, "expected one policy violation")
					require.Equal(l, engine.PolicyViolationEventPayload{
						ResourceURN:       "urn:pulumi:test::policy-config-schema::simple:index:Resource::resY",
						Message:           "<{%reset%}>Verifies property matches config\nProperty was true<{%reset%}>\n",
						Color:             "raw",
						PolicyName:        "validator",
						PolicyPackName:    "config-schema",
						PolicyPackVersion: "3.0.0",
						EnforcementLevel:  "advisory",
						Prefix:            "<{%fg 3%}>advisory: <{%reset%}>",
					}, policyViolations[0])
				},
				Assert: func(l *L,
					projectDirectory string, err error, snap *deploy.Snapshot,
					changes display.ResourceChanges, events []engine.Event,
				) {
					require.NoError(l, err)
					policyViolations := getPolicyViolationEvents(l, events)
					require.Len(l, policyViolations, 1, "expected one policy violation")
				},
			},
		},
	}
}
