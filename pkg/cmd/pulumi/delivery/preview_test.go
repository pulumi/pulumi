// Copyright 2026, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package delivery

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/auto"
	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

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
