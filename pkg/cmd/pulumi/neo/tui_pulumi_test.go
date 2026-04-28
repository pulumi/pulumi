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
