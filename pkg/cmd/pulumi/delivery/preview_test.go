// Copyright 2026, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

func validManifest() candidatePreviewManifest {
	return candidatePreviewManifest{
		CandidateID: "cand-1", ShapeVersion: 1, ReleaseID: "rel-1", ChangeRequestID: "cr-1",
		RevisionNumber: 0, WorkflowRunID: "wf-1",
	}
}

func unavailable(urn string) candidatePreviewUnavailableStack {
	return candidatePreviewUnavailableStack{
		URN: urn, Name: "services", Stage: "prod", TargetStack: "org/services/prod",
		Status: "not-previewable", Reason: "input depends on an output no stage has produced yet",
	}
}

func writeManifest(t *testing.T, manifest candidatePreviewManifest) string {
	t.Helper()
	contents, err := json.Marshal(manifest)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "manifest.json")
	require.NoError(t, os.WriteFile(path, contents, 0o600))
	return path
}

// TestReadCandidatePreviewManifest_MixedAccepted proves a manifest with both runnable members and
// unavailable rows parses, and that URNs across the two lists are tracked as one disjoint set.
func TestReadCandidatePreviewManifest_MixedAccepted(t *testing.T) {
	t.Parallel()

	manifest := validManifest()
	manifest.Workspace = "/workspace"
	manifest.Members = []candidatePreviewMember{
		{URN: "urn:member-a", Name: "a", TargetStack: "org/a/prod", Directory: "a"},
	}
	manifest.UnpreviewableStacks = []candidatePreviewUnavailableStack{unavailable("urn:services")}

	parsed, err := readCandidatePreviewManifest(writeManifest(t, manifest))
	require.NoError(t, err)
	require.Len(t, parsed.Members, 1)
	require.Len(t, parsed.UnpreviewableStacks, 1)
}

// TestReadCandidatePreviewManifest_AllUnavailableAccepted proves an empty Members list is valid
// as long as UnpreviewableStacks explains every stack, and that no Workspace is required.
func TestReadCandidatePreviewManifest_AllUnavailableAccepted(t *testing.T) {
	t.Parallel()

	manifest := validManifest()
	manifest.UnpreviewableStacks = []candidatePreviewUnavailableStack{unavailable("urn:services")}

	parsed, err := readCandidatePreviewManifest(writeManifest(t, manifest))
	require.NoError(t, err)
	assert.Empty(t, parsed.Members)
	require.Len(t, parsed.UnpreviewableStacks, 1)
}

// TestReadCandidatePreviewManifest_RejectsEmptyBoth proves a manifest with neither a runnable
// member nor an unavailable disposition is rejected -- there would be nothing to report.
func TestReadCandidatePreviewManifest_RejectsEmptyBoth(t *testing.T) {
	t.Parallel()

	_, err := readCandidatePreviewManifest(writeManifest(t, validManifest()))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no members or unpreviewable stacks")
}

// TestReadCandidatePreviewManifest_RejectsDuplicateURN proves URNs are validated as one set
// across Members and UnpreviewableStacks, not independently per list.
func TestReadCandidatePreviewManifest_RejectsDuplicateURN(t *testing.T) {
	t.Parallel()

	manifest := validManifest()
	manifest.Workspace = "/workspace"
	manifest.Members = []candidatePreviewMember{
		{URN: "urn:dup", Name: "a", TargetStack: "org/a/prod", Directory: "a"},
	}
	manifest.UnpreviewableStacks = []candidatePreviewUnavailableStack{unavailable("urn:dup")}

	_, err := readCandidatePreviewManifest(writeManifest(t, manifest))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate")
}

// TestReadCandidatePreviewManifest_RejectsMissingReason proves an unavailable row without a
// reason is rejected -- the console must always have something to show next to the warning.
func TestReadCandidatePreviewManifest_RejectsMissingReason(t *testing.T) {
	t.Parallel()

	manifest := validManifest()
	stack := unavailable("urn:services")
	stack.Reason = ""
	manifest.UnpreviewableStacks = []candidatePreviewUnavailableStack{stack}

	_, err := readCandidatePreviewManifest(writeManifest(t, manifest))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing a reason")
}

// TestReadCandidatePreviewManifest_RejectsMissingDisplayIdentity proves an unavailable row must
// carry the same Name/Stage identity the wire type requires for a succeeded row -- otherwise the
// service would have to render an unavailable stack with a blank name/stage.
func TestReadCandidatePreviewManifest_RejectsMissingDisplayIdentity(t *testing.T) {
	t.Parallel()

	manifest := validManifest()
	stack := unavailable("urn:services")
	stack.Name = ""
	manifest.UnpreviewableStacks = []candidatePreviewUnavailableStack{stack}

	_, err := readCandidatePreviewManifest(writeManifest(t, manifest))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "incomplete")
}

// TestReadCandidatePreviewManifest_RejectsInvalidStatus proves an unavailable row must carry the
// one defined disposition status.
func TestReadCandidatePreviewManifest_RejectsInvalidStatus(t *testing.T) {
	t.Parallel()

	manifest := validManifest()
	stack := unavailable("urn:services")
	stack.Status = "pending"
	manifest.UnpreviewableStacks = []candidatePreviewUnavailableStack{stack}

	_, err := readCandidatePreviewManifest(writeManifest(t, manifest))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid status")
}

// TestRunCandidatePreview_AllUnavailable proves an all-unavailable manifest never touches the
// filesystem or a subprocess (a bogus, nonexistent Workspace would fail immediately if it did),
// and returns the unavailable rows with a nil plan.
func TestRunCandidatePreview_AllUnavailable(t *testing.T) {
	t.Parallel()

	manifest := validManifest()
	manifest.Workspace = "/does/not/exist"
	manifest.UnpreviewableStacks = []candidatePreviewUnavailableStack{
		unavailable("urn:services"), unavailable("urn:jobs"),
	}

	stacks, plan, events, err := runCandidatePreview(t.Context(), manifest, nil)
	require.NoError(t, err)
	assert.Nil(t, plan)
	assert.JSONEq(t, "[]", string(events))
	require.Len(t, stacks, 2)
	for _, stack := range stacks {
		assert.Equal(t, "not-previewable", stack.Status)
		assert.NotEmpty(t, stack.Reason)
		assert.Equal(t, map[string]int{}, stack.Changes)
	}
}

func planWithResource(urn resource.URN) *deploy.Plan {
	return &deploy.Plan{ResourcePlans: map[resource.URN]*deploy.ResourcePlan{
		urn: {},
	}}
}

func TestAggregateNativePreviewPlan_UnionsPerMemberURNs(t *testing.T) {
	t.Parallel()

	urnA := resource.NewURN("dev", "member-a", "", "pulumi:pulumi:Stack", "member-a-dev")
	urnB := resource.NewURN("dev", "member-b", "", "pulumi:pulumi:Stack", "member-b-dev")

	merged, err := aggregateNativePreviewPlan(
		[]*deploy.Plan{planWithResource(urnA), planWithResource(urnB)}, []string{"a", "b"})
	require.NoError(t, err)
	require.Len(t, merged.ResourcePlans, 2)
	assert.Contains(t, merged.ResourcePlans, urnA)
	assert.Contains(t, merged.ResourcePlans, urnB)
	assert.Nil(t, merged.Config, "aggregate must not merge per-member Plan.Config")
}

func TestAggregateNativePreviewPlan_RejectsDuplicateURN(t *testing.T) {
	t.Parallel()

	urn := resource.NewURN("dev", "same-project", "", "pulumi:pulumi:Stack", "same-project-dev")
	_, err := aggregateNativePreviewPlan(
		[]*deploy.Plan{planWithResource(urn), planWithResource(urn)}, []string{"a", "b"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate resource URN")
}

func TestAggregateNativePreviewEvents_CombinesSummaries(t *testing.T) {
	t.Parallel()

	member := func(creates int, updates int) []apitype.EngineEvent {
		return []apitype.EngineEvent{
			{Sequence: 7, ResourcePreEvent: &apitype.ResourcePreEvent{}},
			{Sequence: 8, SummaryEvent: &apitype.SummaryEvent{
				IsPreview:       true,
				ResourceChanges: map[apitype.OpType]int{apitype.OpCreate: creates, apitype.OpUpdate: updates},
			}},
		}
	}
	combined := aggregateNativePreviewEvents([][]apitype.EngineEvent{member(1, 0), member(2, 1)})

	require.Len(t, combined, 3, "two member events plus exactly one summary")
	last := combined[2].SummaryEvent
	require.NotNil(t, last)
	assert.True(t, last.IsPreview)
	assert.Equal(t, 3, last.ResourceChanges[apitype.OpCreate])
	assert.Equal(t, 1, last.ResourceChanges[apitype.OpUpdate])
	for i, event := range combined {
		assert.Equal(t, i+1, event.Sequence)
	}
}

// fakeSubgroups records subgroup create/close calls, in order, into the shared log.
type fakeSubgroups struct {
	mu        sync.Mutex
	log       *fakeLog
	createErr error
}

func (f *fakeSubgroups) CreateCoherenceWindowSubgroup(
	_ context.Context, org string, windowID string, size int,
) (string, error) {
	if f.createErr != nil {
		return "", f.createErr
	}
	f.log.add(fmt.Sprintf("create %s %s %d", org, windowID, size))
	return fmt.Sprintf("sub-%d", f.log.count("create ")), nil
}

func (f *fakeSubgroups) CloseCoherenceWindowSubgroup(
	_ context.Context, org string, windowID string, subgroupID string,
) error {
	f.log.add(fmt.Sprintf("close %s %s %s", org, windowID, subgroupID))
	return nil
}

type fakeLog struct {
	mu    sync.Mutex
	lines []string
}

func (l *fakeLog) add(line string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lines = append(l.lines, line)
}

func (l *fakeLog) count(prefix string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	n := 0
	for _, line := range l.lines {
		if strings.HasPrefix(line, prefix) {
			n++
		}
	}
	return n
}

func (l *fakeLog) all() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.Clone(l.lines)
}

// installFakeCLI writes a stand-in for the pulumi CLI binary. It appends every invocation to a log file
// (which the test merges with the subgroup log), and for `preview` writes a plan with one
// resource named after the stack, an event log and a --json digest. A stack whose name contains
// "fail" exits 1 with a message that includes the secret value.
func installFakeCLI(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	logPath := filepath.Join(dir, "calls.log")
	script := `#!/bin/sh
echo "$*" >> "` + logPath + `"
[ "$1" = preview ] || exit 0
while [ $# -gt 0 ]; do
  case "$1" in
    --save-plan) plan=$2; shift;;
    --event-log) events=$2; shift;;
    --stack) stack=$2; shift;;
  esac
  shift
done
case "$stack" in *fail*) echo "boom hunter2" >&2; exit 1;; esac
name=$(echo "$stack" | cut -d/ -f2)
cat > "$plan" <<EOF2
{"manifest":{"time":"2026-01-01T00:00:00Z","magic":"","version":""},"resourcePlans":{"urn:pulumi:$name::proj::pulumi:pulumi:Stack::proj-$name":{}}}
EOF2
cat > "$events" <<EOF2
{"sequence":1,"timestamp":1,"resourcePreEvent":{"metadata":{"op":"create","urn":"u","type":"t"}}}
{"sequence":2,"timestamp":2,"summaryEvent":{"maybeCorrupt":false,"durationSeconds":0,"resourceChanges":{"create":1},"PolicyPacks":{},"isPreview":true,"result":"succeeded"}}
EOF2
echo '{"changeSummary":{"create":1}}'
`
	path := filepath.Join(dir, "pulumi")
	require.NoError(t, os.WriteFile(path, []byte(script), 0o755))
	prev := executable
	executable = func() (string, error) { return path, nil }
	t.Cleanup(func() { executable = prev })
	return logPath
}

func groupedManifest(t *testing.T, members ...candidatePreviewMember) candidatePreviewManifest {
	t.Helper()
	workspace := t.TempDir()
	for _, member := range members {
		require.NoError(t, os.MkdirAll(filepath.Join(workspace, member.Directory), 0o755))
	}
	manifest := validManifest()
	manifest.Workspace, manifest.Organization, manifest.Members = workspace, "acme", members
	return manifest
}

func calls(t *testing.T, logPath string) []string {
	t.Helper()
	contents, err := os.ReadFile(logPath)
	if err != nil {
		return nil
	}
	return strings.Split(strings.TrimSpace(string(contents)), "\n")
}

// TestRunCandidatePreview_GroupsRunInOrderWithExactSubgroups proves groups run in ascending
// groupOrder (not manifest order), each in a fresh window with a subgroup sized to the group,
// closed once the group's members are done, and that config, secrets, ESC environments and
// environment variables are applied before the group's previews.
//
//nolint:paralleltest // replaces the CLI binary lookup
func TestRunCandidatePreview_GroupsRunInOrderWithExactSubgroups(t *testing.T) {
	logPath := installFakeCLI(t)
	secretEnv, err := json.Marshal(candidatePreviewEnvironment{
		Env:            map[string]candidatePreviewConfigValue{"TOKEN": {Value: "tok", Secret: true}},
		Environments:   []string{"acme/base"},
		PreRunCommands: []string{"echo prerun >> " + logPath},
	})
	require.NoError(t, err)
	manifest := groupedManifest(t,
		candidatePreviewMember{URN: "urn:app", Name: "app", Stage: "prod", TargetStack: "acme/app/prod",
			Directory: "app", Group: "g2", GroupOrder: 1},
		candidatePreviewMember{URN: "urn:net", Name: "net", Stage: "prod", TargetStack: "acme/net/prod",
			Directory: "net", Group: "g1", GroupOrder: 0,
			Config: map[string]candidatePreviewConfigValue{
				"proj:plain":  {Value: "v1"},
				"proj:secret": {Value: "hunter2", Secret: true},
			}, Environment: secretEnv},
		candidatePreviewMember{URN: "urn:db", Name: "db", Stage: "prod", TargetStack: "acme/db/prod",
			Directory: "db", Group: "g1", GroupOrder: 0},
	)
	manifest.UnpreviewableStacks = []candidatePreviewUnavailableStack{unavailable("urn:later")}
	log := &fakeLog{}
	api := &fakeSubgroups{log: log}

	stacks, plan, events, err := runCandidatePreview(t.Context(), manifest, api)
	require.NoError(t, err)

	got := log.all()
	require.Len(t, got, 4)
	assert.Regexp(t, `^create acme [0-9a-f-]{36} 2$`, got[0], "g1 has two members")
	assert.Regexp(t, `^close acme [0-9a-f-]{36} sub-1$`, got[1])
	assert.Regexp(t, `^create acme [0-9a-f-]{36} 1$`, got[2], "g2 has one member")
	assert.Regexp(t, `^close acme [0-9a-f-]{36} sub-2$`, got[3])
	assert.NotEqual(t, strings.Fields(got[0])[2], strings.Fields(got[2])[2], "fresh window per group")

	all := strings.Join(calls(t, logPath), "\n")
	assert.Contains(t, all, "config env add acme/base --yes --non-interactive --stack acme/net/prod")
	assert.Contains(t, all, "config set --non-interactive --stack acme/net/prod")
	assert.Contains(t, all, "-- proj:plain v1")
	assert.Contains(t, all, "config set --secret --non-interactive --stack acme/net/prod")
	assert.Contains(t, all, "-- proj:secret hunter2")
	assert.Contains(t, all, "prerun")
	assert.Contains(t, all, "preview --non-interactive --coherence-window sub-1 --stack acme/net/prod")
	assert.Contains(t, all, "preview --non-interactive --coherence-window sub-1 --stack acme/db/prod")
	assert.Contains(t, all, "preview --non-interactive --coherence-window sub-2 --stack acme/app/prod")
	assert.Less(t, strings.Index(all, "coherence-window sub-1"), strings.Index(all, "coherence-window sub-2"))

	require.Len(t, stacks, 4, "members in manifest order, then unavailable rows")
	assert.Equal(t, []string{"urn:app", "urn:net", "urn:db", "urn:later"},
		[]string{stacks[0].URN, stacks[1].URN, stacks[2].URN, stacks[3].URN})
	assert.Equal(t, map[string]int{"create": 1}, stacks[0].Changes)
	assert.Equal(t, "not-previewable", stacks[3].Status)

	var serialized apitype.DeploymentPlanV1
	require.NoError(t, json.Unmarshal(plan, &serialized))
	assert.Len(t, serialized.ResourcePlans, 3)
	var aggregated []apitype.EngineEvent
	require.NoError(t, json.Unmarshal(events, &aggregated))
	require.Len(t, aggregated, 4, "one pre event per member plus one summary")
	assert.Equal(t, 3, aggregated[3].SummaryEvent.ResourceChanges[apitype.OpCreate])
}

// TestRunCandidatePreview_NoGroupsIsOneGroup proves a manifest from before groups existed runs
// as a single subgroup holding every member.
//
//nolint:paralleltest // replaces the CLI binary lookup
func TestRunCandidatePreview_NoGroupsIsOneGroup(t *testing.T) {
	installFakeCLI(t)
	manifest := groupedManifest(t,
		candidatePreviewMember{URN: "urn:a", Name: "a", TargetStack: "acme/a/prod", Directory: "a"},
		candidatePreviewMember{URN: "urn:b", Name: "b", TargetStack: "acme/b/prod", Directory: "b"},
	)
	log := &fakeLog{}
	_, _, _, err := runCandidatePreview(t.Context(), manifest, &fakeSubgroups{log: log})
	require.NoError(t, err)
	got := log.all()
	require.Len(t, got, 2)
	assert.Regexp(t, ` 2$`, got[0])
	assert.True(t, strings.HasPrefix(got[1], "close"))
}

// TestRunCandidatePreview_MemberFailureClosesSubgroup proves a failing member fails the run with
// its stderr tail (secrets redacted), still closes the subgroup, and skips later groups.
//
//nolint:paralleltest // replaces the CLI binary lookup
func TestRunCandidatePreview_MemberFailureClosesSubgroup(t *testing.T) {
	logPath := installFakeCLI(t)
	manifest := groupedManifest(t,
		candidatePreviewMember{URN: "urn:a", Name: "a", TargetStack: "acme/a/fail", Directory: "a",
			GroupOrder: 0, Group: "g1",
			Config: map[string]candidatePreviewConfigValue{"proj:s": {Value: "hunter2", Secret: true}}},
		candidatePreviewMember{URN: "urn:b", Name: "b", TargetStack: "acme/b/prod", Directory: "b",
			GroupOrder: 1, Group: "g2"},
	)
	log := &fakeLog{}
	_, _, _, err := runCandidatePreview(t.Context(), manifest, &fakeSubgroups{log: log})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
	assert.NotContains(t, err.Error(), "hunter2")
	assert.Contains(t, err.Error(), "[secret]")
	got := log.all()
	require.Len(t, got, 2, "g2 never starts")
	assert.True(t, strings.HasPrefix(got[0], "create"))
	assert.True(t, strings.HasPrefix(got[1], "close"))
	assert.NotContains(t, strings.Join(calls(t, logPath), "\n"), "acme/b/prod")
}

//nolint:paralleltest // replaces the CLI binary lookup
func TestRunCandidatePreview_SubgroupCreateFailure(t *testing.T) {
	installFakeCLI(t)
	manifest := groupedManifest(t,
		candidatePreviewMember{URN: "urn:a", Name: "a", TargetStack: "acme/a/prod", Directory: "a"})
	_, _, _, err := runCandidatePreview(t.Context(), manifest,
		&fakeSubgroups{log: &fakeLog{}, createErr: errors.New("nope")})
	require.ErrorContains(t, err, "creating coherence window subgroup")
}

// TestCoherenceWindowSubgroupClientRoutes proves the service client calls the documented routes
// and bodies.
func TestCoherenceWindowSubgroupClientRoutes(t *testing.T) {
	t.Parallel()

	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		requests = append(requests, r.Method+" "+r.URL.Path+" "+string(body))
		if r.Method == http.MethodPost {
			_, _ = w.Write([]byte(`{"id":"sub-9"}`))
		}
	}))
	defer server.Close()
	api := client.NewClient(server.URL, "token", false,
		diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never}))

	id, err := api.CreateCoherenceWindowSubgroup(t.Context(), "acme", "win-1", 3)
	require.NoError(t, err)
	assert.Equal(t, "sub-9", id)
	require.NoError(t, api.CloseCoherenceWindowSubgroup(t.Context(), "acme", "win-1", "sub-9"))
	assert.Equal(t, []string{
		`POST /api/orgs/acme/coherence-windows/win-1/subgroups {"size":3}`,
		`DELETE /api/orgs/acme/coherence-windows/win-1/subgroups/sub-9 `,
	}, requests)
}

// TestCandidatePreviewRequest_FailureIncludesUnavailableRows proves the failure callback still
// reports the manifest's known unavailable dispositions, not just an empty Stacks list, even
// though the failed preview discarded every runnable-member result on error.
func TestCandidatePreviewRequest_FailureIncludesUnavailableRows(t *testing.T) {
	t.Parallel()

	manifest := validManifest()
	manifest.UnpreviewableStacks = []candidatePreviewUnavailableStack{unavailable("urn:services")}

	request := candidatePreviewRequest(manifest, nil, nil, nil, errors.New("member org/a/prod failed"))

	assert.Equal(t, "failed", request.Status)
	assert.Equal(t, "member org/a/prod failed", request.Error)
	require.Len(t, request.Stacks, 1)
	assert.Equal(t, "not-previewable", request.Stacks[0].Status)
	assert.Nil(t, request.Plan)
	assert.Nil(t, request.Events)
}

// TestCandidatePreviewRequest_Success proves a successful run reports exactly the rows, plan and
// events runCandidatePreview produced, with no unavailable-row substitution.
func TestCandidatePreviewRequest_Success(t *testing.T) {
	t.Parallel()

	manifest := validManifest()
	stacks := []client.DeliveryCandidatePreviewStack{{URN: "urn:a", Status: "succeeded"}}
	plan := json.RawMessage(`{"resourcePlans":{}}`)
	events := json.RawMessage(`[]`)

	request := candidatePreviewRequest(manifest, stacks, plan, events, nil)

	assert.Equal(t, "succeeded", request.Status)
	assert.Empty(t, request.Error)
	assert.Equal(t, stacks, request.Stacks)
	assert.Equal(t, plan, request.Plan)
	assert.Equal(t, events, request.Events)
}
