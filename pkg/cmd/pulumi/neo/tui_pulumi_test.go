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

package neo

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
)

// newTestModel returns a Model wide enough that the pulumi block's border
// doesn't force weird word wrapping, without spinning up a full NewModel (which
// pulls in glamour and textinput setup that aren't needed for block rendering).
func newTestModel() *Model {
	return &Model{width: 100, height: 40}
}

func TestFindOpenPulumiBlock(t *testing.T) {
	t.Parallel()

	m := newTestModel()
	assert.Equal(t, -1, m.findOpenPulumiBlock("pulumi__pulumi_preview"))

	m.blocks = []block{
		{kind: blockAssistantFinal, raw: "hi"},
		{kind: blockPulumiOp, pulumi: &pulumiBlockState{toolName: "pulumi__pulumi_preview"}},
	}
	assert.Equal(t, 1, m.findOpenPulumiBlock("pulumi__pulumi_preview"))

	// A finalized block is not "open" — subsequent lookups for the same name
	// should return -1 so UIPulumiStart for a new run opens a fresh block.
	m.blocks[1].pulumi.done = true
	assert.Equal(t, -1, m.findOpenPulumiBlock("pulumi__pulumi_preview"))

	// Different tool name doesn't match.
	m.blocks[1].pulumi.done = false
	assert.Equal(t, -1, m.findOpenPulumiBlock("pulumi__pulumi_up"))
}

func TestRenderPulumiBlockEmpty(t *testing.T) {
	t.Parallel()

	m := newTestModel()
	st := &pulumiBlockState{
		toolName:      "pulumi__pulumi_preview",
		stackName:     "dev",
		isPreview:     true,
		resourceByURN: map[string]int{},
	}
	out := m.renderPulumiBlock(st)
	assert.Contains(t, out, "PulumiPreview")
	assert.Contains(t, out, "dev")
	assert.Contains(t, out, "running")
}

func TestRenderPulumiBlockWithResources(t *testing.T) {
	t.Parallel()

	m := newTestModel()
	st := &pulumiBlockState{
		toolName:      "pulumi__pulumi_preview",
		stackName:     "dev",
		isPreview:     true,
		resourceByURN: map[string]int{},
		resources: []pulumiResourceRow{
			{
				op: deploy.OpCreate, urn: "urn:pulumi:dev::p::aws:s3/Bucket:Bucket::my-bucket",
				typ: "aws:s3/Bucket:Bucket", status: "planned",
			},
			{
				op: deploy.OpUpdate, urn: "urn:pulumi:dev::p::aws:dynamodb/Table::my-table",
				typ: "aws:dynamodb/Table", status: "planned",
			},
			{
				op: deploy.OpDelete, urn: "urn:pulumi:dev::p::aws:lambda/Function::old-fn",
				typ: "aws:lambda/Function", status: "planned",
			},
		},
	}
	out := m.renderPulumiBlock(st)
	assert.Contains(t, out, "Planned changes")
	assert.Contains(t, out, "my-bucket")
	assert.Contains(t, out, "my-table")
	assert.Contains(t, out, "old-fn")
	// Short-name extraction: the URN's last `::` segment must appear.
	assert.NotContains(t, out, "urn:pulumi:dev::")
}

func TestRenderPulumiBlockWithDiagnostics(t *testing.T) {
	t.Parallel()

	m := newTestModel()
	st := &pulumiBlockState{
		toolName:      "pulumi__pulumi_up",
		stackName:     "prod",
		isPreview:     false,
		resourceByURN: map[string]int{},
		diags: []pulumiDiagRow{
			{severity: "warning", message: "deprecated foo", urn: "urn:pulumi:prod::p::aws:s3/Bucket::b"},
			{severity: "error", message: "bad config", urn: ""},
		},
	}
	out := m.renderPulumiBlock(st)
	assert.Contains(t, out, "Diagnostics")
	assert.Contains(t, out, "deprecated foo")
	assert.Contains(t, out, "bad config")
	// The resource URN suffix is shown for resource-level diagnostics.
	assert.Contains(t, out, "b")
}

func TestRenderPulumiBlockFinalState(t *testing.T) {
	t.Parallel()

	m := newTestModel()
	st := &pulumiBlockState{
		toolName:      "pulumi__pulumi_preview",
		stackName:     "dev",
		isPreview:     true,
		resourceByURN: map[string]int{},
		counts:        display.ResourceChanges{deploy.OpCreate: 3, deploy.OpUpdate: 1, deploy.OpSame: 7},
		elapsed:       "3s",
		done:          true,
	}
	out := m.renderPulumiBlock(st)
	assert.Contains(t, out, "done")
	assert.Contains(t, out, "3s")
	assert.Contains(t, out, "3 create")
	assert.Contains(t, out, "1 update")
	// Same-ops are filtered from the counts footer.
	assert.NotContains(t, out, "7 same")
}

func TestRenderPulumiBlockFailed(t *testing.T) {
	t.Parallel()

	m := newTestModel()
	st := &pulumiBlockState{
		toolName:      "pulumi__pulumi_up",
		stackName:     "prod",
		isPreview:     false,
		resourceByURN: map[string]int{},
		err:           "engine crash",
		done:          true,
	}
	out := m.renderPulumiBlock(st)
	assert.Contains(t, out, "failed")
	assert.Contains(t, out, "engine crash")
}

func TestRenderPulumiCounts(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "no changes", renderPulumiCounts(nil))
	assert.Equal(t, "no changes", renderPulumiCounts(display.ResourceChanges{deploy.OpSame: 5}))
	assert.Equal(t, "2 create · 1 update",
		renderPulumiCounts(display.ResourceChanges{deploy.OpUpdate: 1, deploy.OpCreate: 2, deploy.OpSame: 10}))
	// Zero-count entries are filtered.
	assert.Equal(t, "1 create",
		renderPulumiCounts(display.ResourceChanges{deploy.OpCreate: 1, deploy.OpUpdate: 0}))
}

func TestPulumiOpGlyph(t *testing.T) {
	t.Parallel()

	cases := map[display.StepOp]string{
		deploy.OpCreate:  "+",
		deploy.OpUpdate:  "~",
		deploy.OpDelete:  "-",
		deploy.OpReplace: "+-",
		deploy.OpRead:    "→",
		deploy.OpRefresh: "↻",
		"bogus":          "·",
	}
	for op, want := range cases {
		got, _ := pulumiOpGlyph(op)
		assert.Equal(t, want, got, "op=%s", op)
	}
}

func TestPulumiStateDedupesURN(t *testing.T) {
	t.Parallel()

	// During a pulumi up the engine fires ResourcePre ("running") and then
	// ResourceOutputs ("done") for the same URN. addResource must dedupe by
	// URN and upgrade status in place.
	st := &pulumiBlockState{resourceByURN: map[string]int{}}
	st.addResource(deploy.OpCreate, "urn:1", "aws:s3/Bucket", "running")
	st.addResource(deploy.OpCreate, "urn:1", "aws:s3/Bucket", "done")

	require.Len(t, st.resources, 1)
	assert.Equal(t, "done", st.resources[0].status)
}

func TestRenderPulumiBlockNarrowWidth(t *testing.T) {
	t.Parallel()

	// Tiny terminal. Box width is clamped to a floor so lipgloss doesn't blow up.
	m := &Model{width: 10, height: 20}
	st := &pulumiBlockState{
		toolName:      "pulumi__pulumi_preview",
		stackName:     "dev",
		isPreview:     true,
		resourceByURN: map[string]int{},
	}
	out := m.renderPulumiBlock(st)
	// The output must be non-empty and not panic; exact visual layout at
	// width=10 isn't interesting to assert.
	require.NotEmpty(t, out)
	assert.Contains(t, strings.ToLower(out), "pulumipreview")
}

// -----------------------------------------------------------------------------
// Model.Update tests for the UIPulumi* events. These exercise the four event
// handlers added to tui.go in this PR. Pattern matches the other UI* Update
// tests in tui_test.go (e.g. TestModel_Update_UIToolStarted_ShowsBusyBlock).
// -----------------------------------------------------------------------------

func TestModel_Update_UIPulumiStart_OpensBlock(t *testing.T) {
	t.Parallel()

	ch := make(chan UIEvent, 4)
	m := NewModel(ModelConfig{EventCh: ch})
	updated, _ := m.Update(UIPulumiStart{
		ToolName: "pulumi__pulumi_preview", StackName: "dev", IsPreview: true,
	})
	um := updated.(Model)

	require.Len(t, um.blocks, 1, "Start opens exactly one block")
	got := um.blocks[0]
	assert.Equal(t, blockPulumiOp, got.kind)
	require.NotNil(t, got.pulumi)
	assert.Equal(t, "pulumi__pulumi_preview", got.pulumi.toolName)
	assert.Equal(t, "dev", got.pulumi.stackName)
	assert.True(t, got.pulumi.isPreview)
	assert.False(t, got.pulumi.done, "block must start in the open state")
	assert.NotEmpty(t, got.rendered, "Start must render the block immediately")
}

func TestModel_Update_UIPulumiStart_ReusesOpenBlock(t *testing.T) {
	t.Parallel()

	// Defensive: if a UIPulumiStart arrives while another block for the same
	// tool name is still open (e.g. agent retry), the existing block is
	// reset rather than a duplicate appearing in the transcript.
	ch := make(chan UIEvent, 4)
	m := NewModel(ModelConfig{EventCh: ch})
	m.blocks = []block{{
		kind: blockPulumiOp,
		pulumi: &pulumiBlockState{
			toolName:      "pulumi__pulumi_preview",
			stackName:     "stale",
			resourceByURN: map[string]int{"urn:x": 0},
			resources:     []pulumiResourceRow{{urn: "urn:x"}},
		},
	}}

	updated, _ := m.Update(UIPulumiStart{
		ToolName: "pulumi__pulumi_preview", StackName: "fresh", IsPreview: true,
	})
	um := updated.(Model)

	require.Len(t, um.blocks, 1, "must not append a second block for the same open tool")
	st := um.blocks[0].pulumi
	assert.Equal(t, "fresh", st.stackName, "state must be replaced, not merged")
	assert.Empty(t, st.resources, "resources from the prior run must be cleared")
	require.NotNil(t, st.resourceByURN)
	assert.Empty(t, st.resourceByURN)
}

func TestModel_Update_UIPulumiStart_StartsFreshAfterDone(t *testing.T) {
	t.Parallel()

	// A finalized block (done=true) is no longer "open"; a subsequent Start
	// for the same tool name must append a new block, not overwrite history.
	ch := make(chan UIEvent, 4)
	m := NewModel(ModelConfig{EventCh: ch})
	m.blocks = []block{{
		kind: blockPulumiOp,
		pulumi: &pulumiBlockState{
			toolName:      "pulumi__pulumi_preview",
			done:          true,
			resourceByURN: map[string]int{},
		},
	}}

	updated, _ := m.Update(UIPulumiStart{
		ToolName: "pulumi__pulumi_preview", StackName: "second", IsPreview: true,
	})
	um := updated.(Model)

	require.Len(t, um.blocks, 2, "a fresh Start after done must append a new block")
	assert.True(t, um.blocks[0].pulumi.done, "old block stays finalized")
	assert.False(t, um.blocks[1].pulumi.done, "new block is open")
	assert.Equal(t, "second", um.blocks[1].pulumi.stackName)
}

func TestModel_Update_UIPulumiResource_AppendsRow(t *testing.T) {
	t.Parallel()

	ch := make(chan UIEvent, 4)
	m := NewModel(ModelConfig{EventCh: ch})
	updated, _ := m.Update(UIPulumiStart{
		ToolName: "pulumi__pulumi_preview", StackName: "dev", IsPreview: true,
	})
	updated, _ = updated.(Model).Update(UIPulumiResource{
		ToolName: "pulumi__pulumi_preview",
		Op:       deploy.OpCreate,
		URN:      "urn:pulumi:dev::p::aws:s3/Bucket::b",
		Type:     "aws:s3/Bucket",
		Status:   "planned",
	})
	um := updated.(Model)

	require.Len(t, um.blocks, 1)
	require.Len(t, um.blocks[0].pulumi.resources, 1)
	row := um.blocks[0].pulumi.resources[0]
	assert.Equal(t, deploy.OpCreate, row.op)
	assert.Equal(t, "planned", row.status)
}

func TestModel_Update_UIPulumiResource_NoBlockIsDefensiveNoOp(t *testing.T) {
	t.Parallel()

	// If a Resource event arrives without a preceding Start (e.g. event
	// reordering on the wire), the handler must not panic and must not
	// fabricate a block out of thin air.
	ch := make(chan UIEvent, 4)
	m := NewModel(ModelConfig{EventCh: ch})
	require.NotPanics(t, func() {
		_, _ = m.Update(UIPulumiResource{
			ToolName: "pulumi__pulumi_preview",
			Op:       deploy.OpCreate,
			URN:      "urn:x",
		})
	})
	updated, _ := m.Update(UIPulumiResource{
		ToolName: "pulumi__pulumi_preview", Op: deploy.OpCreate, URN: "urn:x",
	})
	um := updated.(Model)
	assert.Empty(t, um.blocks, "no block must be created without a Start")
}

func TestModel_Update_UIPulumiDiag_AppendsRow(t *testing.T) {
	t.Parallel()

	ch := make(chan UIEvent, 4)
	m := NewModel(ModelConfig{EventCh: ch})
	updated, _ := m.Update(UIPulumiStart{
		ToolName: "pulumi__pulumi_up", StackName: "prod", IsPreview: false,
	})
	updated, _ = updated.(Model).Update(UIPulumiDiag{
		ToolName: "pulumi__pulumi_up",
		Severity: "warning",
		Message:  "deprecated foo",
		URN:      "urn:pulumi:prod::p::aws:s3/Bucket::b",
	})
	um := updated.(Model)

	require.Len(t, um.blocks[0].pulumi.diags, 1)
	d := um.blocks[0].pulumi.diags[0]
	assert.Equal(t, "warning", d.severity)
	assert.Equal(t, "deprecated foo", d.message)
}

func TestModel_Update_UIPulumiDiag_NoBlockIsDefensiveNoOp(t *testing.T) {
	t.Parallel()

	ch := make(chan UIEvent, 4)
	m := NewModel(ModelConfig{EventCh: ch})
	require.NotPanics(t, func() {
		_, _ = m.Update(UIPulumiDiag{
			ToolName: "pulumi__pulumi_up", Severity: "error", Message: "x",
		})
	})
}

func TestModel_Update_UIPulumiEnd_FinalizesBlock(t *testing.T) {
	t.Parallel()

	ch := make(chan UIEvent, 4)
	m := NewModel(ModelConfig{EventCh: ch})
	updated, _ := m.Update(UIPulumiStart{
		ToolName: "pulumi__pulumi_preview", StackName: "dev", IsPreview: true,
	})
	updated, _ = updated.(Model).Update(UIPulumiEnd{
		ToolName: "pulumi__pulumi_preview",
		Counts:   display.ResourceChanges{deploy.OpCreate: 2, deploy.OpUpdate: 1},
		Elapsed:  "5s",
	})
	um := updated.(Model)

	require.Len(t, um.blocks, 1)
	st := um.blocks[0].pulumi
	assert.True(t, st.done, "End must mark the block done")
	assert.Equal(t, "5s", st.elapsed)
	assert.Equal(t, display.ResourceChanges{deploy.OpCreate: 2, deploy.OpUpdate: 1}, st.counts)
	assert.Empty(t, st.err)

	// A subsequent Start for the same tool name must open a new block, not
	// reopen this one — proves done-flag gating works end-to-end.
	updated, _ = um.Update(UIPulumiStart{ToolName: "pulumi__pulumi_preview"})
	require.Len(t, updated.(Model).blocks, 2)
}

func TestModel_Update_UIPulumiEnd_CarriesError(t *testing.T) {
	t.Parallel()

	ch := make(chan UIEvent, 4)
	m := NewModel(ModelConfig{EventCh: ch})
	updated, _ := m.Update(UIPulumiStart{ToolName: "pulumi__pulumi_up"})
	updated, _ = updated.(Model).Update(UIPulumiEnd{
		ToolName: "pulumi__pulumi_up",
		Err:      "engine crash",
		Elapsed:  "3s",
	})
	um := updated.(Model)

	st := um.blocks[0].pulumi
	assert.True(t, st.done)
	assert.Equal(t, "engine crash", st.err)
}

func TestModel_Update_UIPulumiEnd_NoBlockIsDefensiveNoOp(t *testing.T) {
	t.Parallel()

	ch := make(chan UIEvent, 4)
	m := NewModel(ModelConfig{EventCh: ch})
	require.NotPanics(t, func() {
		_, _ = m.Update(UIPulumiEnd{ToolName: "pulumi__pulumi_up"})
	})
}

func TestModel_Update_UIPulumi_LeavesBusyAlone(t *testing.T) {
	t.Parallel()

	// UIPulumi* are progress events for an in-flight tool call; they must
	// neither end the busy state (the agent is still working through the
	// tool) nor change the busy *label* (the parent UIToolStarted set
	// "PulumiPreview ..." and that's the right label to keep showing).
	ch := make(chan UIEvent, 4)
	m := NewModel(ModelConfig{EventCh: ch, Busy: true})
	require.True(t, m.busy)

	for _, ev := range []UIEvent{
		UIPulumiStart{ToolName: "pulumi__pulumi_preview", IsPreview: true},
		UIPulumiResource{ToolName: "pulumi__pulumi_preview", Op: deploy.OpCreate, URN: "urn:x"},
		UIPulumiDiag{ToolName: "pulumi__pulumi_preview", Severity: "warning", Message: "w"},
		UIPulumiEnd{ToolName: "pulumi__pulumi_preview", Elapsed: "1s"},
	} {
		updated, _ := m.Update(ev)
		m = updated.(Model)
		assert.True(t, m.busy, "event %T must not end the busy state", ev)
	}
}
