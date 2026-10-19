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

package policy

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// twoBuckets is an update where required-tags failed on both buckets, and s3-no-public-read only on cdn-logs.
func twoBuckets() *fakeExceptionClient {
	return &fakeExceptionClient{
		updates: []apitype.UpdateInfo{{Version: 12}},
		events: map[string][]apitype.EngineEvent{"update/12": {
			violation("aws", "required-tags", "mandatory", logsURN),
			violation("aws", "required-tags", "mandatory", cdnLogsURN),
			violation("aws", "s3-no-public-read", "mandatory", cdnLogsURN),
			violation("aws", "s3-no-public-read", "mandatory", cdnLogsURN),
			violation("aws", "unencrypted", "advisory", logsURN),
		}},
	}
}

func match(policies []string, targets ...string) apitype.PolicyExceptionMatch {
	return apitype.PolicyExceptionMatch{Policies: policies, Targets: targets}
}

func TestPolicyExceptionNewFiltersGroupByURN(t *testing.T) {
	t.Parallel()
	c := twoBuckets()

	stdout, stderr, code := runExceptionCmd(t, testExceptionEnv(c), "", "new", "aws", "--reason", "LOG-31",
		"--policy", "required-tags", "--policy", "s3-no-public-read", "--target", logsURN, "--target", cdnLogsURN)
	require.Equal(t, 0, code, stderr)

	assert.Equal(t, "EXC-7\n", stdout, "stdout holds only the ID")
	assert.Contains(t, stderr, "Created EXC-7\n  https://app.pulumi.com/acme/insights/policy/exceptions/EXC-7")
	assert.Equal(t, []string{"update/12 9"}, c.reads)
	assert.Equal(t, []apitype.CreatePolicyExceptionRequest{{
		PolicyPack: "aws",
		Reason:     "LOG-31",
		Match: []apitype.PolicyExceptionMatch{
			match([]string{"required-tags"}, logsURN),
			match([]string{"required-tags", "s3-no-public-read"}, cdnLogsURN),
		},
	}}, c.created)
}

func TestPolicyExceptionNewDryRunJSON(t *testing.T) {
	t.Parallel()
	c := twoBuckets()

	stdout, stderr, code := runExceptionCmd(t, testExceptionEnv(c), "", "new", "--dry-run", "--output", "json")
	require.Equal(t, 0, code, stderr)
	assert.Empty(t, c.created)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	assert.Equal(t, []any{map[string]any{"stack": "acme/web/prod", "kind": "update", "id": "12"}}, result["runs"])
	violations := result["violations"].([]any)
	require.Len(t, violations, 4)
	assert.Equal(t, map[string]any{
		"stack": "acme/web/prod", "policyPack": "aws", "policy": "required-tags", "level": "mandatory", "urn": logsURN,
	}, violations[0])
	assert.Equal(t, "advisory", violations[3].(map[string]any)["level"], "mandatory violations come first")
	assert.Equal(t, "aws", result["exception"].(map[string]any)["policyPack"])
	assert.Equal(t, true, result["dryRun"])
}

func TestPolicyExceptionNewDryRunText(t *testing.T) {
	t.Parallel()
	c := twoBuckets()

	stdout, stderr, code := runExceptionCmd(t, testExceptionEnv(c), "",
		"new", "aws", "--policy", "required-tags", "--dry-run")
	require.Equal(t, 0, code, stderr)
	assert.Contains(t, stdout, "aws/required-tags  aws:s3/bucket:Bucket logs  (mandatory)\n    "+logsURN)
	assert.Contains(t, stdout, "version: 2\nexceptions:\n  EXC-?:\n")
	assert.Empty(t, c.created)
}

func TestPolicyExceptionNewLatestRun(t *testing.T) {
	t.Parallel()
	c := twoBuckets()
	c.previews = []apitype.StackPreview{{UpdateID: "p-1"}}
	c.events["preview/p-1"] = []apitype.EngineEvent{violation("aws", "required-tags", "mandatory", logsURN)}

	_, stderr, code := runExceptionCmd(t, testExceptionEnv(c), "", "new", "aws", "--dry-run")
	require.Equal(t, 0, code, stderr)
	assert.Equal(t, []string{"preview/p-1 9"}, c.reads, "a preview since the latest update is the latest run")

	_, _, code = runExceptionCmd(t, testExceptionEnv(c), "", "new", "aws", "--dry-run", "--update")
	require.Equal(t, 0, code)
	assert.Equal(t, "update/12 9", c.reads[1], "a bare --update is the latest update")

	_, _, code = runExceptionCmd(t, testExceptionEnv(c), "", "new", "aws", "--dry-run", "--preview=p-0")
	require.Equal(t, 0, code)
	assert.Equal(t, "preview/p-0 9", c.reads[2])

	_, _, code = runExceptionCmd(t, testExceptionEnv(&fakeExceptionClient{}), "", "new", "aws", "--dry-run")
	assert.Equal(t, 1, code, "no runs")
}

func TestPolicyExceptionNewSkipsCovered(t *testing.T) {
	t.Parallel()
	c := twoBuckets()
	c.events["update/12"] = append(c.events["update/12"], violation("aws", "other", "mandatory", logsURN, "EXC-1"))
	c.active = []apitype.PolicyExceptionInfo{{ID: "EXC-2", PolicyPack: "aws", Match: []apitype.PolicyExceptionMatch{
		match([]string{"required-tags"}, logsURN, cdnLogsURN),
	}}}
	c.requests = []apitype.ChangeRequest{{ID: "cr-9", Status: "pending"}, {ID: "cr-8", Status: "applied"}}
	c.drafts = map[string]apitype.PolicyExceptionDraft{"cr-9": {PolicyPack: "aws", Match: []apitype.PolicyExceptionMatch{
		match([]string{"s3-no-public-read", "unencrypted"}, logsURN, cdnLogsURN),
	}}}

	stdout, stderr, code := runExceptionCmd(t, testExceptionEnv(c), "", "new", "aws", "--reason", "x")
	require.Equal(t, 0, code, stderr)
	assert.Empty(t, stdout)
	assert.Empty(t, c.created)
	assert.Contains(t, stderr, "already allowed by EXC-1, EXC-2, change request cr-9")
	assert.Equal(t, "web/prod", c.queries[0].Stack)
}

func TestPolicyExceptionNewSeveralStacks(t *testing.T) {
	t.Parallel()
	c := twoBuckets()
	devURN := "urn:pulumi:dev::web::aws:s3/bucket:Bucket::logs"
	c.events["update/12"] = append(c.events["update/12"], violation("aws", "required-tags", "mandatory", devURN))

	_, stderr, code := runExceptionCmd(t, testExceptionEnv(c, testStack("dev"), testStack("prod")), "",
		"new", "aws", "--stack", "*", "--policy", "required-tags", "--reason", "x")
	require.Equal(t, 0, code, stderr)
	require.Len(t, c.created, 1)
	assert.Equal(t, []apitype.PolicyExceptionMatch{
		match([]string{"required-tags"}, logsURN),
		match([]string{"required-tags"}, cdnLogsURN),
		match([]string{"required-tags"}, devURN),
	}, c.created[0].Match)
}

func TestPolicyExceptionNewFile(t *testing.T) {
	t.Parallel()
	for name, input := range map[string]string{
		"json": `{"reason": "LOG-31", "match": [{"policies": ["required-tags"], "targets": ["` + logsURN + `"]}]}`,
		"yaml": "reason: LOG-31\nmatch:\n  - policies: [required-tags]\n    targets: [" + logsURN + "]\n",
		"v2 config": "version: 2\nexceptions:\n  EXC-?:\n    reason: LOG-31\n    match:\n" +
			"      - policies: [required-tags]\n        targets: [" + logsURN + "]\n",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c := &fakeExceptionClient{}
			_, stderr, code := runExceptionCmd(t, testExceptionEnv(c), input, "new", "aws", "--file", "-")
			require.Equal(t, 0, code, stderr)
			assert.Equal(t, []apitype.CreatePolicyExceptionRequest{{
				PolicyPack: "aws",
				Reason:     "LOG-31",
				Match:      []apitype.PolicyExceptionMatch{match([]string{"required-tags"}, logsURN)},
			}}, c.created)
			assert.Empty(t, c.reads, "a definition doesn't read the run")
		})
	}
}

func TestPolicyExceptionNewInvalid(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		stdin string
		args  []string
		want  string
	}{
		"no reason":     {args: []string{"new", "aws", "--policy", "required-tags"}, want: "--reason is required"},
		"no pack, file": {stdin: "{}", args: []string{"new", "--file", "-"}, want: "a policy pack is required"},
		"bad urn": {
			stdin: `{"reason": "x", "match": [{"policies": ["p"], "targets": ["logs"]}]}`,
			args:  []string{"new", "aws", "--file", "-"}, want: `invalid target "logs"`,
		},
		"no entries":       {stdin: `{"reason": "x"}`, args: []string{"new", "aws", "--file", "-"}, want: "no match entries"},
		"update & preview": {args: []string{"new", "aws", "--update", "--preview"}, want: "can't be used together"},
		"bad pack":         {args: []string{"new", "a/b/c"}, want: "expected [<org>/]<policy-pack>"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c := twoBuckets()
			_, stderr, code := runExceptionCmd(t, testExceptionEnv(c), tc.stdin, tc.args...)
			assert.Equal(t, 2, code)
			assert.Contains(t, stderr, tc.want)
			assert.Empty(t, c.created)
		})
	}
}

func TestPolicyExceptionNewSeveralPacks(t *testing.T) {
	t.Parallel()
	c := twoBuckets()
	c.events["update/12"] = append(c.events["update/12"], violation("gcp", "required-labels", "mandatory", logsURN))

	_, stderr, code := runExceptionCmd(t, testExceptionEnv(c), "", "new", "--reason", "x")
	assert.Equal(t, 2, code)
	assert.Contains(t, stderr, "several policy packs (aws, gcp)")

	_, stderr, code = runExceptionCmd(t, testExceptionEnv(c), "", "new", "--policy", "required-labels", "--reason", "x")
	require.Equal(t, 0, code, stderr)
	assert.Equal(t, "gcp", c.created[0].PolicyPack, "one pack is inferred")
}

func TestPolicyExceptionNewRequiresApproval(t *testing.T) {
	t.Parallel()
	c := twoBuckets()
	c.createErr = &apitype.ErrorResponse{
		Code: http.StatusConflict, Message: "creating policy exceptions in acme requires approval; request it as a draft",
	}

	stdout, stderr, code := runExceptionCmd(t, testExceptionEnv(c), "",
		"new", "aws", "--policy", "required-tags", "--reason", "LOG-31", "--note", "until Q3", "--output", "json")
	require.Equal(t, 0, code, stderr)
	require.Len(t, c.drafted, 1)
	assert.Equal(t, []string{"cr-1: until Q3"}, c.submitted)
	assert.Contains(t, stderr, "change request cr-1")

	var result policyExceptionNewResult
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	assert.Equal(t, "cr-1", result.ChangeRequestID)
	assert.Equal(t, "https://app.pulumi.com/acme/change-requests/cr-1", result.URL)
}

func TestPolicyExceptionNewDisabled(t *testing.T) {
	t.Parallel()
	c := twoBuckets()
	c.createErr = &apitype.ErrorResponse{
		Code: http.StatusConflict, Message: "policy exceptions are disabled for this organization",
	}

	_, stderr, code := runExceptionCmd(t, testExceptionEnv(c), "", "new", "aws", "--reason", "x")
	assert.Equal(t, 1, code)
	assert.Contains(t, stderr, "disabled")
	assert.Empty(t, c.drafted)
}
