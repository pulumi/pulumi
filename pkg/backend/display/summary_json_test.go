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

package display

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makePreEvent is a small helper to keep the per-resource event tests readable.
func makePreEvent(op display.StepOp, urn resource.URN, newParent, oldParent resource.URN, internal bool,
) engine.ResourcePreEventPayload {
	meta := engine.StepEventMetadata{
		Op:  op,
		URN: urn,
	}
	if newParent != "" || op != deploy.OpDelete {
		meta.New = &engine.StepEventStateMetadata{URN: urn, Parent: newParent}
	}
	if oldParent != "" {
		meta.Old = &engine.StepEventStateMetadata{URN: urn, Parent: oldParent}
	}
	return engine.ResourcePreEventPayload{Metadata: meta, Internal: internal}
}

func TestSummaryJSONFromEvent(t *testing.T) {
	t.Parallel()

	payload := engine.SummaryEventPayload{
		Result:          apitype.OperationResultSucceeded,
		Duration:        7 * time.Second,
		ResourceChanges: display.ResourceChanges{"create": 2, "update": 1},
	}

	got := summaryJSONFromEvent(payload)

	assert.Equal(t, apitype.OperationResultSucceeded, got.Result)
	assert.Equal(t, 7*time.Second, got.Duration)
	assert.Equal(t, 2, got.Summary["create"])
	assert.Equal(t, 1, got.Summary["update"])
}

func TestWriteSummaryJSON(t *testing.T) {
	t.Parallel()

	s := SummaryJSON{
		Result:   apitype.OperationResultFailed,
		Duration: 3 * time.Second,
		Summary:  display.ResourceChanges{"delete": 1},
	}

	var buf bytes.Buffer
	require.NoError(t, writeSummaryJSON(&buf, s))

	// Output is a single line of JSON terminated by a newline.
	out := buf.String()
	assert.True(t, strings.HasSuffix(out, "\n"), "output should end with newline")
	assert.Equal(t, 1, strings.Count(out, "\n"), "output should be a single line")

	var roundTrip SummaryJSON
	require.NoError(t, json.Unmarshal([]byte(out), &roundTrip))
	assert.Equal(t, s, roundTrip)

	// The wire shape uses our preferred keys.
	assert.Contains(t, out, `"result":"failed"`)
	assert.Contains(t, out, `"summary":`)
	assert.NotContains(t, out, "changeSummary")
	assert.NotContains(t, out, "resourceChanges")
}

func TestTapSummaryJSON_EmitsOnSummaryEvent(t *testing.T) {
	t.Parallel()

	in := make(chan engine.Event, 2)
	in <- engine.NewEvent(engine.SummaryEventPayload{
		Result:          apitype.OperationResultSucceeded,
		Duration:        1 * time.Second,
		ResourceChanges: display.ResourceChanges{"same": 5},
	})
	close(in)

	var buf bytes.Buffer
	out := tapSummaryJSON(in, Options{Stdout: &buf})

	// Drain the output channel so the goroutine completes.
	for range out { //nolint:revive // intentional drain
	}

	var summary SummaryJSON
	require.NoError(t, json.Unmarshal(buf.Bytes(), &summary))
	assert.Equal(t, apitype.OperationResultSucceeded, summary.Result)
	assert.Equal(t, 1*time.Second, summary.Duration)
	assert.Equal(t, 5, summary.Summary["same"])
}

func TestResourceJSONFromEvent_SkipsInternal(t *testing.T) {
	t.Parallel()

	urn := resource.NewURN("dev", "myapp", "", "aws:s3/bucket:Bucket", "mybucket")
	payload := makePreEvent(deploy.OpCreate, urn, "parent-urn", "", true /* internal */)

	got := resourceJSONFromEvent(payload, false)
	assert.Nil(t, got, "internal events must not surface in the summary")
}

func TestResourceJSONFromEvent_SkipsSameByDefault(t *testing.T) {
	t.Parallel()

	urn := resource.NewURN("dev", "myapp", "", "aws:s3/bucket:Bucket", "mybucket")
	payload := makePreEvent(deploy.OpSame, urn, "parent-urn", "", false)

	got := resourceJSONFromEvent(payload, false /* showSames */)
	assert.Nil(t, got, "same resources are filtered out unless --show-sames is set")
}

func TestResourceJSONFromEvent_IncludesSameWhenShowSames(t *testing.T) {
	t.Parallel()

	urn := resource.NewURN("dev", "myapp", "", "aws:s3/bucket:Bucket", "mybucket")
	payload := makePreEvent(deploy.OpSame, urn, "parent-urn", "", false)

	got := resourceJSONFromEvent(payload, true /* showSames */)
	require.NotNil(t, got)
	assert.Equal(t, apitype.OpType("same"), got.Op)
}

func TestResourceJSONFromEvent_ExtractsTypeAndName(t *testing.T) {
	t.Parallel()

	urn := resource.NewURN("dev", "myapp", "", "aws:s3/bucket:Bucket", "mybucket")
	parentURN := resource.NewURN("dev", "myapp", "", "pulumi:pulumi:Stack", "myapp-dev")
	payload := makePreEvent(deploy.OpUpdate, urn, parentURN, "", false)

	got := resourceJSONFromEvent(payload, false)
	require.NotNil(t, got)
	assert.Equal(t, string(urn), got.URN)
	assert.Equal(t, "aws:s3/bucket:Bucket", got.Type)
	assert.Equal(t, "mybucket", got.Name)
	assert.Equal(t, apitype.OpType("update"), got.Op)
	assert.Equal(t, string(parentURN), got.Parent)
}

func TestResourceJSONFromEvent_ParentFallsBackToOldOnDelete(t *testing.T) {
	t.Parallel()

	// Deletes carry only the Old state — New is nil. The parent URN must still
	// be reported.
	urn := resource.NewURN("dev", "myapp", "", "aws:s3/bucket:Bucket", "mybucket")
	parentURN := resource.NewURN("dev", "myapp", "", "pulumi:pulumi:Stack", "myapp-dev")
	payload := engine.ResourcePreEventPayload{
		Metadata: engine.StepEventMetadata{
			Op:  deploy.OpDelete,
			URN: urn,
			Old: &engine.StepEventStateMetadata{URN: urn, Parent: parentURN},
			// New is nil for deletes.
		},
	}

	got := resourceJSONFromEvent(payload, false)
	require.NotNil(t, got)
	assert.Equal(t, string(parentURN), got.Parent)
	assert.Equal(t, apitype.OpType("delete"), got.Op)
}

func TestTapSummaryJSON_AccumulatesResources(t *testing.T) {
	t.Parallel()

	bucketURN := resource.NewURN("dev", "myapp", "", "aws:s3/bucket:Bucket", "mybucket")
	stackURN := resource.NewURN("dev", "myapp", "", "pulumi:pulumi:Stack", "myapp-dev")
	tableURN := resource.NewURN("dev", "myapp", "", "aws:dynamodb/table:Table", "mytable")

	in := make(chan engine.Event, 5)
	// Stack: surfaced (not internal, not same).
	in <- engine.NewEvent(makePreEvent(deploy.OpCreate, stackURN, "", "", false))
	// Bucket: surfaced with parent.
	in <- engine.NewEvent(makePreEvent(deploy.OpCreate, bucketURN, stackURN, "", false))
	// Table same: filtered out (showSames = false below).
	in <- engine.NewEvent(makePreEvent(deploy.OpSame, tableURN, stackURN, "", false))
	// Internal create: filtered out regardless.
	in <- engine.NewEvent(makePreEvent(deploy.OpCreate, bucketURN, stackURN, "", true))
	in <- engine.NewEvent(engine.SummaryEventPayload{
		Result:          apitype.OperationResultSucceeded,
		Duration:        2 * time.Second,
		ResourceChanges: display.ResourceChanges{"create": 2},
	})
	close(in)

	var buf bytes.Buffer
	out := tapSummaryJSON(in, Options{Stdout: &buf})
	for range out { //nolint:revive // intentional drain
	}

	var summary SummaryJSON
	require.NoError(t, json.Unmarshal(buf.Bytes(), &summary))

	require.Len(t, summary.Resources, 2, "expected stack + bucket; same and internal must be filtered")
	assert.Equal(t, string(stackURN), summary.Resources[0].URN)
	assert.Equal(t, apitype.OpType("create"), summary.Resources[0].Op)
	assert.Empty(t, summary.Resources[0].Parent)

	assert.Equal(t, string(bucketURN), summary.Resources[1].URN)
	assert.Equal(t, "aws:s3/bucket:Bucket", summary.Resources[1].Type)
	assert.Equal(t, "mybucket", summary.Resources[1].Name)
	assert.Equal(t, string(stackURN), summary.Resources[1].Parent)
}

func TestTapSummaryJSON_OmitsResourcesFieldWhenEmpty(t *testing.T) {
	t.Parallel()

	// No per-resource events: the summary JSON should not include a "resources"
	// key at all.
	in := make(chan engine.Event, 1)
	in <- engine.NewEvent(engine.SummaryEventPayload{
		Result:   apitype.OperationResultSucceeded,
		Duration: 1 * time.Second,
	})
	close(in)

	var buf bytes.Buffer
	out := tapSummaryJSON(in, Options{Stdout: &buf})
	for range out { //nolint:revive // intentional drain
	}

	assert.NotContains(t, buf.String(), `"resources"`)
}

func TestNewDiffJSON_ComputedInputsDiff(t *testing.T) {
	t.Parallel()

	m := engine.StepEventMetadata{
		Op: deploy.OpUpdate,
		Old: &engine.StepEventStateMetadata{Inputs: resource.PropertyMap{
			"region": resource.NewProperty("us-west-2"),
			"tags": resource.NewProperty(resource.PropertyMap{
				"env":  resource.NewProperty("dev"),
				"gone": resource.NewProperty("bye"),
			}),
			"password": resource.MakeSecret(resource.NewProperty("hunter2")),
		}},
		New: &engine.StepEventStateMetadata{Inputs: resource.PropertyMap{
			"region": resource.NewProperty("us-east-1"),
			"tags": resource.NewProperty(resource.PropertyMap{
				"env":  resource.NewProperty("prod"),
				"team": resource.NewProperty("infra"),
			}),
			"password": resource.MakeSecret(resource.NewProperty("hunter3")),
			"arn":      resource.MakeComputed(resource.NewProperty("")),
		}},
	}

	got := NewDiffJSON(&m, false, false)
	assert.Equal(t, map[string]PropertyDiffJSON{
		"region":    {Kind: "update", Old: "us-west-2", New: "us-east-1"},
		"tags.env":  {Kind: "update", Old: "dev", New: "prod"},
		"tags.gone": {Kind: "delete", Old: "bye"},
		"tags.team": {Kind: "add", New: "infra"},
		"password":  {Kind: "update", Old: "[secret]", New: "[secret]"},
		"arn":       {Kind: "add", New: "[unknown]"},
	}, got)

	// With showSecrets, secret values are revealed.
	withSecrets := NewDiffJSON(&m, false, true)
	assert.Equal(t, PropertyDiffJSON{Kind: "update", Old: "hunter2", New: "hunter3"}, withSecrets["password"])
}

func TestNewDiffJSON_CreateAndDelete(t *testing.T) {
	t.Parallel()

	inputs := resource.PropertyMap{
		"bucket": resource.NewProperty("my-bucket"),
		"tags":   resource.NewProperty(resource.PropertyMap{"env": resource.NewProperty("dev")}),
	}

	create := engine.StepEventMetadata{
		Op:  deploy.OpCreate,
		New: &engine.StepEventStateMetadata{Inputs: inputs},
	}
	assert.Equal(t, map[string]PropertyDiffJSON{
		"bucket": {Kind: "add", New: "my-bucket"},
		"tags":   {Kind: "add", New: map[string]any{"env": "dev"}},
	}, NewDiffJSON(&create, false, false))

	del := engine.StepEventMetadata{
		Op:  deploy.OpDelete,
		Old: &engine.StepEventStateMetadata{Inputs: inputs},
	}
	assert.Equal(t, map[string]PropertyDiffJSON{
		"bucket": {Kind: "delete", Old: "my-bucket"},
		"tags":   {Kind: "delete", Old: map[string]any{"env": "dev"}},
	}, NewDiffJSON(&del, false, false))
}

func TestNewDiffJSON_ArrayAndSameSkipped(t *testing.T) {
	t.Parallel()

	// OpSame never reports a property diff, even if the inputs differ.
	same := engine.StepEventMetadata{
		Op:  deploy.OpSame,
		Old: &engine.StepEventStateMetadata{Inputs: resource.PropertyMap{"a": resource.NewProperty("1")}},
		New: &engine.StepEventStateMetadata{Inputs: resource.PropertyMap{"a": resource.NewProperty("2")}},
	}
	assert.Nil(t, NewDiffJSON(&same, false, false))

	m := engine.StepEventMetadata{
		Op: deploy.OpUpdate,
		Old: &engine.StepEventStateMetadata{Inputs: resource.PropertyMap{
			"subnets": resource.NewProperty([]resource.PropertyValue{
				resource.NewProperty("a"),
				resource.NewProperty("b"),
			}),
		}},
		New: &engine.StepEventStateMetadata{Inputs: resource.PropertyMap{
			"subnets": resource.NewProperty([]resource.PropertyValue{
				resource.NewProperty("a"),
				resource.NewProperty("c"),
				resource.NewProperty("d"),
			}),
		}},
	}
	assert.Equal(t, map[string]PropertyDiffJSON{
		"subnets[1]": {Kind: "update", Old: "b", New: "c"},
		"subnets[2]": {Kind: "add", New: "d"},
	}, NewDiffJSON(&m, false, false))
}

func TestNewDiffJSON_DetailedDiff(t *testing.T) {
	t.Parallel()

	m := engine.StepEventMetadata{
		Op: deploy.OpUpdate,
		DetailedDiff: map[string]plugin.PropertyDiff{
			"tags.env":  {Kind: plugin.DiffUpdate},
			"tags.team": {Kind: plugin.DiffAdd},
		},
		Old: &engine.StepEventStateMetadata{Outputs: resource.PropertyMap{
			"tags": resource.NewProperty(resource.PropertyMap{"env": resource.NewProperty("dev")}),
		}},
		New: &engine.StepEventStateMetadata{Inputs: resource.PropertyMap{
			"tags": resource.NewProperty(resource.PropertyMap{
				"env":  resource.NewProperty("prod"),
				"team": resource.NewProperty("infra"),
			}),
		}},
	}

	assert.Equal(t, map[string]PropertyDiffJSON{
		"tags.env":  {Kind: "update", Old: "dev", New: "prod"},
		"tags.team": {Kind: "add", New: "infra"},
	}, NewDiffJSON(&m, false, false))
}

func TestNewDiffJSON_HiddenPathsExcluded(t *testing.T) {
	t.Parallel()

	m := engine.StepEventMetadata{
		Op: deploy.OpUpdate,
		Old: &engine.StepEventStateMetadata{Inputs: resource.PropertyMap{
			"visible": resource.NewProperty("1"),
			"hidden":  resource.NewProperty("1"),
		}},
		New: &engine.StepEventStateMetadata{
			Inputs: resource.PropertyMap{
				"visible": resource.NewProperty("2"),
				"hidden":  resource.NewProperty("2"),
			},
			HideDiffs: []resource.PropertyPath{{"hidden"}},
		},
	}

	assert.Equal(t, map[string]PropertyDiffJSON{
		"visible": {Kind: "update", Old: "1", New: "2"},
	}, NewDiffJSON(&m, false, false))
}

// makeDiffPreEvent builds a pre-event carrying old/new inputs so the tap can
// compute a property diff.
func makeDiffPreEvent(op display.StepOp, urn resource.URN, old, new resource.PropertyMap,
) engine.ResourcePreEventPayload {
	meta := engine.StepEventMetadata{Op: op, URN: urn}
	if old != nil {
		meta.Old = &engine.StepEventStateMetadata{URN: urn, Inputs: old}
	}
	if new != nil {
		meta.New = &engine.StepEventStateMetadata{URN: urn, Inputs: new}
	}
	return engine.ResourcePreEventPayload{Metadata: meta}
}

func TestTapSummaryJSON_IncludesDiffOnlyInDiffMode(t *testing.T) {
	t.Parallel()

	urn := resource.NewURN("dev", "myapp", "", "aws:s3/bucket:Bucket", "mybucket")
	makeEvents := func() chan engine.Event {
		in := make(chan engine.Event, 2)
		in <- engine.NewEvent(makeDiffPreEvent(deploy.OpUpdate, urn,
			resource.PropertyMap{"a": resource.NewProperty("1")},
			resource.PropertyMap{"a": resource.NewProperty("2")}))
		in <- engine.NewEvent(engine.SummaryEventPayload{Result: apitype.OperationResultSucceeded})
		close(in)
		return in
	}

	// Default (progress) mode: no diff key at all.
	var plain bytes.Buffer
	for range tapSummaryJSON(makeEvents(), Options{Stdout: &plain}) { //nolint:revive // intentional drain
	}
	assert.NotContains(t, plain.String(), `"diff"`)

	// Diff mode: the changed property is reported.
	var buf bytes.Buffer
	for range tapSummaryJSON(makeEvents(), Options{Stdout: &buf, Type: DisplayDiff}) { //nolint:revive // drain
	}
	var summary SummaryJSON
	require.NoError(t, json.Unmarshal(buf.Bytes(), &summary))
	require.Len(t, summary.Resources, 1)
	assert.Equal(t, map[string]PropertyDiffJSON{
		"a": {Kind: "update", Old: "1", New: "2"},
	}, summary.Resources[0].Diff)
}

func TestTapSummaryJSON_RefreshDiffArrivesOnOutputsEvent(t *testing.T) {
	t.Parallel()

	urn := resource.NewURN("dev", "myapp", "", "aws:s3/bucket:Bucket", "mybucket")
	inputs := resource.PropertyMap{"acl": resource.NewProperty("private")}

	// The pre-event announces the refresh with identical old/new state; the
	// outputs event carries the update discovered by the provider read.
	in := make(chan engine.Event, 3)
	in <- engine.NewEvent(makeDiffPreEvent(deploy.OpRefresh, urn, inputs, inputs))
	in <- engine.NewEvent(engine.ResourceOutputsEventPayload{
		Metadata: engine.StepEventMetadata{
			Op:           deploy.OpUpdate,
			URN:          urn,
			DetailedDiff: map[string]plugin.PropertyDiff{"acl": {Kind: plugin.DiffUpdate}},
			Old:          &engine.StepEventStateMetadata{URN: urn, Outputs: inputs},
			New: &engine.StepEventStateMetadata{
				URN:     urn,
				Inputs:  inputs,
				Outputs: resource.PropertyMap{"acl": resource.NewProperty("public-read")},
			},
		},
	})
	in <- engine.NewEvent(engine.SummaryEventPayload{Result: apitype.OperationResultSucceeded})
	close(in)

	var buf bytes.Buffer
	for range tapSummaryJSON(in, Options{Stdout: &buf, Type: DisplayDiff}) { //nolint:revive // intentional drain
	}

	var summary SummaryJSON
	require.NoError(t, json.Unmarshal(buf.Bytes(), &summary))
	require.Len(t, summary.Resources, 1)
	assert.Equal(t, apitype.OpRefresh, summary.Resources[0].Op)
	assert.Equal(t, map[string]PropertyDiffJSON{
		"acl": {Kind: "update", Old: "private", New: "public-read"},
	}, summary.Resources[0].Diff)
}

func TestTapSummaryJSON_ReturnsOnCancelEvent(t *testing.T) {
	t.Parallel()

	// The input channel is intentionally never closed: the tap must terminate
	// on CancelEvent alone. startEventLogger never closes its output channel,
	// so a tap that waits for closure deadlocks the whole display pipeline.
	in := make(chan engine.Event, 1)
	in <- engine.NewCancelEvent()

	out := tapSummaryJSON(in, Options{Stdout: io.Discard})

	done := make(chan struct{})
	go func() {
		for range out { //nolint:revive // intentional drain
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("tap did not return after CancelEvent")
	}
}
