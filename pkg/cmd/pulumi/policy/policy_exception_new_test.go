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
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	cmdCmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

const cdnLogsURN = "urn:pulumi:prod::web::aws:s3/bucket:Bucket::cdn-logs"

func policyViolation(pack, policy, level, urn string, exceptions ...string) apitype.EngineEvent {
	p := &apitype.PolicyEvent{PolicyPackName: pack, PolicyName: policy, EnforcementLevel: level, ResourceURN: urn}
	for _, id := range exceptions {
		p.Exceptions = append(p.Exceptions, apitype.PolicyEventException{ID: id})
	}
	return apitype.EngineEvent{PolicyEvent: p}
}

// twoBuckets is an update where required-tags failed on both buckets, s3-no-public-read only on cdn-logs, and the
// advisory unencrypted only on logs.
func twoBuckets() *mockPolicyExceptionClient {
	return &mockPolicyExceptionClient{
		updates: []apitype.UpdateInfo{{Version: 12}},
		events: map[string][]apitype.EngineEvent{"update/12": {
			policyViolation("aws", "unencrypted", "advisory", logsURN),
			policyViolation("aws", "required-tags", "mandatory", logsURN),
			policyViolation("aws", "required-tags", "mandatory", cdnLogsURN),
			policyViolation("aws", "s3-no-public-read", "mandatory", cdnLogsURN),
			policyViolation("aws", "s3-no-public-read", "mandatory", cdnLogsURN),
		}},
	}
}

func match(policies []string, targets ...string) apitype.PolicyExceptionMatch {
	return apitype.PolicyExceptionMatch{Policies: policies, Targets: targets}
}

// stubStackResolver resolves a name to a stack of acme/web, and the current stack to prod.
func stubStackResolver(_ context.Context, name string) (client.StackIdentifier, error) {
	if name == "" {
		name = "prod"
	}
	return client.StackIdentifier{Owner: "acme", Project: "web", Stack: tokens.MustParseStackName(name)}, nil
}

// runNew runs the command without a terminal, and returns its stdout and stderr.
func runNew(
	t *testing.T, c *mockPolicyExceptionClient, stdin string, args policyExceptionNewArgs,
) (string, string, error) {
	t.Helper()
	args.outputFormat = defaultPolicyExceptionNewOutputFormat()
	var stdout, stderr bytes.Buffer
	err := runPolicyExceptionNew(t.Context(), &stdout, &stderr, strings.NewReader(stdin),
		stubPolicyExceptionFactory(c, "acme"), stubStackResolver, false, args)
	return stdout.String(), stderr.String(), err
}

func TestPolicyExceptionNew(t *testing.T) {
	t.Parallel()

	c := twoBuckets()
	stdout, stderr, err := runNew(t, c, "", policyExceptionNewArgs{
		policyPack: "aws",
		reason:     "LOG-31",
		policies:   []string{"required-tags", "s3-no-public-read"},
		targets:    []string{logsURN, cdnLogsURN},
	})
	require.NoError(t, err)
	assert.Empty(t, stdout)
	assert.Equal(t, "Created policy exception EXC-7: http://localhost:3000/acme/web/prod?exception=EXC-7\n", stderr)
	assert.Equal(t, []string{"acme/web/prod update/12 [9]"}, c.gotReads)
	assert.Empty(t, c.gotListOpts, "existing exceptions aren't read")
	assert.Equal(t, []apitype.CreatePolicyExceptionRequest{{
		PolicyPack: "aws",
		PolicyException: apitype.PolicyException{
			Reason: "LOG-31",
			Match: []apitype.PolicyExceptionMatch{
				match([]string{"required-tags"}, logsURN),
				match([]string{"required-tags", "s3-no-public-read"}, cdnLogsURN),
			},
		},
	}}, c.gotCreated)

	c = twoBuckets()
	_, _, err = runNew(t, c, "", policyExceptionNewArgs{
		policyPack: "aws", reason: "LOG-31", policies: []string{"s3-no-public-read"},
	})
	require.NoError(t, err)
	assert.Equal(t, []apitype.PolicyExceptionMatch{match([]string{"s3-no-public-read"}, cdnLogsURN)},
		c.gotCreated[0].Match, "only the pairs that occurred in the run")

	// Pulumi Cloud returns the existing exception for a repeated request.
	c = twoBuckets()
	c.existed = true
	stdout, stderr, err = runNew(t, c, "", policyExceptionNewArgs{policyPack: "aws", reason: "LOG-31"})
	require.NoError(t, err)
	assert.Empty(t, stdout)
	assert.Equal(t, "Updated policy exception EXC-7: http://localhost:3000/acme/web/prod?exception=EXC-7\n", stderr)
}

func TestPolicyExceptionNew_Levels(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		levels []string
		want   []string
	}{
		{[]string{"mandatory"}, []string{"required-tags", "required-tags", "s3-no-public-read"}},
		{[]string{"advisory"}, []string{"unencrypted"}},
		{[]string{"Advisory", "mandatory"}, []string{"required-tags", "required-tags", "s3-no-public-read", "unencrypted"}},
		{[]string{"ALL"}, []string{"required-tags", "required-tags", "s3-no-public-read", "unencrypted"}},
	} {
		args := policyExceptionNewArgs{dryRun: true, levels: tc.levels, outputFormat: defaultPolicyExceptionNewOutputFormat()}
		require.NoError(t, args.outputFormat.Set("json"))
		var buf bytes.Buffer
		require.NoError(t, runPolicyExceptionNew(t.Context(), &buf, &bytes.Buffer{}, nil,
			stubPolicyExceptionFactory(twoBuckets(), "acme"), stubStackResolver, false, args))
		var result policyExceptionNewResult
		require.NoError(t, json.Unmarshal(buf.Bytes(), &result))
		got := make([]string, 0, len(result.Violations))
		for _, v := range result.Violations {
			got = append(got, v.Policy)
		}
		assert.Equal(t, tc.want, got, "--level %v", tc.levels)
	}

	_, _, err := runNew(t, twoBuckets(), "", policyExceptionNewArgs{dryRun: true, levels: []string{"warn"}})
	assert.Equal(t, cmdCmd.ConfigurationError{
		Message: `invalid --level "warn": expected mandatory, advisory, remediate or all`,
	}, err)
}

func TestPolicyExceptionNew_DryRun(t *testing.T) {
	t.Parallel()

	c := twoBuckets()
	out, stderr, err := runNew(t, c, "", policyExceptionNewArgs{
		policyPack: "aws", policies: []string{"required-tags"}, dryRun: true,
	})
	require.NoError(t, err)
	assert.Empty(t, stderr)
	assert.Empty(t, c.gotCreated)
	assert.Equal(t, `acme/web/prod: update 12

Violations to exempt:
┌─────────────┬─────────────┬───────────────┬──────────────────────────────────────────────────────┐
│ LEVEL       │ POLICY PACK │ POLICY        │ URN                                                  │
├─────────────┼─────────────┼───────────────┼──────────────────────────────────────────────────────┤
│ [mandatory] │ aws         │ required-tags │ urn:pulumi:prod::web::aws:s3/bucket:Bucket::logs     │
│ [mandatory] │ aws         │ required-tags │ urn:pulumi:prod::web::aws:s3/bucket:Bucket::cdn-logs │
└─────────────┴─────────────┴───────────────┴──────────────────────────────────────────────────────┘

Policy Pack exceptions configuration:
{
  "reason": "",
  "match": [
    {
      "policies": [
        "required-tags"
      ],
      "targets": [
        "urn:pulumi:prod::web::aws:s3/bucket:Bucket::logs"
      ]
    },
    {
      "policies": [
        "required-tags"
      ],
      "targets": [
        "urn:pulumi:prod::web::aws:s3/bucket:Bucket::cdn-logs"
      ]
    }
  ]
}
`, out)

	args := policyExceptionNewArgs{dryRun: true, outputFormat: defaultPolicyExceptionNewOutputFormat()}
	require.NoError(t, args.outputFormat.Set("json"))
	var buf bytes.Buffer
	c = twoBuckets()
	exempted := policyViolation("aws", "s3-no-public-read", "mandatory", logsURN, "EXC-1")
	exempted.PolicyEvent.Severity = "high"
	c.events["update/12"] = append(c.events["update/12"], exempted)
	err = runPolicyExceptionNew(t.Context(), &buf, &bytes.Buffer{}, nil, stubPolicyExceptionFactory(c, "acme"),
		stubStackResolver, true, args)
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"organizationName": "acme",
		"runs": [{"stack": "acme/web/prod", "kind": "update", "id": "12"}],
		"violations": [
			{"stack": "acme/web/prod", "policyPack": "aws", "policy": "required-tags", "level": "mandatory", "urn": "`+
		logsURN+`"},
			{"stack": "acme/web/prod", "policyPack": "aws", "policy": "required-tags", "level": "mandatory", "urn": "`+
		cdnLogsURN+`"},
			{"stack": "acme/web/prod", "policyPack": "aws", "policy": "s3-no-public-read", "level": "mandatory", "urn": "`+
		cdnLogsURN+`"},
			{"stack": "acme/web/prod", "policyPack": "aws", "policy": "s3-no-public-read", "level": "mandatory",
				"severity": "high", "urn": "`+logsURN+`", "exceptions": ["EXC-1"]},
			{"stack": "acme/web/prod", "policyPack": "aws", "policy": "unencrypted", "level": "advisory", "urn": "`+
		logsURN+`"}
		],
		"exception": {
			"policyPack": "aws",
			"reason": "",
			"match": [
				{"policies": ["required-tags", "s3-no-public-read", "unencrypted"], "targets": ["`+logsURN+`"]},
				{"policies": ["required-tags", "s3-no-public-read"], "targets": ["`+cdnLogsURN+`"]}
			]
		},
		"dryRun": true
	}`, buf.String(), "a dry run doesn't prompt, even in a terminal")
}

func TestPolicyExceptionNew_LatestRun(t *testing.T) {
	t.Parallel()

	c := twoBuckets()
	c.previews = []apitype.StackPreview{{UpdateID: "p-1"}}
	for _, args := range []policyExceptionNewArgs{
		{dryRun: true},
		{dryRun: true, update: latestRun},
		{dryRun: true, preview: "p-0"},
		{dryRun: true, update: "3"},
		{dryRun: true, stacks: []string{"dev", "prod"}},
	} {
		_, _, err := runNew(t, c, "", args)
		require.NoError(t, err)
	}
	assert.Equal(t, []string{
		"acme/web/prod preview/p-1 [9]",
		"acme/web/prod update/12 [9]",
		"acme/web/prod preview/p-0 [9]",
		"acme/web/prod update/3 [9]",
		"acme/web/dev preview/p-1 [9]",
		"acme/web/prod preview/p-1 [9]",
	}, c.gotReads)

	_, _, err := runNew(t, &mockPolicyExceptionClient{}, "", policyExceptionNewArgs{dryRun: true})
	assert.EqualError(t, err, "acme/web/prod has no updates or previews; run `pulumi preview` first")
	_, _, err = runNew(t, &mockPolicyExceptionClient{}, "", policyExceptionNewArgs{dryRun: true, preview: latestRun})
	assert.EqualError(t, err, "acme/web/prod has no previews since its latest update")
}

func TestPolicyExceptionNew_NoViolations(t *testing.T) {
	t.Parallel()

	stdout, stderr, err := runNew(t, twoBuckets(), "", policyExceptionNewArgs{policyPack: "gcp", reason: "x"})
	require.NoError(t, err)
	assert.Empty(t, stdout)
	assert.Equal(t, "No violations of Policy Pack \"gcp\" to exempt.\n", stderr)

	_, stderr, err = runNew(t, &mockPolicyExceptionClient{updates: []apitype.UpdateInfo{{Version: 1}}}, "",
		policyExceptionNewArgs{reason: "x"})
	require.NoError(t, err)
	assert.Equal(t, "No violations to exempt.\n", stderr)
}

func TestPolicyExceptionViolationTags(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		v    policyExceptionViolation
		want string
	}{
		{policyExceptionViolation{Level: "mandatory"}, "[mandatory]"},
		{policyExceptionViolation{Level: "advisory", Severity: "low"}, "[advisory] [severity: low]"},
		{
			policyExceptionViolation{Level: "mandatory", Exceptions: []string{"EXC-1", "EXC-2"}},
			"[exempted: EXC-1, EXC-2] [mandatory]",
		},
	} {
		assert.Equal(t, tc.want, policyExceptionViolationTags(tc.v, colors.Never))
	}
	assert.Equal(t, "\x1b[38;5;1m[mandatory]\x1b[0m",
		policyExceptionViolationTags(policyExceptionViolation{Level: "mandatory"}, colors.Always))
}

func TestPolicyExceptionPickerLabels(t *testing.T) {
	t.Parallel()

	const parent = "urn:pulumi:prod::web::my:app:Site$aws:s3/bucket:Bucket::logs"
	const long = "urn:pulumi:prod::web::aws:s3/bucket:Bucket::access-logs-for-the-public-website-"
	violations := []policyExceptionViolation{
		{Stack: "acme/web/prod", Policy: "required-tags", Level: "mandatory", URN: logsURN},
		{Stack: "acme/web/prod", Policy: "required-tags", Level: "mandatory", URN: parent},
		{Stack: "acme/web/prod", Policy: "required-tags", Level: "mandatory", URN: long + "eu"},
		{Stack: "acme/web/prod", Policy: "required-tags", Level: "mandatory", URN: long + "us"},
	}

	assert.Equal(t, []string{
		"[mandatory]  required-tags  (aws:s3:Bucket: logs)",
		"[mandatory]  required-tags  (aws:s3:Bucket: logs) (2)",
		"[mandatory]  required-tags  (aws:s3:Bucket: access-logs-for-the-public-…",
		"[mandatory]  required-tags  (aws:s3:Bucket: access-logs-for-the-pub… (2)",
	}, policyExceptionPickerLabels(violations, false, 80, colors.Never))

	assert.Equal(t, "[mandatory]  acme/web/prod: required-tags  (aws:s3:Bucket: logs)",
		policyExceptionPickerLabels(violations[:1], true, 0, colors.Never)[0], "no width doesn't cut")
}

func TestPolicyExceptionConsoleURL(t *testing.T) {
	t.Parallel()

	const api = "https://api.pulumi.com"
	assert.Equal(t, "https://app.pulumi.com/acme/web/prod?exception=EXC-7",
		policyExceptionConsoleURL(api, "acme", []apitype.PolicyExceptionMatch{
			match([]string{"p"}, logsURN), match([]string{"p"}, "urn:pulumi:dev::web::aws:s3/bucket:Bucket::logs"),
		}, "EXC-7"), "the link is to the first target's stack")
	assert.Empty(t, policyExceptionConsoleURL("https://pulumi.example.com", "acme", nil, "EXC-7"),
		"a backend without a known console has no link")
}

func TestPolicyExceptionNew_SeveralPacks(t *testing.T) {
	t.Parallel()

	c := twoBuckets()
	c.events["update/12"] = append(c.events["update/12"],
		policyViolation("gcp", "required-labels", "mandatory", logsURN))

	_, _, err := runNew(t, c, "", policyExceptionNewArgs{reason: "x"})
	assert.Equal(t, cmdCmd.ConfigurationError{
		Message: "the violations are from several Policy Packs (aws, gcp); name one as the first argument",
	}, err)

	_, _, err = runNew(t, c, "", policyExceptionNewArgs{policies: []string{"required-labels"}, reason: "x"})
	require.NoError(t, err)
	assert.Equal(t, "gcp", c.gotCreated[0].PolicyPack, "the only Policy Pack is inferred")

	assert.Equal(t, "acme", c.gotOrgFlag, "the organization defaults to the stack's")

	_, _, err = runNew(t, c, "", policyExceptionNewArgs{policyPack: "other-org/aws", reason: "x"})
	require.NoError(t, err)
	assert.Equal(t, "aws", c.gotCreated[1].PolicyPack)
	assert.Equal(t, "other-org", c.gotOrgFlag)
}

func TestPolicyExceptionNew_File(t *testing.T) {
	t.Parallel()

	for name, input := range map[string]string{
		"json": `{"reason": "LOG-31", "match": [{"policies": ["required-tags"], "targets": ["` + logsURN + `"]}]}`,
		"yaml": "reason: LOG-31\nmatch:\n  - policies: [required-tags]\n    targets: [" + logsURN + "]\n",
		"v2 config": "version: 2\nexceptions:\n  EXC-?:\n    reason: LOG-31\n    match:\n" +
			"      - policies: [required-tags]\n        targets: [" + logsURN + "]\n",
		"dry run output": `{"runs": [], "exception": {"policyPack": "aws", "reason": "LOG-31", ` +
			`"match": [{"policies": ["required-tags"], "targets": ["` + logsURN + `"]}]}}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			c := &mockPolicyExceptionClient{}
			_, _, err := runNew(t, c, input, policyExceptionNewArgs{policyPack: "aws", file: "-"})
			require.NoError(t, err)
			assert.Equal(t, []apitype.CreatePolicyExceptionRequest{{
				PolicyPack: "aws",
				PolicyException: apitype.PolicyException{
					Reason: "LOG-31",
					Match:  []apitype.PolicyExceptionMatch{match([]string{"required-tags"}, logsURN)},
				},
			}}, c.gotCreated)
			assert.Empty(t, c.gotReads, "a file doesn't read the run")
		})
	}
}

func TestPolicyExceptionNew_Errors(t *testing.T) {
	t.Parallel()

	file := `{"reason": "x", "match": [{"policies": ["p"], "targets": ["` + logsURN + `"]}]}`
	for name, tc := range map[string]struct {
		stdin string
		args  policyExceptionNewArgs
		want  string
	}{
		"no reason": {
			args: policyExceptionNewArgs{policyPack: "aws"},
			want: "--reason is required when not running interactively",
		},
		"update and preview": {
			args: policyExceptionNewArgs{update: latestRun, preview: latestRun},
			want: "--update and --preview can't be used together",
		},
		"bad pack": {
			args: policyExceptionNewArgs{policyPack: "a/b/c"},
			want: `invalid Policy Pack "a/b/c": expected [<org>/]<policy-pack>`,
		},
		"file without pack": {
			stdin: file, args: policyExceptionNewArgs{file: "-"},
			want: "a Policy Pack is required with --file",
		},
		"file and filters": {
			stdin: file, args: policyExceptionNewArgs{policyPack: "aws", file: "-", policies: []string{"p"}},
			want: "--file can't be used with --stack, --update, --preview, --policy or --target",
		},
		"no entries": {
			stdin: `{"reason": "x"}`, args: policyExceptionNewArgs{policyPack: "aws", file: "-"},
			want: "the exception has no match entries",
		},
		"empty entry": {
			stdin: `{"reason": "x", "match": [{"policies": ["p"]}]}`,
			args:  policyExceptionNewArgs{policyPack: "aws", file: "-"},
			want:  "match entry 1 needs at least one policy and one target",
		},
		"bad urn": {
			stdin: `{"reason": "x", "match": [{"policies": ["p"], "targets": ["logs"]}]}`,
			args:  policyExceptionNewArgs{policyPack: "aws", file: "-"},
			want:  `match entry 1: invalid target "logs": invalid URN "logs"`,
		},
		"two exceptions": {
			stdin: "version: 2\nexceptions:\n  a: {}\n  b: {}\n",
			args:  policyExceptionNewArgs{policyPack: "aws", file: "-"},
			want:  "the file has more than one exception",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			c := twoBuckets()
			_, _, err := runNew(t, c, tc.stdin, tc.args)
			var configErr cmdCmd.ConfigurationError
			require.ErrorAs(t, err, &configErr)
			assert.Equal(t, tc.want, configErr.Message)
			assert.Empty(t, c.gotCreated)
		})
	}

	c := &mockPolicyExceptionClient{err: &apitype.ErrorResponse{
		Code: http.StatusForbidden, Message: "policy exceptions are disabled for this organization",
	}}
	_, _, err := runNew(t, c, file, policyExceptionNewArgs{policyPack: "aws", file: "-"})
	assert.EqualError(t, err, "[403] policy exceptions are disabled for this organization")
	assert.Empty(t, c.gotDrafts, "only a 409 falls back to a draft")
	assert.Empty(t, c.gotSubmitted)
}

func TestPolicyExceptionNew_RequiresApproval(t *testing.T) {
	t.Parallel()

	args := policyExceptionNewArgs{policyPack: "aws", reason: "LOG-31", policies: []string{"s3-no-public-read"}}
	want := apitype.CreatePolicyExceptionRequest{
		PolicyPack: "aws",
		PolicyException: apitype.PolicyException{
			Reason: "LOG-31",
			Match:  []apitype.PolicyExceptionMatch{match([]string{"s3-no-public-read"}, cdnLogsURN)},
		},
	}
	requested := "Requested policy exception: http://localhost:3000/acme/web/prod?exception=cr-1\n"

	c := twoBuckets()
	c.gated, c.crStatus = true, "pending"
	stdout, stderr, err := runNew(t, c, "", args)
	require.NoError(t, err)
	assert.Empty(t, stdout, "there's no exception ID until the change request is approved")
	assert.Equal(t, requested+"Exception will be applied after the request is approved.\n", stderr)
	assert.Equal(t, []apitype.CreatePolicyExceptionRequest{want}, c.gotCreated)
	assert.Equal(t, []apitype.CreatePolicyExceptionRequest{want}, c.gotDrafts)
	assert.Equal(t, []string{"cr-1: LOG-31"}, c.gotSubmitted)

	jsonArgs := args
	jsonArgs.outputFormat = defaultPolicyExceptionNewOutputFormat()
	require.NoError(t, jsonArgs.outputFormat.Set("json"))
	var buf, errBuf bytes.Buffer
	c = twoBuckets()
	c.gated, c.crStatus = true, "pending"
	require.NoError(t, runPolicyExceptionNew(t.Context(), &buf, &errBuf, nil, stubPolicyExceptionFactory(c, "acme"),
		stubStackResolver, false, jsonArgs))
	assert.Empty(t, errBuf.String())
	assert.JSONEq(t, `{
		"organizationName": "acme",
		"runs": [{"stack": "acme/web/prod", "kind": "update", "id": "12"}],
		"violations": [
			{"stack": "acme/web/prod", "policyPack": "aws", "policy": "s3-no-public-read", "level": "mandatory", "urn": "`+
		cdnLogsURN+`"}
		],
		"exception": {
			"policyPack": "aws",
			"reason": "LOG-31",
			"match": [{"policies": ["s3-no-public-read"], "targets": ["`+cdnLogsURN+`"]}]
		},
		"changeRequestId": "cr-1",
		"changeRequestStatus": "pending",
		"url": "http://localhost:3000/acme/web/prod?exception=cr-1"
	}`, buf.String())

	// A change request whose approvals are met on submission is applied then, so the exception exists.
	c = twoBuckets()
	c.gated, c.crStatus = true, changeRequestApplied
	stdout, stderr, err = runNew(t, c, "", args)
	require.NoError(t, err)
	assert.Empty(t, stdout)
	assert.Equal(t, requested+"Policy exception created.\n", stderr)
	assert.Empty(t, c.gotListOpts, "the exception isn't looked up")

	// The same request from the same author returns their open change request, which isn't submitted again.
	c = twoBuckets()
	c.gated, c.crOpen = true, "pending"
	stdout, stderr, err = runNew(t, c, "", args)
	require.NoError(t, err)
	assert.Empty(t, stdout)
	assert.Equal(t, "Updated policy exception request: http://localhost:3000/acme/web/prod?exception=cr-1\n"+
		"Exception will be applied after the request is approved.\n", stderr)
	assert.Empty(t, c.gotSubmitted)

	// Without approval, the exception is created directly.
	c = twoBuckets()
	_, stderr, err = runNew(t, c, "", args)
	require.NoError(t, err)
	assert.Equal(t, "Created policy exception EXC-7: http://localhost:3000/acme/web/prod?exception=EXC-7\n", stderr)
	assert.Empty(t, c.gotDrafts)
	assert.Empty(t, c.gotSubmitted)

	// Other errors don't fall back to a change request.
	c = twoBuckets()
	c.createErr = &apitype.ErrorResponse{Code: http.StatusBadRequest, Message: "bad"}
	_, _, err = runNew(t, c, "", args)
	assert.EqualError(t, err, "[400] bad")
	assert.Empty(t, c.gotDrafts)

	// Nor does the 409 of an organization without the feature.
	c = twoBuckets()
	c.createErr = &apitype.ErrorResponse{Code: http.StatusConflict, Message: `"acme" does not support policy-exceptions.`}
	_, _, err = runNew(t, c, "", args)
	assert.EqualError(t, err, `[409] "acme" does not support policy-exceptions.`)
	assert.Empty(t, c.gotDrafts)
}

func TestPolicyExceptionNew_Draft(t *testing.T) {
	t.Parallel()

	reason := strings.Repeat("é", 1500)
	c := twoBuckets()
	c.crStatus = "pending"
	stdout, stderr, err := runNew(t, c, "", policyExceptionNewArgs{
		policyPack: "aws", reason: reason, policies: []string{"s3-no-public-read"}, draft: true,
	})
	require.NoError(t, err)
	assert.Empty(t, c.gotCreated, "--draft doesn't try to create the exception directly")
	require.Len(t, c.gotDrafts, 1)
	assert.Equal(t, reason, c.gotDrafts[0].Reason)
	assert.Equal(t, []string{"cr-1: " + strings.Repeat("é", 1000)}, c.gotSubmitted,
		"the description is cut to the change request's limit")
	assert.Empty(t, stdout)
	assert.Equal(t, "Requested policy exception: http://localhost:3000/acme/web/prod?exception=cr-1\n"+
		"Exception will be applied after the request is approved.\n", stderr)

	// Without an approval gate, a draft's change request is ready on submission and waits to be applied.
	c = twoBuckets()
	c.crStatus = changeRequestReady
	_, stderr, err = runNew(t, c, "", policyExceptionNewArgs{
		policyPack: "aws", reason: "x", policies: []string{"s3-no-public-read"}, draft: true,
	})
	require.NoError(t, err)
	assert.Equal(t, "Requested policy exception: http://localhost:3000/acme/web/prod?exception=cr-1\n"+
		"Apply the request to create the exception.\n", stderr)

	c = twoBuckets()
	c.submitErr = &apitype.ErrorResponse{Code: http.StatusBadRequest, Message: "bad"}
	_, _, err = runNew(t, c, "", policyExceptionNewArgs{
		policyPack: "aws", reason: "x", policies: []string{"s3-no-public-read"}, draft: true,
	})
	assert.EqualError(t, err, "submitting change request cr-1: [400] bad")
}
