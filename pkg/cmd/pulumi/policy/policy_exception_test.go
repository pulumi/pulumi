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
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	esc_client "github.com/pulumi/pulumi/pkg/v3/cmd/esc/cli/client"
	cmdCmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// mockPolicyExceptionClient serves pages of exceptions in order and one exception, a stack's runs and their events
// (keyed by `<kind>/<id>`), and records what it's asked for.
type mockPolicyExceptionClient struct {
	pages     []apitype.ListPolicyExceptionsResponse
	exception apitype.PolicyExceptionInfo
	err       error
	updates   []apitype.UpdateInfo
	previews  []apitype.StackPreview
	events    map[string][]apitype.EngineEvent
	// createErr fails creates, but not drafts; gated returns a 409 for them, as when the org requires approval.
	createErr error
	gated     bool
	// existed makes creates return an existing exception, as for a repeated request.
	existed   bool
	submitErr error
	// crStatus is the status of the change request a draft creates, after it's submitted. crOpen is its status
	// before, when the draft returned an open change request; it's a draft otherwise.
	crStatus string
	crOpen   string

	gotListOpts  []client.ListPolicyExceptionsOptions
	gotOrgFlag   string
	gotOrg       string
	gotRemoved   []string
	gotCreated   []apitype.CreatePolicyExceptionRequest
	gotDrafts    []apitype.CreatePolicyExceptionRequest
	gotSubmitted []string
	gotReads     []string
}

func (m *mockPolicyExceptionClient) CreatePolicyException(
	_ context.Context, org string, req apitype.CreatePolicyExceptionRequest,
) (apitype.PolicyExceptionInfo, bool, error) {
	m.gotOrg = org
	m.gotCreated = append(m.gotCreated, req)
	if m.gated {
		return apitype.PolicyExceptionInfo{}, false,
			&apitype.ErrorResponse{Code: http.StatusConflict, Message: "requires approval"}
	}
	return apitype.PolicyExceptionInfo{ID: "EXC-7"}, m.existed, cmp.Or(m.createErr, m.err)
}

func (m *mockPolicyExceptionClient) CreatePolicyExceptionDraft(
	_ context.Context, org string, req apitype.CreatePolicyExceptionRequest,
) (apitype.PolicyExceptionDraft, error) {
	m.gotOrg = org
	m.gotDrafts = append(m.gotDrafts, req)
	return apitype.PolicyExceptionDraft{CreatePolicyExceptionRequest: req, ChangeRequestID: "cr-1"}, m.err
}

func (m *mockPolicyExceptionClient) SubmitChangeRequest(_ context.Context, org, id string, description *string) error {
	m.gotOrg = org
	m.gotSubmitted = append(m.gotSubmitted, id+": "+*description)
	return cmp.Or(m.submitErr, m.err)
}

func (m *mockPolicyExceptionClient) GetChangeRequest(
	_ context.Context, org, id string,
) (*esc_client.GetChangeRequestResponse, error) {
	m.gotOrg = org
	status := cmp.Or(m.crOpen, changeRequestDraft)
	if len(m.gotSubmitted) > 0 {
		status = m.crStatus
	}
	return &esc_client.GetChangeRequestResponse{ChangeRequest: esc_client.ChangeRequest{ID: id, Status: status}}, m.err
}

func (m *mockPolicyExceptionClient) URL() string {
	return "http://localhost:8080"
}

func (m *mockPolicyExceptionClient) GetStackUpdates(
	context.Context, client.StackIdentifier, int, int,
) ([]apitype.UpdateInfo, error) {
	return m.updates, nil
}

func (m *mockPolicyExceptionClient) GetLatestStackPreviews(
	context.Context, client.StackIdentifier,
) ([]apitype.StackPreview, error) {
	return m.previews, nil
}

func (m *mockPolicyExceptionClient) GetUpdateEngineEvents(
	_ context.Context, update client.UpdateIdentifier, opts client.GetUpdateEngineEventsOptions,
) (apitype.GetUpdateEventsResponse, error) {
	m.gotReads = append(m.gotReads, fmt.Sprintf("%s %s/%s %v",
		update.StackIdentifier, update.UpdateKind, update.UpdateID, opts.EventTypes))
	return apitype.GetUpdateEventsResponse{Events: m.events[string(update.UpdateKind)+"/"+update.UpdateID]}, nil
}

func (m *mockPolicyExceptionClient) ListPolicyExceptions(
	_ context.Context, org string, opts client.ListPolicyExceptionsOptions,
) (apitype.ListPolicyExceptionsResponse, error) {
	m.gotOrg = org
	m.gotListOpts = append(m.gotListOpts, opts)
	if m.err != nil {
		return apitype.ListPolicyExceptionsResponse{}, m.err
	}
	if i := len(m.gotListOpts) - 1; i < len(m.pages) {
		return m.pages[i], nil
	}
	return apitype.ListPolicyExceptionsResponse{}, nil
}

func (m *mockPolicyExceptionClient) GetPolicyException(
	_ context.Context, org, _ string,
) (apitype.PolicyExceptionInfo, error) {
	m.gotOrg = org
	return m.exception, m.err
}

func (m *mockPolicyExceptionClient) RemovePolicyException(_ context.Context, org, exceptionID string) error {
	m.gotOrg = org
	m.gotRemoved = append(m.gotRemoved, exceptionID)
	return m.err
}

func stubPolicyExceptionFactory(c *mockPolicyExceptionClient, org string) policyExceptionClientFactory {
	return func(_ context.Context, orgFlag string) (policyExceptionClients, string, error) {
		c.gotOrgFlag = orgFlag
		return policyExceptionClients{c, c}, org, nil
	}
}

const logsURN = "urn:pulumi:prod::web::aws:s3/bucket:Bucket::logs"

func samplePolicyException(id string) apitype.PolicyExceptionInfo {
	return apitype.PolicyExceptionInfo{
		PolicyException: apitype.PolicyException{
			Reason: "Logs are public by design",
			Match: []apitype.PolicyExceptionMatch{
				{Policies: []string{"s3-no-public-read", "required-tags"}, Targets: []string{logsURN}},
				{Policies: []string{"required-tags"}, Targets: []string{logsURN + "-cdn"}},
				{Policies: []string{"s3-no-public-read"}, Targets: []string{logsURN + "-cdn"}},
			},
		},
		ID:         id,
		PolicyPack: "aws-security",
		Status:     "active",
		CreatedBy:  apitype.UserInfo{GitHubLogin: "alice"},
		CreatedAt:  time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC),
	}
}

func TestPolicyExceptionList(t *testing.T) {
	t.Parallel()

	var errBuf bytes.Buffer
	c := &mockPolicyExceptionClient{pages: []apitype.ListPolicyExceptionsResponse{
		{Exceptions: []apitype.PolicyExceptionInfo{samplePolicyException("EXC-2")}, ContinuationToken: "next"},
		{Exceptions: []apitype.PolicyExceptionInfo{samplePolicyException("EXC-1")}},
	}}
	var buf bytes.Buffer
	err := runPolicyExceptionList(t.Context(), &buf, &errBuf, stubPolicyExceptionFactory(c, "acme"),
		policyExceptionListArgs{
			policyPack:   "aws-security",
			policy:       "required-tags",
			stack:        "web/prod",
			statuses:     []string{"active", "removed"},
			outputFormat: defaultPolicyExceptionListOutputFormat(),
		})
	require.NoError(t, err)

	assert.Equal(t, "acme", c.gotOrg)
	assert.Empty(t, c.gotOrgFlag, "the organization defaults to the backend's")
	query := client.ListPolicyExceptionsOptions{
		PolicyPack: "aws-security",
		Policy:     "required-tags",
		Stack:      "web/prod",
		Status:     []string{"active", "removed"},
	}
	next := query
	next.ContinuationToken = "next"
	assert.Equal(t, []client.ListPolicyExceptionsOptions{query, next}, c.gotListOpts)
	want := `┌───────┬──────────────┬────────┬──────────────────────────────────┬─────────┬───────────────────────────┐
│ ID    │ POLICY PACK  │ STATUS │ POLICIES                         │ TARGETS │ REASON                    │
├───────┼──────────────┼────────┼──────────────────────────────────┼─────────┼───────────────────────────┤
│ EXC-2 │ aws-security │ active │ required-tags, s3-no-public-read │       2 │ Logs are public by design │
│ EXC-1 │ aws-security │ active │ required-tags, s3-no-public-read │       2 │ Logs are public by design │
└───────┴──────────────┴────────┴──────────────────────────────────┴─────────┴───────────────────────────┘
`
	assert.Equal(t, want, buf.String())

	c = &mockPolicyExceptionClient{pages: []apitype.ListPolicyExceptionsResponse{{}}}
	buf.Reset()
	err = runPolicyExceptionList(t.Context(), &buf, &errBuf, stubPolicyExceptionFactory(c, "acme"),
		policyExceptionListArgs{outputFormat: defaultPolicyExceptionListOutputFormat()})
	require.NoError(t, err)
	assert.Empty(t, buf.String())
	assert.Equal(t, "No policy exceptions found.\n", errBuf.String())
}

func TestPolicyExceptionList_JSONOutput(t *testing.T) {
	t.Parallel()

	var errBuf bytes.Buffer
	c := &mockPolicyExceptionClient{pages: []apitype.ListPolicyExceptionsResponse{{}}}
	args := policyExceptionListArgs{outputFormat: defaultPolicyExceptionListOutputFormat()}
	require.NoError(t, args.outputFormat.Set("json"))
	var buf bytes.Buffer
	require.NoError(t, runPolicyExceptionList(t.Context(), &buf, &errBuf, stubPolicyExceptionFactory(c, "acme"), args))
	assert.JSONEq(t, `{"exceptions": []}`, buf.String())

	c = &mockPolicyExceptionClient{pages: []apitype.ListPolicyExceptionsResponse{
		{Exceptions: []apitype.PolicyExceptionInfo{samplePolicyException("EXC-1")}},
	}}
	buf.Reset()
	require.NoError(t, runPolicyExceptionList(t.Context(), &buf, &errBuf, stubPolicyExceptionFactory(c, "acme"), args))
	assert.JSONEq(t, `{"exceptions": [{
		"id": "EXC-1",
		"policyPack": "aws-security",
		"status": "active",
		"reason": "Logs are public by design",
		"match": [
			{"policies": ["s3-no-public-read", "required-tags"], "targets": ["`+logsURN+`"]},
			{"policies": ["required-tags"], "targets": ["`+logsURN+`-cdn"]},
			{"policies": ["s3-no-public-read"], "targets": ["`+logsURN+`-cdn"]}
		],
		"createdBy": {"name": "", "githubLogin": "alice", "avatarUrl": ""},
		"createdAt": "2026-10-01T12:00:00Z"
	}]}`, buf.String())
}

func TestPolicyExceptionList_Errors(t *testing.T) {
	t.Parallel()

	c := &mockPolicyExceptionClient{err: &apitype.ErrorResponse{Code: http.StatusNotFound}}
	err := runPolicyExceptionList(t.Context(), &bytes.Buffer{}, &bytes.Buffer{}, stubPolicyExceptionFactory(c, "acme"),
		policyExceptionListArgs{outputFormat: defaultPolicyExceptionListOutputFormat()})
	assert.EqualError(t, err, "policy exceptions aren't available for acme on this Pulumi Cloud")

	c = &mockPolicyExceptionClient{err: &apitype.ErrorResponse{Code: http.StatusInternalServerError, Message: "boom"}}
	err = runPolicyExceptionList(t.Context(), &bytes.Buffer{}, &bytes.Buffer{}, stubPolicyExceptionFactory(c, "acme"),
		policyExceptionListArgs{outputFormat: defaultPolicyExceptionListOutputFormat()})
	assert.EqualError(t, err, "[500] boom")

	for _, stack := range []string{"prod", "/prod", "web/", "acme/web/prod"} {
		c = &mockPolicyExceptionClient{}
		err = runPolicyExceptionList(t.Context(), &bytes.Buffer{}, &bytes.Buffer{}, stubPolicyExceptionFactory(c, "acme"),
			policyExceptionListArgs{stack: stack, outputFormat: defaultPolicyExceptionListOutputFormat()})
		assert.Equal(t, cmdCmd.ConfigurationError{
			Message: fmt.Sprintf("--stack must be <project>/<stack>, got %q", stack),
		}, err)
		assert.Empty(t, c.gotListOpts, stack)
	}
}

func TestPolicyExceptionGet(t *testing.T) {
	t.Parallel()

	removed := samplePolicyException("EXC-3")
	removed.Status = "removed"
	removed.RemovedBy = apitype.UserInfo{GitHubLogin: "bob"}
	removed.RemovedAt = time.Date(2026, 10, 2, 9, 30, 0, 0, time.UTC)
	removed.ChangeRequestID = "cr-1"
	removed.Match = removed.Match[:1]
	c := &mockPolicyExceptionClient{exception: removed}
	var buf bytes.Buffer
	err := runPolicyExceptionGet(t.Context(), &buf, stubPolicyExceptionFactory(c, "acme"), "EXC-3",
		policyExceptionGetArgs{org: "other-org", outputFormat: defaultPolicyExceptionGetOutputFormat()})
	require.NoError(t, err)
	assert.Equal(t, "other-org", c.gotOrgFlag)
	assert.Equal(t, `ID:            EXC-3
Policy pack:   aws-security
Status:        removed
Requested in:  change request cr-1
Created:       2026-10-01T12:00:00Z by alice
Removed:       2026-10-02T09:30:00Z by bob

Policy Pack exceptions configuration:
{
  "reason": "Logs are public by design",
  "match": [
    {
      "policies": [
        "s3-no-public-read",
        "required-tags"
      ],
      "targets": [
        "urn:pulumi:prod::web::aws:s3/bucket:Bucket::logs"
      ]
    }
  ]
}
`, buf.String(), "a removed exception shows what it exempted")

	deletedUser := samplePolicyException("EXC-4")
	deletedUser.CreatedBy = apitype.UserInfo{}
	c = &mockPolicyExceptionClient{exception: deletedUser}
	buf.Reset()
	err = runPolicyExceptionGet(t.Context(), &buf, stubPolicyExceptionFactory(c, "acme"), "EXC-4",
		policyExceptionGetArgs{outputFormat: defaultPolicyExceptionGetOutputFormat()})
	require.NoError(t, err)
	assert.Equal(t, `ID:            EXC-4
Policy pack:   aws-security
Status:        active
Created:       2026-10-01T12:00:00Z

Policy Pack exceptions configuration:
{
  "reason": "Logs are public by design",
  "match": [
    {
      "policies": [
        "s3-no-public-read",
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
        "urn:pulumi:prod::web::aws:s3/bucket:Bucket::logs-cdn"
      ]
    },
    {
      "policies": [
        "s3-no-public-read"
      ],
      "targets": [
        "urn:pulumi:prod::web::aws:s3/bucket:Bucket::logs-cdn"
      ]
    }
  ]
}
`, buf.String())
}

func TestPolicyExceptionGet_JSONOutput(t *testing.T) {
	t.Parallel()

	removed := samplePolicyException("EXC-3")
	removed.Status = "removed"
	removed.RemovedAt = time.Date(2026, 10, 2, 9, 30, 0, 0, time.UTC)
	c := &mockPolicyExceptionClient{exception: removed}
	args := policyExceptionGetArgs{outputFormat: defaultPolicyExceptionGetOutputFormat()}
	require.NoError(t, args.outputFormat.Set("json"))
	var buf bytes.Buffer
	require.NoError(t, runPolicyExceptionGet(t.Context(), &buf, stubPolicyExceptionFactory(c, "acme"), "EXC-3", args))
	assert.JSONEq(t, `{
		"id": "EXC-3",
		"policyPack": "aws-security",
		"status": "removed",
		"reason": "Logs are public by design",
		"match": [
			{"policies": ["s3-no-public-read", "required-tags"], "targets": ["`+logsURN+`"]},
			{"policies": ["required-tags"], "targets": ["`+logsURN+`-cdn"]},
			{"policies": ["s3-no-public-read"], "targets": ["`+logsURN+`-cdn"]}
		],
		"createdBy": {"name": "", "githubLogin": "alice", "avatarUrl": ""},
		"createdAt": "2026-10-01T12:00:00Z",
		"removedAt": "2026-10-02T09:30:00Z"
	}`, buf.String())
}

func TestPolicyExceptionGet_Errors(t *testing.T) {
	t.Parallel()

	args := policyExceptionGetArgs{outputFormat: defaultPolicyExceptionGetOutputFormat()}
	c := &mockPolicyExceptionClient{err: &apitype.ErrorResponse{Code: http.StatusNotFound}}
	err := runPolicyExceptionGet(t.Context(), &bytes.Buffer{}, stubPolicyExceptionFactory(c, "acme"), "EXC-9", args)
	assert.EqualError(t, err, "policy exception EXC-9 not found in acme")

	c = &mockPolicyExceptionClient{err: &apitype.ErrorResponse{Code: http.StatusInternalServerError, Message: "boom"}}
	err = runPolicyExceptionGet(t.Context(), &bytes.Buffer{}, stubPolicyExceptionFactory(c, "acme"), "EXC-9", args)
	assert.EqualError(t, err, "[500] boom")

	failing := func(context.Context, string) (policyExceptionClients, string, error) {
		return policyExceptionClients{}, "", errors.New("not logged in")
	}
	err = runPolicyExceptionGet(t.Context(), &bytes.Buffer{}, failing, "EXC-9", args)
	assert.EqualError(t, err, "not logged in")
}

func TestPolicyExceptionRemove(t *testing.T) {
	t.Parallel()

	c := &mockPolicyExceptionClient{}
	var stdout, stderr bytes.Buffer
	args := policyExceptionRemoveArgs{outputFormat: defaultPolicyExceptionRemoveOutputFormat()}
	err := runPolicyExceptionRemove(t.Context(), &stdout, &stderr, stubPolicyExceptionFactory(c, "acme"), "EXC-3", args)
	assert.ErrorIs(t, err, backenderr.ErrNonInteractiveRequiresYes)
	assert.Empty(t, c.gotRemoved)

	args.yes = true
	err = runPolicyExceptionRemove(t.Context(), &stdout, &stderr, stubPolicyExceptionFactory(c, "acme"), "EXC-3", args)
	require.NoError(t, err)
	assert.Empty(t, stdout.String())
	assert.Equal(t, "Removed policy exception EXC-3 from organization acme.\n", stderr.String())
	assert.Equal(t, []string{"EXC-3"}, c.gotRemoved)

	jsonArgs := policyExceptionRemoveArgs{
		org: "other-org", yes: true, outputFormat: defaultPolicyExceptionRemoveOutputFormat(),
	}
	require.NoError(t, jsonArgs.outputFormat.Set("json"))
	stderr.Reset()
	err = runPolicyExceptionRemove(t.Context(), &stdout, &stderr, stubPolicyExceptionFactory(c, "acme"), "EXC-4",
		jsonArgs)
	require.NoError(t, err)
	assert.JSONEq(t, `{"organizationName": "acme", "id": "EXC-4"}`, stdout.String())
	assert.Empty(t, stderr.String())
	assert.Equal(t, []string{"EXC-3", "EXC-4"}, c.gotRemoved)
	assert.Equal(t, "other-org", c.gotOrgFlag)

	c = &mockPolicyExceptionClient{err: &apitype.ErrorResponse{Code: http.StatusNotFound}}
	err = runPolicyExceptionRemove(t.Context(), &stdout, &stderr, stubPolicyExceptionFactory(c, "acme"), "EXC-9", args)
	assert.EqualError(t, err, "policy exception EXC-9 not found in acme")

	c = &mockPolicyExceptionClient{
		err: &apitype.ErrorResponse{Code: http.StatusConflict, Message: "EXC-3 is already removed"},
	}
	stdout.Reset()
	stderr.Reset()
	err = runPolicyExceptionRemove(t.Context(), &stdout, &stderr, stubPolicyExceptionFactory(c, "acme"), "EXC-3", args)
	require.NoError(t, err, "removing an exception twice isn't an error")
	assert.Equal(t, "Policy exception EXC-3 is already removed.\n", stderr.String())

	c = &mockPolicyExceptionClient{err: &apitype.ErrorResponse{Code: http.StatusInternalServerError, Message: "boom"}}
	err = runPolicyExceptionRemove(t.Context(), &stdout, &stderr, stubPolicyExceptionFactory(c, "acme"), "EXC-3", args)
	assert.EqualError(t, err, "[500] boom")
}
