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
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/pkg/v3/testing/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Verifies that policy packs can inspect known and secret-known property
// values and surface violations for them, and that unknown (and secret-unknown)
// values during preview cause the policy to emit a "can't run policy ...
// during preview" diagnostic rather than evaluate the policy body.
func init() {
	const (
		urnUnknown       = "urn:pulumi:test::policy-unknowns::simple:index:Resource::unknownRes"
		urnSecretUnknown = "urn:pulumi:test::policy-unknowns::simple:index:Resource::secretUnknownRes" //nolint:gosec
		urnKnown         = "urn:pulumi:test::policy-unknowns::simple:index:Resource::knownRes"         //nolint:gosec
		urnSecretKnown   = "urn:pulumi:test::policy-unknowns::simple:index:Resource::secretKnownRes"   //nolint:gosec
	)

	collect := func(events []engine.Event) []engine.PolicyViolationEventPayload {
		var violations []engine.PolicyViolationEventPayload
		for _, e := range events {
			if e.Type == engine.PolicyViolationEvent {
				violations = append(violations, e.Payload().(engine.PolicyViolationEventPayload))
			}
		}
		return violations
	}

	findViolation := func(
		violations []engine.PolicyViolationEventPayload, urn string,
	) *engine.PolicyViolationEventPayload {
		for i, v := range violations {
			if v.ResourceURN == resource.URN(urn) {
				return &violations[i]
			}
		}
		return nil
	}

	assertValueViolation := func(l *L, violations []engine.PolicyViolationEventPayload, urn, boolMsg string) {
		v := findViolation(violations, urn)
		require.NotNilf(l, v, "no violation for %s", urn)
		assert.Containsf(l, v.Message, boolMsg, "violation for %s should mention %q", urn, boolMsg)
		assert.Equal(l, "advisory", string(v.EnforcementLevel))
		assert.Equal(l, "report", v.PolicyName)
	}

	assertSkipViolation := func(l *L, violations []engine.PolicyViolationEventPayload, urn string) {
		v := findViolation(violations, urn)
		require.NotNilf(l, v, "no violation for %s", urn)
		// The exact message varies across language SDKs; assert on the stable
		// substrings that identify a "policy couldn't run" diagnostic.
		assert.Truef(l,
			strings.Contains(v.Message, "can't run policy") &&
				strings.Contains(v.Message, "can't be known during preview"),
			"violation for %s should look like a skip-during-preview message, got %q", urn, v.Message)
		assert.Equal(l, "report", v.PolicyName)
	}

	LanguageTests["policy-unknowns"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.SimpleProvider{} },
			func() plugin.Provider { return &providers.OutputProvider{} },
		},
		Runs: []TestRun{
			{
				PolicyPacks: map[string]map[string]any{
					"unknowns": nil,
				},
				AssertPreview: func(l *L, res AssertPreviewArgs) {
					require.NoError(l, res.Err)

					violations := collect(res.Events)
					require.Len(l, violations, 4, "expected one violation per simple resource, got %v", violations)

					assertSkipViolation(l, violations, urnUnknown)
					assertSkipViolation(l, violations, urnSecretUnknown)
					assertValueViolation(l, violations, urnKnown, "value is false")
					assertValueViolation(l, violations, urnSecretKnown, "value is false")
				},
				Assert: func(l *L, res AssertArgs) {
					require.NoError(l, res.Err)

					violations := collect(res.Events)
					require.Len(l, violations, 4, "expected one violation per simple resource, got %v", violations)

					assertValueViolation(l, violations, urnUnknown, "value is true")
					assertValueViolation(l, violations, urnSecretUnknown, "value is true")
					assertValueViolation(l, violations, urnKnown, "value is false")
					assertValueViolation(l, violations, urnSecretKnown, "value is false")
				},
			},
		},
	}
}
