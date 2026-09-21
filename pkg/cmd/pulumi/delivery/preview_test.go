// Copyright 2026, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/auto"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
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
// filesystem or PreviewMany (a bogus, nonexistent Workspace would fail immediately if it did),
// and returns the unavailable rows with a nil plan.
func TestRunCandidatePreview_AllUnavailable(t *testing.T) {
	t.Parallel()

	manifest := validManifest()
	manifest.Workspace = "/does/not/exist"
	manifest.UnpreviewableStacks = []candidatePreviewUnavailableStack{
		unavailable("urn:services"), unavailable("urn:jobs"),
	}

	stacks, plan, events, err := runCandidatePreview(t.Context(), manifest)
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

// TestAggregateNativePreviewPlan_UnionsPerMemberURNs proves that once every member runs as its
// own real Deployment (each stamping its own project+stack into every URN), the Delivery
// aggregate is a straightforward union of per-member ResourcePlans, keyed by URN.
func TestAggregateNativePreviewPlan_UnionsPerMemberURNs(t *testing.T) {
	t.Parallel()

	urnA := resource.NewURN("dev", "member-a", "", "pulumi:pulumi:Stack", "member-a-dev")
	urnB := resource.NewURN("dev", "member-b", "", "pulumi:pulumi:Stack", "member-b-dev")

	results := []auto.Result{
		{Plan: planWithResource(urnA)},
		{Plan: planWithResource(urnB)},
	}
	members := []candidatePreviewMember{{Name: "a"}, {Name: "b"}}

	merged, err := aggregateNativePreviewPlan(results, members)
	require.NoError(t, err)
	require.Len(t, merged.ResourcePlans, 2)
	assert.Contains(t, merged.ResourcePlans, urnA)
	assert.Contains(t, merged.ResourcePlans, urnB)
	assert.Nil(t, merged.Config, "aggregate must not merge per-member Plan.Config")
}

// TestAggregateNativePreviewPlan_RejectsDuplicateURN proves the aggregate rejects two members
// that (incorrectly, or from a real name collision) produced the identical resource URN, rather
// than silently letting one member's resource overwrite the other's in the approval artifact.
func TestAggregateNativePreviewPlan_RejectsDuplicateURN(t *testing.T) {
	t.Parallel()

	urn := resource.NewURN("dev", "same-project", "", "pulumi:pulumi:Stack", "same-project-dev")
	results := []auto.Result{
		{Plan: planWithResource(urn)},
		{Plan: planWithResource(urn)},
	}
	members := []candidatePreviewMember{{Name: "a"}, {Name: "b"}}

	_, err := aggregateNativePreviewPlan(results, members)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate resource URN")
	assert.Contains(t, err.Error(), "a")
	assert.Contains(t, err.Error(), "b")
}

// TestAggregateNativePreviewEvents_CombinesSummaries proves the combined event stream keeps
// every member's own diff/diagnostic events and collapses the per-member summary events into
// one summary whose resource-change counts are the sum across all members.
func TestAggregateNativePreviewEvents_CombinesSummaries(t *testing.T) {
	t.Parallel()

	memberAEvents := []engine.Event{
		engine.NewEvent(engine.ResourcePreEventPayload{}),
		engine.NewEvent(engine.SummaryEventPayload{
			IsPreview:       true,
			ResourceChanges: display.ResourceChanges{deploy.OpCreate: 1},
		}),
	}
	memberBEvents := []engine.Event{
		engine.NewEvent(engine.ResourcePreEventPayload{}),
		engine.NewEvent(engine.SummaryEventPayload{
			IsPreview:       true,
			ResourceChanges: display.ResourceChanges{deploy.OpCreate: 2, deploy.OpUpdate: 1},
		}),
	}
	results := []auto.Result{{Events: memberAEvents}, {Events: memberBEvents}}

	combined := aggregateNativePreviewEvents(results)

	var summaries int
	var nonSummaries int
	var lastSummary engine.SummaryEventPayload
	for _, event := range combined {
		if event.Type == engine.SummaryEvent {
			summaries++
			lastSummary = event.Payload().(engine.SummaryEventPayload)
			continue
		}
		nonSummaries++
	}
	assert.Equal(t, 1, summaries, "per-member summaries must collapse into exactly one")
	assert.Equal(t, 2, nonSummaries, "every member's non-summary events must be preserved")
	assert.Equal(t, 3, lastSummary.ResourceChanges[deploy.OpCreate], "creates must sum across members")
	assert.Equal(t, 1, lastSummary.ResourceChanges[deploy.OpUpdate])
}

// TestRunCandidatePreview_MixedOrdering proves a manifest with runnable members and unavailable
// rows previews only the members (via the previewManyFunc seam, never touching a real Pulumi
// workspace), and orders the result as members-in-manifest-order then unavailable-in-manifest-
// order, with executed rows marked succeeded and unavailable rows appended unmodified.
//
//nolint:paralleltest // mutates the package-global previewManyFunc seam
func TestRunCandidatePreview_MixedOrdering(t *testing.T) {
	workspace := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(workspace, "a"), 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(workspace, "b"), 0o755))

	manifest := validManifest()
	manifest.Workspace = workspace
	manifest.Members = []candidatePreviewMember{
		{URN: "urn:a", Name: "a", Stage: "prod", TargetStack: "org/a/prod", Directory: "a"},
		{URN: "urn:b", Name: "b", Stage: "prod", TargetStack: "org/b/prod", Directory: "b"},
	}
	manifest.UnpreviewableStacks = []candidatePreviewUnavailableStack{unavailable("urn:services")}

	urnA := resource.NewURN("prod", "a", "", "pulumi:pulumi:Stack", "a-prod")
	urnB := resource.NewURN("prod", "b", "", "pulumi:pulumi:Stack", "b-prod")

	original := previewManyFunc
	t.Cleanup(func() { previewManyFunc = original })
	previewManyFunc = func(_ context.Context, specs []auto.Options) ([]auto.Result, error) {
		require.Len(t, specs, 2, "unavailable rows must never reach PreviewMany")
		// Distinct, spec-order-derived results: if runCandidatePreview zipped a result against
		// the wrong manifest member, these assertions on the *returned* rows below would catch
		// it, since each stack's Changes/Stack are tied to which spec produced which result.
		require.Equal(t, "org/a/prod", specs[0].Stack, "spec order must match manifest member order")
		require.Equal(t, "org/b/prod", specs[1].Stack, "spec order must match manifest member order")
		return []auto.Result{
			{Plan: planWithResource(urnA), Changes: display.ResourceChanges{deploy.OpCreate: 1}},
			{Plan: planWithResource(urnB), Changes: display.ResourceChanges{deploy.OpUpdate: 2}},
		}, nil
	}

	stacks, plan, events, err := runCandidatePreview(t.Context(), manifest)
	require.NoError(t, err)
	require.NotNil(t, plan)
	require.NotNil(t, events)
	require.Len(t, stacks, 3)
	assert.Equal(t, "urn:a", stacks[0].URN)
	assert.Equal(t, "org/a/prod", stacks[0].TargetStack)
	assert.Equal(t, "succeeded", stacks[0].Status)
	assert.Equal(t, map[string]int{"create": 1}, stacks[0].Changes, "member a's result must not be attributed to b")
	assert.Equal(t, "urn:b", stacks[1].URN)
	assert.Equal(t, "org/b/prod", stacks[1].TargetStack)
	assert.Equal(t, "succeeded", stacks[1].Status)
	assert.Equal(t, map[string]int{"update": 2}, stacks[1].Changes, "member b's result must not be attributed to a")
	assert.Equal(t, "urn:services", stacks[2].URN)
	assert.Equal(t, "not-previewable", stacks[2].Status)
	assert.Equal(t, map[string]int{}, stacks[2].Changes)
}

// TestCandidatePreviewRequest_FailureIncludesUnavailableRows proves the failure callback still
// reports the manifest's known unavailable dispositions, not just an empty Stacks list, even
// though PreviewMany discarded every runnable-member result on error.
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
