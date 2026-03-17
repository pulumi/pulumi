// Copyright 2016-2026, Pulumi Corporation.
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

package deploy_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

// recordingPolicyEvents records all policy events for later inspection.
type recordingPolicyEvents struct {
	violations    []plugin.AnalyzeDiagnostic
	remediations  []remediationRecord
	analyzeSumm   []plugin.PolicySummary
	remediateSumm []plugin.PolicySummary
	stackSumm     []plugin.PolicySummary
}

type remediationRecord struct {
	urn    resource.URN
	before resource.PropertyMap
	after  resource.PropertyMap
}

func (r *recordingPolicyEvents) OnPolicyViolation(urn resource.URN, d plugin.AnalyzeDiagnostic) {
	r.violations = append(r.violations, d)
}

func (r *recordingPolicyEvents) OnPolicyRemediation(
	urn resource.URN, _ plugin.Remediation,
	before, after resource.PropertyMap,
) {
	r.remediations = append(r.remediations, remediationRecord{urn: urn, before: before, after: after})
}

func (r *recordingPolicyEvents) OnPolicyAnalyzeSummary(s plugin.PolicySummary) {
	r.analyzeSumm = append(r.analyzeSumm, s)
}

func (r *recordingPolicyEvents) OnPolicyRemediateSummary(s plugin.PolicySummary) {
	r.remediateSumm = append(r.remediateSumm, s)
}

func (r *recordingPolicyEvents) OnPolicyAnalyzeStackSummary(s plugin.PolicySummary) {
	r.stackSumm = append(r.stackSumm, s)
}

// makeTestResource creates a simple resource.State for use in tests.
func makeTestResource(urn resource.URN) *resource.State {
	return &resource.State{
		Type:    tokens.Type("pkg:index:MyResource"),
		URN:     urn,
		Custom:  true,
		Outputs: resource.PropertyMap{"key": resource.NewProperty("value")},
	}
}

func TestAnalyzeSnapshot_NilSnapshot(t *testing.T) {
	t.Parallel()

	events := &recordingPolicyEvents{}
	analyzer := &deploytest.Analyzer{
		Info: plugin.AnalyzerInfo{Name: "test"},
	}

	hasMandatory, err := deploy.AnalyzeSnapshot(context.Background(), nil, []plugin.Analyzer{analyzer}, events)
	require.NoError(t, err)
	assert.False(t, hasMandatory)
	assert.Empty(t, events.violations)
}

func TestAnalyzeSnapshot_NoAnalyzers(t *testing.T) {
	t.Parallel()

	snap := &deploy.Snapshot{
		Resources: []*resource.State{
			makeTestResource("urn:pulumi:stack::project::pkg:index:MyResource::res"),
		},
	}
	events := &recordingPolicyEvents{}

	hasMandatory, err := deploy.AnalyzeSnapshot(context.Background(), snap, nil, events)
	require.NoError(t, err)
	assert.False(t, hasMandatory)
	assert.Empty(t, events.violations)
}

func TestAnalyzeSnapshot_EmptySnapshot(t *testing.T) {
	t.Parallel()

	snap := &deploy.Snapshot{}
	events := &recordingPolicyEvents{}
	analyzer := &deploytest.Analyzer{
		Info: plugin.AnalyzerInfo{Name: "test"},
	}

	hasMandatory, err := deploy.AnalyzeSnapshot(context.Background(), snap, []plugin.Analyzer{analyzer}, events)
	require.NoError(t, err)
	assert.False(t, hasMandatory)
}

func TestAnalyzeSnapshot_AdvisoryViolationReturnsFalse(t *testing.T) {
	t.Parallel()

	urn := resource.URN("urn:pulumi:stack::project::pkg:index:MyResource::res")
	snap := &deploy.Snapshot{
		Resources: []*resource.State{makeTestResource(urn)},
	}
	events := &recordingPolicyEvents{}

	analyzer := &deploytest.Analyzer{
		Info: plugin.AnalyzerInfo{Name: "test-pack"},
		AnalyzeF: func(r plugin.AnalyzerResource) (plugin.AnalyzeResponse, error) {
			return plugin.AnalyzeResponse{
				Diagnostics: []plugin.AnalyzeDiagnostic{{
					PolicyName:       "my-policy",
					PolicyPackName:   "test-pack",
					EnforcementLevel: apitype.Advisory,
					Message:          "advisory warning",
					URN:              r.URN,
				}},
			}, nil
		},
	}

	hasMandatory, err := deploy.AnalyzeSnapshot(context.Background(), snap, []plugin.Analyzer{analyzer}, events)
	require.NoError(t, err)
	assert.False(t, hasMandatory, "advisory violations should not return true")
	require.Len(t, events.violations, 1)
	assert.Equal(t, apitype.Advisory, events.violations[0].EnforcementLevel)
	assert.Equal(t, "advisory warning", events.violations[0].Message)
}

func TestAnalyzeSnapshot_MandatoryViolationReturnsTrue(t *testing.T) {
	t.Parallel()

	urn := resource.URN("urn:pulumi:stack::project::pkg:index:MyResource::res")
	snap := &deploy.Snapshot{
		Resources: []*resource.State{makeTestResource(urn)},
	}
	events := &recordingPolicyEvents{}

	analyzer := &deploytest.Analyzer{
		Info: plugin.AnalyzerInfo{Name: "test-pack"},
		AnalyzeF: func(r plugin.AnalyzerResource) (plugin.AnalyzeResponse, error) {
			return plugin.AnalyzeResponse{
				Diagnostics: []plugin.AnalyzeDiagnostic{{
					PolicyName:       "must-tag",
					PolicyPackName:   "test-pack",
					EnforcementLevel: apitype.Mandatory,
					Message:          "resource is not tagged",
					URN:              r.URN,
				}},
			}, nil
		},
	}

	hasMandatory, err := deploy.AnalyzeSnapshot(context.Background(), snap, []plugin.Analyzer{analyzer}, events)
	require.NoError(t, err)
	assert.True(t, hasMandatory)
	require.Len(t, events.violations, 1)
	assert.Equal(t, apitype.Mandatory, events.violations[0].EnforcementLevel)
}

func TestAnalyzeSnapshot_SkipsDeletedResources(t *testing.T) {
	t.Parallel()

	live := makeTestResource("urn:pulumi:stack::project::pkg:index:MyResource::live")
	deleted := makeTestResource("urn:pulumi:stack::project::pkg:index:MyResource::deleted")
	deleted.Delete = true

	snap := &deploy.Snapshot{Resources: []*resource.State{live, deleted}}
	events := &recordingPolicyEvents{}

	var analyzed []resource.URN
	analyzer := &deploytest.Analyzer{
		Info: plugin.AnalyzerInfo{Name: "test"},
		AnalyzeF: func(r plugin.AnalyzerResource) (plugin.AnalyzeResponse, error) {
			analyzed = append(analyzed, r.URN)
			return plugin.AnalyzeResponse{}, nil
		},
	}

	_, err := deploy.AnalyzeSnapshot(context.Background(), snap, []plugin.Analyzer{analyzer}, events)
	require.NoError(t, err)
	assert.Equal(t, []resource.URN{live.URN}, analyzed)
}

func TestAnalyzeSnapshot_RemediationReportedNotApplied(t *testing.T) {
	t.Parallel()

	urn := resource.URN("urn:pulumi:stack::project::pkg:index:MyResource::res")
	original := resource.PropertyMap{"key": resource.NewProperty("bad")}
	remediated := resource.PropertyMap{"key": resource.NewProperty("good")}

	res := makeTestResource(urn)
	res.Inputs = original

	snap := &deploy.Snapshot{Resources: []*resource.State{res}}
	events := &recordingPolicyEvents{}

	analyzer := &deploytest.Analyzer{
		Info: plugin.AnalyzerInfo{Name: "test-pack"},
		RemediateF: func(r plugin.AnalyzerResource) (plugin.RemediateResponse, error) {
			return plugin.RemediateResponse{
				Remediations: []plugin.Remediation{{
					PolicyName:     "fix-key",
					PolicyPackName: "test-pack",
					Description:    "sets key to good",
					Properties:     remediated,
				}},
			}, nil
		},
		// Analysis still sees the original properties (remediation was not applied).
		AnalyzeF: func(r plugin.AnalyzerResource) (plugin.AnalyzeResponse, error) {
			assert.Equal(t, original, r.Properties,
				"analysis should see original properties, not remediated ones")
			return plugin.AnalyzeResponse{}, nil
		},
	}

	hasMandatory, err := deploy.AnalyzeSnapshot(context.Background(), snap, []plugin.Analyzer{analyzer}, events)
	require.NoError(t, err)
	assert.False(t, hasMandatory)
	require.Len(t, events.remediations, 1, "should report one remediation")
	assert.Equal(t, original, events.remediations[0].before)
	assert.Equal(t, remediated, events.remediations[0].after)
}

func TestAnalyzeSnapshot_RemediationDiagnosticReportedAsAdvisory(t *testing.T) {
	t.Parallel()

	urn := resource.URN("urn:pulumi:stack::project::pkg:index:MyResource::res")
	snap := &deploy.Snapshot{Resources: []*resource.State{makeTestResource(urn)}}
	events := &recordingPolicyEvents{}

	analyzer := &deploytest.Analyzer{
		Info: plugin.AnalyzerInfo{Name: "test-pack"},
		RemediateF: func(r plugin.AnalyzerResource) (plugin.RemediateResponse, error) {
			return plugin.RemediateResponse{
				Remediations: []plugin.Remediation{{
					PolicyName:     "warn-policy",
					PolicyPackName: "test-pack",
					Diagnostic:     "cannot auto-remediate: manual action required",
				}},
			}, nil
		},
	}

	_, err := deploy.AnalyzeSnapshot(context.Background(), snap, []plugin.Analyzer{analyzer}, events)
	require.NoError(t, err)
	require.Len(t, events.violations, 1)
	assert.Equal(t, apitype.Advisory, events.violations[0].EnforcementLevel)
	assert.Equal(t, "cannot auto-remediate: manual action required", events.violations[0].Message)
}

func TestAnalyzeSnapshot_StackLevelViolation(t *testing.T) {
	t.Parallel()

	urn := resource.URN("urn:pulumi:stack::project::pkg:index:MyResource::res")
	snap := &deploy.Snapshot{Resources: []*resource.State{makeTestResource(urn)}}
	events := &recordingPolicyEvents{}

	analyzer := &deploytest.Analyzer{
		Info: plugin.AnalyzerInfo{Name: "test-pack"},
		AnalyzeStackF: func(resources []plugin.AnalyzerStackResource) (plugin.AnalyzeResponse, error) {
			return plugin.AnalyzeResponse{
				Diagnostics: []plugin.AnalyzeDiagnostic{{
					PolicyName:       "stack-policy",
					PolicyPackName:   "test-pack",
					EnforcementLevel: apitype.Mandatory,
					Message:          "stack violates policy",
				}},
			}, nil
		},
	}

	hasMandatory, err := deploy.AnalyzeSnapshot(context.Background(), snap, []plugin.Analyzer{analyzer}, events)
	require.NoError(t, err)
	assert.True(t, hasMandatory)
	require.Len(t, events.violations, 1)
	assert.Equal(t, "stack violates policy", events.violations[0].Message)
	require.Len(t, events.stackSumm, 1)
}

func TestAnalyzeSnapshot_AnalyzeErrorPropagated(t *testing.T) {
	t.Parallel()

	urn := resource.URN("urn:pulumi:stack::project::pkg:index:MyResource::res")
	snap := &deploy.Snapshot{Resources: []*resource.State{makeTestResource(urn)}}
	events := &recordingPolicyEvents{}

	analyzeErr := errors.New("analyzer exploded")
	analyzer := &deploytest.Analyzer{
		Info: plugin.AnalyzerInfo{Name: "test"},
		AnalyzeF: func(r plugin.AnalyzerResource) (plugin.AnalyzeResponse, error) {
			return plugin.AnalyzeResponse{}, analyzeErr
		},
	}

	_, err := deploy.AnalyzeSnapshot(context.Background(), snap, []plugin.Analyzer{analyzer}, events)
	require.Error(t, err)
	assert.ErrorContains(t, err, "failed to run policy")
}

func TestAnalyzeSnapshot_RemediateErrorPropagated(t *testing.T) {
	t.Parallel()

	urn := resource.URN("urn:pulumi:stack::project::pkg:index:MyResource::res")
	snap := &deploy.Snapshot{Resources: []*resource.State{makeTestResource(urn)}}
	events := &recordingPolicyEvents{}

	remediateErr := errors.New("remediation exploded")
	analyzer := &deploytest.Analyzer{
		Info: plugin.AnalyzerInfo{Name: "test"},
		RemediateF: func(r plugin.AnalyzerResource) (plugin.RemediateResponse, error) {
			return plugin.RemediateResponse{}, remediateErr
		},
	}

	_, err := deploy.AnalyzeSnapshot(context.Background(), snap, []plugin.Analyzer{analyzer}, events)
	require.Error(t, err)
	assert.ErrorContains(t, err, "failed to run remediation")
}

func TestAnalyzeSnapshot_MultipleResources(t *testing.T) {
	t.Parallel()

	res1 := makeTestResource("urn:pulumi:stack::project::pkg:index:MyResource::res1")
	res2 := makeTestResource("urn:pulumi:stack::project::pkg:index:MyResource::res2")
	snap := &deploy.Snapshot{Resources: []*resource.State{res1, res2}}
	events := &recordingPolicyEvents{}

	analyzer := &deploytest.Analyzer{
		Info: plugin.AnalyzerInfo{Name: "test-pack"},
		AnalyzeF: func(r plugin.AnalyzerResource) (plugin.AnalyzeResponse, error) {
			return plugin.AnalyzeResponse{
				Diagnostics: []plugin.AnalyzeDiagnostic{{
					PolicyName:       "all-must-comply",
					PolicyPackName:   "test-pack",
					EnforcementLevel: apitype.Mandatory,
					Message:          "violation on " + string(r.URN),
					URN:              r.URN,
				}},
			}, nil
		},
	}

	hasMandatory, err := deploy.AnalyzeSnapshot(context.Background(), snap, []plugin.Analyzer{analyzer}, events)
	require.NoError(t, err)
	assert.True(t, hasMandatory)
	require.Len(t, events.violations, 2, "both resources should be analyzed")
}

func TestAnalyzeSnapshot_SummaryEventsEmitted(t *testing.T) {
	t.Parallel()

	urn := resource.URN("urn:pulumi:stack::project::pkg:index:MyResource::res")
	snap := &deploy.Snapshot{Resources: []*resource.State{makeTestResource(urn)}}
	events := &recordingPolicyEvents{}

	analyzer := &deploytest.Analyzer{
		Info: plugin.AnalyzerInfo{Name: "test-pack"},
	}

	_, err := deploy.AnalyzeSnapshot(context.Background(), snap, []plugin.Analyzer{analyzer}, events)
	require.NoError(t, err)
	require.Len(t, events.analyzeSumm, 1, "one analyze summary per resource×analyzer")
	require.Len(t, events.remediateSumm, 1, "one remediate summary per resource×analyzer")
	require.Len(t, events.stackSumm, 1, "one stack summary per analyzer")
}
