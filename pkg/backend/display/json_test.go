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

package display

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testBucketURN = "urn:pulumi:dev::proj::aws:s3/bucket:Bucket::my-bucket"

// policyEventJSON returns a serialized policyEvent engine event with the given enforcement level and exceptions.
func policyEventJSON(enforcementLevel, exceptions string) string {
	return fmt.Sprintf(`{
		"policyEvent": {
			"resourceUrn": %q,
			"message": "<{%%fg 1%%}>Buckets must set serverSideEncryptionConfiguration.<{%%reset%%}>\n",
			"color": "raw",
			"policyName": "s3-bucket-encryption",
			"policyPackName": "aws-security",
			"policyPackVersion": "1.2.3",
			"enforcementLevel": %q,
			"severity": "high"%s
		}
	}`, testBucketURN, enforcementLevel, exceptions)
}

const errorDiagnosticJSON = `{
	"diagnosticEvent": {
		"prefix": "error: ",
		"message": "preview failed\n",
		"color": "raw",
		"severity": "error"
	}
}`

const summaryJSON = `{
	"summaryEvent": {
		"maybeCorrupt": false,
		"durationSeconds": 1,
		"resourceChanges": {"create": 1},
		"PolicyPacks": {"aws-security": "v1.2.3"},
		"isPreview": true
	}
}`

// convertJSONEvents deserializes the given JSON engine events into engine events.
func convertJSONEvents(t *testing.T, jsonEvents ...string) []engine.Event {
	t.Helper()

	events := make([]engine.Event, 0, len(jsonEvents))
	for _, s := range jsonEvents {
		var apiEvent apitype.EngineEvent
		require.NoError(t, json.Unmarshal([]byte(s), &apiEvent))
		event, err := ConvertJSONEvent(apiEvent)
		require.NoError(t, err)
		events = append(events, event)
	}
	return events
}

// renderPreviewDigest runs ShowPreviewDigest over the given events and returns its stdout.
func renderPreviewDigest(t *testing.T, events []engine.Event) []byte {
	t.Helper()

	var stdout bytes.Buffer
	opts := defaultOpts()
	opts.Color = colors.Never
	opts.Stdout = &stdout
	done := make(chan bool)
	go ShowPreviewDigest(sliceToBufferedChan(events), done, opts)
	<-done
	return stdout.Bytes()
}

// expectedPolicyViolation is the violation encoded by policyEventJSON, with colorization elided.
func expectedPolicyViolation(enforcementLevel string) apitype.PolicyEvent {
	return apitype.PolicyEvent{
		ResourceURN:          testBucketURN,
		Message:              "Buckets must set serverSideEncryptionConfiguration.\n",
		Color:                string(colors.Never),
		PolicyName:           "s3-bucket-encryption",
		PolicyPackName:       "aws-security",
		PolicyPackVersion:    "1.2.3",
		PolicyPackVersionTag: "1.2.3",
		EnforcementLevel:     enforcementLevel,
		Severity:             "high",
	}
}

func TestPreviewDigestMandatoryPolicyViolation(t *testing.T) {
	t.Parallel()

	events := convertJSONEvents(t, policyEventJSON("mandatory", ""), errorDiagnosticJSON, summaryJSON)
	out := renderPreviewDigest(t, events)

	var digest display.PreviewDigest
	require.NoError(t, json.Unmarshal(out, &digest), "output: %s", out)

	assert.Equal(t, []apitype.PolicyEvent{expectedPolicyViolation("mandatory")}, digest.PolicyViolations)
	assert.Equal(t, []display.PreviewDiagnostic{{
		Message:  "error: preview failed\n",
		Severity: diag.Error,
	}}, digest.Diagnostics)
	assert.Equal(t, display.ResourceChanges{"create": 1}, digest.ChangeSummary)
}

func TestPreviewDigestAdvisoryPolicyViolation(t *testing.T) {
	t.Parallel()

	events := convertJSONEvents(t, policyEventJSON("advisory", ""), summaryJSON)
	out := renderPreviewDigest(t, events)

	var digest display.PreviewDigest
	require.NoError(t, json.Unmarshal(out, &digest), "output: %s", out)

	assert.Equal(t, []apitype.PolicyEvent{expectedPolicyViolation("advisory")}, digest.PolicyViolations)
	assert.Empty(t, digest.Diagnostics)
}

func TestPreviewDigestNoPolicyViolations(t *testing.T) {
	t.Parallel()

	events := convertJSONEvents(t, summaryJSON)
	out := renderPreviewDigest(t, events)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(out, &raw), "output: %s", out)
	assert.NotContains(t, raw, "policyViolations")
	assert.Contains(t, raw, "changeSummary")
}

func TestPreviewDigestExceptedPolicyViolation(t *testing.T) {
	t.Parallel()

	exceptions := `,
			"exceptions": [{"id": "EXC-42", "reason": "Legacy bucket, migration tracked separately"}]`
	events := convertJSONEvents(t, policyEventJSON("mandatory", exceptions), summaryJSON)
	out := renderPreviewDigest(t, events)

	var digest display.PreviewDigest
	require.NoError(t, json.Unmarshal(out, &digest), "output: %s", out)

	expected := expectedPolicyViolation("mandatory")
	expected.Exceptions = []apitype.PolicyEventException{
		{ID: "EXC-42", Reason: "Legacy bucket, migration tracked separately"},
	}
	assert.Equal(t, []apitype.PolicyEvent{expected}, digest.PolicyViolations)
}

func TestShowEventsJSONPreviewIncludesPolicyViolations(t *testing.T) {
	t.Parallel()

	events := convertJSONEvents(t, policyEventJSON("mandatory", ""), errorDiagnosticJSON, summaryJSON)

	var stdout, stderr bytes.Buffer
	opts := Options{
		JSONDisplay: true,
		Color:       colors.Never,
		Stdout:      &stdout,
		Stderr:      &stderr,
	}
	done := make(chan bool)
	go ShowEvents("preview", apitype.PreviewUpdate, tokens.MustParseStackName("dev"), "proj", "",
		sliceToBufferedChan(events), done, opts, true /* isPreview */)
	<-done

	var digest display.PreviewDigest
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &digest), "output: %s", stdout.String())
	assert.Equal(t, []apitype.PolicyEvent{expectedPolicyViolation("mandatory")}, digest.PolicyViolations)
}
