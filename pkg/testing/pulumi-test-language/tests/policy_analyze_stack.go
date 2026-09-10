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
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	// A pack whose only policy is a stack validation policy: it examines the
	// full set of resources in the stack and reports a violation if more than
	// one "simple:index:Resource" is present. This exercises the AnalyzerServer's
	// AnalyzeStack RPC, which is otherwise not covered by any conformance test.
	//
	// The program uses `options { range = extraCount }` on the second resource,
	// so the two runs share source but produce different stack contents:
	//   extraCount=0 -> one resource,  no violation
	//   extraCount=1 -> two resources, one stack-level violation
	expectedViolation := engine.PolicyViolationEventPayload{
		ResourceURN: "urn:pulumi:test::policy-analyze-stack::pulumi:pulumi:Stack::policy-analyze-stack-test",
		Message: "<{%reset%}>Stack must contain at most one simple:index:Resource\n" +
			"Found an extra simple:index:Resource<{%reset%}>\n",
		Color:             "raw",
		PolicyName:        "stack-size",
		PolicyPackName:    "analyze-stack",
		PolicyPackVersion: "1.0.0",
		EnforcementLevel:  "mandatory",
		Prefix:            "<{%fg 1%}>mandatory: <{%reset%}>",
	}

	collectViolations := func(events []engine.Event) []engine.PolicyViolationEventPayload {
		var policyViolations []engine.PolicyViolationEventPayload
		for _, event := range events {
			if event.Type == engine.PolicyViolationEvent {
				policyViolations = append(policyViolations, event.Payload().(engine.PolicyViolationEventPayload))
			}
		}
		return policyViolations
	}

	extraCountKey := config.MustMakeKey("policy-analyze-stack", "extraCount")

	LanguageTests["policy-analyze-stack"] = LanguageTest{
		RunsShareSource: true,
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.SimpleProvider{} },
		},
		Runs: []TestRun{
			// One resource: policy should not fire.
			{
				Config: config.Map{
					extraCountKey: config.NewValue("0"),
				},
				PolicyPacks: map[string]map[string]any{
					"analyze-stack": nil,
				},
				AssertPreview: func(l *L, res AssertPreviewArgs) {
					require.NoError(l, res.Err)
					require.Empty(l, collectViolations(res.Events), "expected no policy violations")
				},
				Assert: func(l *L, res AssertArgs) {
					require.NoError(l, res.Err)
					require.Empty(l, collectViolations(res.Events), "expected no policy violations")
				},
			},
			// Two resources: policy should fire with a single stack-level violation.
			{
				Config: config.Map{
					extraCountKey: config.NewValue("1"),
				},
				PolicyPacks: map[string]map[string]any{
					"analyze-stack": nil,
				},
				AssertPreview: func(l *L, res AssertPreviewArgs) {
					require.ErrorContains(l, res.Err, "BAIL: step generator errored")
					violations := collectViolations(res.Events)
					require.Len(l, violations, 1, "expected 1 policy violation")
					assert.Contains(l, violations, expectedViolation)
				},
				Assert: func(l *L, res AssertArgs) {
					require.ErrorContains(l, res.Err, "BAIL: step generator errored")
					violations := collectViolations(res.Events)
					require.Len(l, violations, 1, "expected 1 policy violation")
					assert.Contains(l, violations, expectedViolation)
				},
			},
		},
	}
}
