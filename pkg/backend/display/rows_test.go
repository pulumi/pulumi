// Copyright 2025, Pulumi Corporation.
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

package display

import (
	"bytes"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend/display/internal/terminal"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/require"
)

func TestResourceRowDataColorizedColumns(t *testing.T) {
	t.Parallel()

	out := bytes.Buffer{}
	term := terminal.NewMockTerminal(&out, 80, 20, true)
	_, display := createRendererAndDisplay(term, true)
	display.isTerminal = true

	for _, tt := range []struct {
		name     string
		urn      string
		expected string
	}{
		{
			name:     "control chars",
			urn:      "urn:pulumi:stack:proj::\tprovider:res\n",
			expected: "stack:proj::\\tprovider:res\\n",
		},
		{
			name:     "emoji",
			urn:      "urn:pulumi:stack:proj::provider:🦄",
			expected: "stack:proj::provider:🦄",
		},
		{
			name: "emoji with ZWJ",
			urn:  "urn:pulumi:stack:proj::provider:\U0001F575\U0001F3FD\u200D\u2642\uFE0F", // Emoji with ZWJ 🕵🏽‍♂️
			// Arguably this could be as is without escaping, but
			// strconv.QuoteToGraphic always escales zero width spaces..
			expected: "stack:proj::provider:🕵🏽\\u200d♂️",
		},
		{
			name:     "zwj",
			urn:      "urn:pulumi:stack:proj::provider:A\u00A0Z", // Non-breaking space
			expected: "stack:proj::provider:A\u00a0Z",
		},
		{
			name:     "zwj",
			urn:      "urn:pulumi:stack:proj::provider:A\u200dZ", // ZWJ
			expected: "stack:proj::provider:A\\u200dZ",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			row := &resourceRowData{
				display:  display,
				diagInfo: &DiagInfo{},
				step: engine.StepEventMetadata{
					URN: resource.URN(tt.urn),
					Op:  deploy.OpUpdate,
				},
				hideRowIfUnnecessary: true,
			}

			cols := row.ColorizedColumns()
			name := cols[nameColumn]
			require.Equal(t, tt.expected, name)
		})
	}
}

func TestRenderDiffStateMigrationEvent(t *testing.T) {
	t.Parallel()

	left := resource.URN("urn:pulumi:test::test::example:index:Component$example:index:Left$example:index:Part::shared")
	right := resource.URN("urn:pulumi:test::test::example:index:Component$example:index:Right$example:index:Part::shared")
	unified := resource.URN("urn:pulumi:test::test::example:index:Component$example:index:Unified::unified")
	added := resource.URN("urn:pulumi:test::test::example:index:Component$example:index:Added::added")
	root := resource.URN("urn:pulumi:test::test::example:index:Component::component")
	payload := engine.StateMigrationEventPayload{
		URN:   root,
		Added: []resource.URN{added, unified},
		Successors: map[resource.URN]resource.URN{
			right: unified,
			left:  unified,
		},
	}

	require.Equal(t, "    example:index:Component::component: state migrated\n"+
		"        example:index:Unified::unified is the successor of:\n"+
		"            example:index:Component$example:index:Left$example:index:Part::shared\n"+
		"            example:index:Component$example:index:Right$example:index:Part::shared\n"+
		"        added resource: example:index:Added::added\n",
		renderDiffStateMigrationEvent(payload, Options{}))
	require.Equal(t, "    "+string(root)+": state migrated\n"+
		"        "+string(unified)+" is the successor of:\n"+
		"            "+string(left)+"\n"+
		"            "+string(right)+"\n"+
		"        added resource: "+string(added)+"\n",
		renderDiffStateMigrationEvent(payload, Options{ShowURNs: true}))
}

func TestRenderDiffStateMigrationEventDisambiguatesSharedNames(t *testing.T) {
	t.Parallel()

	oldComponent := resource.URN("urn:pulumi:dev::test::awsx:ecr:Repository::repo")
	oldRepository := resource.URN(
		"urn:pulumi:dev::test::awsx:ecr:Repository$aws:ecr/repository:Repository::repo")
	oldPolicy := resource.URN(
		"urn:pulumi:dev::test::awsx:ecr:Repository$aws:ecr/lifecyclePolicy:LifecyclePolicy::repo")
	newRepository := resource.URN("urn:pulumi:dev::test::aws:ecr/repository:Repository::repository")
	newPolicy := resource.URN(
		"urn:pulumi:dev::test::aws:ecr/repository:Repository$aws:ecr/lifecyclePolicy:LifecyclePolicy::lifecycle-policy")
	payload := engine.StateMigrationEventPayload{
		URN:   newRepository,
		Added: []resource.URN{newRepository, newPolicy},
		Successors: map[resource.URN]resource.URN{
			oldComponent:  newRepository,
			oldRepository: newRepository,
			oldPolicy:     newPolicy,
		},
	}

	require.Equal(t, "    aws:ecr:Repository::repository: state migrated\n"+
		"        aws:ecr:LifecyclePolicy::lifecycle-policy is the successor of:\n"+
		"            aws:ecr:LifecyclePolicy::repo\n"+
		"        aws:ecr:Repository::repository is the successor of:\n"+
		"            aws:ecr:Repository::repo\n"+
		"            awsx:ecr:Repository::repo\n",
		renderDiffStateMigrationEvent(payload, Options{}))
}

func TestStateMigrationProgressMarkerIsCompact(t *testing.T) {
	t.Parallel()

	row := &resourceRowData{
		display:  &ProgressDisplay{},
		diagInfo: &DiagInfo{},
		step: engine.StepEventMetadata{
			URN: "urn:pulumi:test::test::example:index:Component::component",
			Op:  deploy.OpSame,
		},
		stateMigrationPayloads: []engine.StateMigrationEventPayload{{}},
	}
	require.Equal(t, "[state migrated]", colors.Never.Colorize(row.getInfoColumn()))
}

type recordingProgressRenderer struct {
	lines []string
}

func (*recordingProgressRenderer) Close() error                               { return nil }
func (*recordingProgressRenderer) initializeDisplay(*ProgressDisplay)         {}
func (*recordingProgressRenderer) tick()                                      {}
func (*recordingProgressRenderer) rowUpdated(Row)                             {}
func (*recordingProgressRenderer) systemMessage(engine.StdoutEventPayload)    {}
func (*recordingProgressRenderer) progress(engine.ProgressEventPayload, bool) {}
func (*recordingProgressRenderer) done()                                      {}
func (renderer *recordingProgressRenderer) println(line string) {
	renderer.lines = append(renderer.lines, colors.Never.Colorize(line))
}

func TestProgressDisplayPrintsStateMigrationDetailsOutsideTable(t *testing.T) {
	t.Parallel()

	root := resource.URN("urn:pulumi:test::test::example:index:Component::component")
	oldChild := resource.URN("urn:pulumi:test::test::example:index:Component$example:index:Child::old")
	newChild := resource.URN("urn:pulumi:test::test::example:index:Component$example:index:Child::new")
	payload := engine.StateMigrationEventPayload{
		URN:   root,
		Added: []resource.URN{newChild},
		Successors: map[resource.URN]resource.URN{
			oldChild: newChild,
		},
	}
	renderer := &recordingProgressRenderer{}
	display := &ProgressDisplay{
		renderer: renderer,
		eventUrnToResourceRow: map[resource.URN]ResourceRow{
			root: &resourceRowData{
				step:                   engine.StepEventMetadata{URN: root},
				stateMigrationPayloads: []engine.StateMigrationEventPayload{payload},
			},
		},
	}

	display.printStateMigrations()

	require.Equal(t, []string{
		"State migrations:",
		"    example:index:Component::component: state migrated",
		"        example:index:Child::new is the successor of:",
		"            example:index:Child::old",
		"",
	}, renderer.lines)
}

func TestResourceRowDataInterruptedStatus(t *testing.T) {
	t.Parallel()

	customCreate := engine.StepEventMetadata{
		URN: "urn:pulumi:stack::proj::aws:eks/nodeGroup:NodeGroup::ng",
		Op:  deploy.OpCreate,
		New: &engine.StepEventStateMetadata{Custom: true},
	}
	componentCreate := engine.StepEventMetadata{
		URN: "urn:pulumi:stack::proj::my:component:Thing::c",
		Op:  deploy.OpCreate,
		New: &engine.StepEventStateMetadata{Custom: false},
	}
	customToComponentUpdate := engine.StepEventMetadata{
		URN: "urn:pulumi:stack::proj::my:component:Thing::c",
		Op:  deploy.OpUpdate,
		Old: &engine.StepEventStateMetadata{Custom: true},
		New: &engine.StepEventStateMetadata{Custom: false},
		Res: &engine.StepEventStateMetadata{Custom: false},
	}

	for _, tt := range []struct {
		name        string
		step        engine.StepEventMetadata
		hasOutputs  bool
		displayDone bool
		expectMatch string
		rejectMatch string
	}{
		{
			name:        "custom create interrupted",
			step:        customCreate,
			displayDone: true,
			expectMatch: "creating (interrupted)",
			rejectMatch: "created",
		},
		{
			name:        "custom create completed",
			step:        customCreate,
			hasOutputs:  true,
			displayDone: true,
			expectMatch: "created",
		},
		{
			name:        "component create without outputs is not interrupted",
			step:        componentCreate,
			displayDone: true,
			expectMatch: "created",
		},
		{
			name:        "still running is not interrupted",
			step:        customCreate,
			displayDone: false,
			rejectMatch: "interrupted",
		},
		{
			name:        "custom converted to component without outputs is not interrupted",
			step:        customToComponentUpdate,
			displayDone: true,
			expectMatch: "updated",
			rejectMatch: "interrupted",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			out := bytes.Buffer{}
			term := terminal.NewMockTerminal(&out, 200, 20, true)
			_, display := createRendererAndDisplay(term, true)
			display.opts.SuppressTimings = true
			display.done.Store(tt.displayDone)

			row := &resourceRowData{
				display:  display,
				diagInfo: &DiagInfo{},
				step:     tt.step,
			}
			if tt.hasOutputs {
				row.outputSteps = []engine.StepEventMetadata{tt.step}
			}

			status := row.ColorizedColumns()[statusColumn]
			if tt.expectMatch != "" {
				require.Contains(t, status, tt.expectMatch)
			}
			if tt.rejectMatch != "" {
				require.NotContains(t, status, tt.rejectMatch)
			}
		})
	}
}

// TestGetDiffInfo_FiltersInternalProperties tests that internal properties like __defaults
// are not shown in the short diff display. This is a regression test for issue #2586.
func TestGetDiffInfo_FiltersInternalProperties(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		oldInputs      resource.PropertyMap
		newInputs      resource.PropertyMap
		expectDiff     bool
		shouldMatch    string
		shouldNotMatch string
	}{
		{
			name: "__defaults should be filtered out",
			oldInputs: resource.PropertyMap{
				"normalProp": resource.NewProperty("value1"),
			},
			newInputs: resource.PropertyMap{
				"normalProp": resource.NewProperty("value1"),
				"__defaults": resource.NewProperty([]resource.PropertyValue{
					resource.NewProperty("default1"),
				}),
			},
			expectDiff:     false,
			shouldNotMatch: "__defaults",
		},
		{
			name: "normal property changes should still be shown",
			oldInputs: resource.PropertyMap{
				"normalProp": resource.NewProperty("value1"),
			},
			newInputs: resource.PropertyMap{
				"normalProp": resource.NewProperty("value2"),
			},
			expectDiff:  true,
			shouldMatch: "normalProp",
		},
		{
			name: "both normal and __defaults changes - only normal shown",
			oldInputs: resource.PropertyMap{
				"normalProp": resource.NewProperty("value1"),
			},
			newInputs: resource.PropertyMap{
				"normalProp": resource.NewProperty("value2"),
				"__defaults": resource.NewProperty([]resource.PropertyValue{
					resource.NewProperty("default1"),
				}),
			},
			expectDiff:     true,
			shouldMatch:    "normalProp",
			shouldNotMatch: "__defaults",
		},
		{
			name: "other internal properties should also be filtered",
			oldInputs: resource.PropertyMap{
				"normalProp": resource.NewProperty("value1"),
			},
			newInputs: resource.PropertyMap{
				"normalProp": resource.NewProperty("value1"),
				"__meta":     resource.NewProperty("metadata"),
			},
			expectDiff:     false,
			shouldNotMatch: "__meta",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			step := engine.StepEventMetadata{
				Op: deploy.OpUpdate,
				Old: &engine.StepEventStateMetadata{
					Inputs: tt.oldInputs,
				},
				New: &engine.StepEventStateMetadata{
					Inputs: tt.newInputs,
				},
			}

			result := getDiffInfo(step, apitype.UpdateUpdate)

			if tt.expectDiff {
				require.NotEmpty(t, result)
			}

			if tt.shouldMatch != "" {
				require.Contains(t, result, tt.shouldMatch)
			}

			if tt.shouldNotMatch != "" {
				require.NotContains(t, result, tt.shouldNotMatch)
			}

			// Verify that if there's a diff output, it doesn't contain any internal properties
			if result != "" && strings.Contains(result, "diff:") {
				require.NotContains(t, result, "__",
					"diff output should not contain any internal properties (starting with __): %s", result)
			}
		})
	}
}
