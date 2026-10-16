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
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

// fakeExceptionClient serves canned runs, events, exceptions and requests, and records what it's asked to change.
type fakeExceptionClient struct {
	updates  []apitype.UpdateInfo
	previews []apitype.StackPreview
	// events are keyed by "<kind>/<id>".
	events   map[string][]apitype.EngineEvent
	active   []apitype.PolicyExceptionInfo
	requests []apitype.ChangeRequest
	drafts   map[string]apitype.PolicyExceptionDraft
	usage    []apitype.PolicyExceptionMatchUsage

	createErr error
	getErr    error
	removeErr error

	created   []apitype.CreatePolicyExceptionRequest
	drafted   []apitype.CreatePolicyExceptionRequest
	submitted []string
	removed   []string
	queries   []client.ListPolicyExceptionsQuery
	reads     []string
}

func (f *fakeExceptionClient) CreatePolicyException(
	_ context.Context, _ string, req apitype.CreatePolicyExceptionRequest,
) (apitype.PolicyExceptionInfo, error) {
	if f.createErr != nil {
		return apitype.PolicyExceptionInfo{}, f.createErr
	}
	f.created = append(f.created, req)
	return apitype.PolicyExceptionInfo{ID: "EXC-7", PolicyPack: req.PolicyPack, Reason: req.Reason, Match: req.Match}, nil
}

func (f *fakeExceptionClient) ListPolicyExceptions(
	_ context.Context, _ string, query client.ListPolicyExceptionsQuery,
) ([]apitype.PolicyExceptionInfo, error) {
	f.queries = append(f.queries, query)
	return f.active, nil
}

func (f *fakeExceptionClient) GetPolicyException(_ context.Context, _, id string) (apitype.PolicyExceptionInfo, error) {
	if f.getErr != nil {
		return apitype.PolicyExceptionInfo{}, f.getErr
	}
	return apitype.PolicyExceptionInfo{
		ID: id, PolicyPack: "aws", Status: "active", Reason: "LOG-31",
		Match: []apitype.PolicyExceptionMatch{{Policies: []string{"required-tags"}, Targets: []string{logsURN}}},
	}, nil
}

func (f *fakeExceptionClient) ListPolicyExceptionMatchUsage(
	context.Context, string, string,
) ([]apitype.PolicyExceptionMatchUsage, error) {
	return f.usage, nil
}

func (f *fakeExceptionClient) RemovePolicyException(
	_ context.Context, _, id string,
) (apitype.PolicyExceptionInfo, error) {
	f.removed = append(f.removed, id)
	return apitype.PolicyExceptionInfo{ID: id, Status: "removed"}, f.removeErr
}

func (f *fakeExceptionClient) CreatePolicyExceptionDraft(
	_ context.Context, _ string, req apitype.CreatePolicyExceptionRequest,
) (apitype.PolicyExceptionDraft, error) {
	f.drafted = append(f.drafted, req)
	return apitype.PolicyExceptionDraft{ChangeRequestID: "cr-1"}, nil
}

func (f *fakeExceptionClient) GetPolicyExceptionDraft(
	_ context.Context, _, id string,
) (apitype.PolicyExceptionDraft, error) {
	return f.drafts[id], nil
}

func (f *fakeExceptionClient) ListChangeRequests(context.Context, string, string) ([]apitype.ChangeRequest, error) {
	return f.requests, nil
}

func (f *fakeExceptionClient) SubmitChangeRequest(_ context.Context, _, id, description string) error {
	f.submitted = append(f.submitted, id+": "+description)
	return nil
}

func (f *fakeExceptionClient) GetStackUpdates(
	context.Context, client.StackIdentifier, int, int,
) ([]apitype.UpdateInfo, error) {
	return f.updates, nil
}

func (f *fakeExceptionClient) GetLatestStackPreviews(
	context.Context, client.StackIdentifier,
) ([]apitype.StackPreview, error) {
	return f.previews, nil
}

func (f *fakeExceptionClient) GetUpdateEngineEvents(
	_ context.Context, update client.UpdateIdentifier, opts client.GetUpdateEngineEventsOptions,
) (apitype.GetUpdateEventsResponse, error) {
	key := string(update.UpdateKind) + "/" + update.UpdateID
	f.reads = append(f.reads, key+" "+strings.Join(opts.EventTypes, ","))
	return apitype.GetUpdateEventsResponse{Events: f.events[key]}, nil
}

const (
	logsURN    = "urn:pulumi:prod::web::aws:s3/bucket:Bucket::logs"
	cdnLogsURN = "urn:pulumi:prod::web::aws:s3/bucket:Bucket::cdn-logs"
)

func violation(pack, policy, level, urn string, exceptions ...string) apitype.EngineEvent {
	p := &apitype.PolicyEvent{PolicyPackName: pack, PolicyName: policy, EnforcementLevel: level, ResourceURN: urn}
	for _, id := range exceptions {
		p.Exceptions = append(p.Exceptions, apitype.PolicyEventException{ID: id})
	}
	return apitype.EngineEvent{PolicyEvent: p}
}

func testStack(name string) client.StackIdentifier {
	return client.StackIdentifier{Owner: "acme", Project: "web", Stack: tokens.MustParseStackName(name)}
}

func testExceptionEnv(c *fakeExceptionClient, stacks ...client.StackIdentifier) policyExceptionEnv {
	if len(stacks) == 0 {
		stacks = []client.StackIdentifier{testStack("prod")}
	}
	return policyExceptionEnv{
		client:     c,
		org:        "acme",
		consoleURL: func(paths ...string) string { return "https://app.pulumi.com/" + strings.Join(paths, "/") },
		stacks: func(context.Context, []string) ([]client.StackIdentifier, error) {
			return stacks, nil
		},
	}
}

// runExceptionCmd runs `pulumi policy exception <args>` non-interactively.
func runExceptionCmd(
	t *testing.T, env policyExceptionEnv, stdin string, args ...string,
) (string, string, int) {
	t.Helper()
	cmd := newPolicyExceptionCmdWith(func(context.Context) (policyExceptionEnv, error) { return env, nil })
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetIn(strings.NewReader(stdin))
	cmd.SetArgs(args)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	err := cmd.ExecuteContext(t.Context())
	if err != nil {
		stderr.WriteString(err.Error())
	}
	return stdout.String(), stderr.String(), cmdCmd.ExitCodeFor(err)
}

func TestPolicyExceptionList(t *testing.T) {
	t.Parallel()
	c := &fakeExceptionClient{active: []apitype.PolicyExceptionInfo{{
		ID: "EXC-1", PolicyPack: "aws", Status: "active", Reason: "LOG-31",
		Match: []apitype.PolicyExceptionMatch{{Policies: []string{"a", "b", "c"}, Targets: []string{logsURN, cdnLogsURN}}},
	}}}

	stdout, _, code := runExceptionCmd(t, testExceptionEnv(c), "",
		"list", "aws", "--stack", "web/prod", "--policy", "a", "--status", "removed", "--output", "json")
	require.Equal(t, 0, code)
	assert.Equal(t, []client.ListPolicyExceptionsQuery{{
		PolicyPack: "aws", Policy: "a", Stack: "web/prod", Status: []string{"removed"},
	}}, c.queries)
	var resp apitype.ListPolicyExceptionsResponse
	require.NoError(t, json.Unmarshal([]byte(stdout), &resp))
	assert.Equal(t, c.active, resp.Exceptions)

	stdout, _, code = runExceptionCmd(t, testExceptionEnv(c), "", "list")
	require.Equal(t, 0, code)
	assert.Equal(t, []string{"active"}, c.queries[1].Status)
	assert.Contains(t, stdout, "EXC-1")
	assert.Contains(t, stdout, "a, b, +1")
}

func TestPolicyExceptionGet(t *testing.T) {
	t.Parallel()
	c := &fakeExceptionClient{usage: []apitype.PolicyExceptionMatchUsage{
		{Policy: "required-tags", URN: logsURN, LastUpdateVersion: new(12)},
	}}

	stdout, _, code := runExceptionCmd(t, testExceptionEnv(c), "", "get", "EXC-3")
	require.Equal(t, 0, code)
	assert.Contains(t, stdout, "EXC-3:\n")
	assert.Contains(t, stdout, "reason: LOG-31")
	assert.Contains(t, stdout, logsURN+"  -          12")
	assert.Contains(t, stdout, "https://app.pulumi.com/acme/insights/policy/exceptions/EXC-3")

	c.getErr = &apitype.ErrorResponse{Code: http.StatusNotFound}
	_, stderr, code := runExceptionCmd(t, testExceptionEnv(c), "", "get", "EXC-9")
	assert.Equal(t, 1, code)
	assert.Contains(t, stderr, "policy exception EXC-9 not found in acme")
}

func TestPolicyExceptionRemove(t *testing.T) {
	t.Parallel()
	c := &fakeExceptionClient{}

	_, _, code := runExceptionCmd(t, testExceptionEnv(c), "", "remove", "EXC-3")
	assert.Equal(t, 2, code, "not interactive and no --force")
	assert.Empty(t, c.removed)

	stdout, _, code := runExceptionCmd(t, testExceptionEnv(c), "", "remove", "EXC-3", "--force")
	require.Equal(t, 0, code)
	assert.Equal(t, "EXC-3\n", stdout)
	assert.Equal(t, []string{"EXC-3"}, c.removed)

	c.removeErr = &apitype.ErrorResponse{Code: http.StatusConflict, Message: "EXC-3 is already removed"}
	_, stderr, code := runExceptionCmd(t, testExceptionEnv(c), "", "remove", "EXC-3", "--yes")
	assert.Equal(t, 0, code)
	assert.Contains(t, stderr, "already removed")
}
