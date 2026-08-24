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

package display_test

import (
	"context"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/util/fsa"
	"github.com/pulumi/pulumi/pkg/v3/workflow"
	"github.com/pulumi/pulumi/pkg/v3/workflow/display"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

func identity(_ context.Context, _ workflow.Workflow, _ workflow.Cursor, in property.Map) (property.Map, error) {
	return in, nil
}

func always(context.Context, workflow.Workflow, workflow.Cursor, property.Map) (bool, workflow.Overlay, error) {
	return true, workflow.Overlay{}, nil
}

func flag(key string) workflow.EdgeFunc {
	return func(_ context.Context, _ workflow.Workflow, _ workflow.Cursor, in property.Map) (
		bool, workflow.Overlay, error,
	) {
		v := in.Get(key)
		return v.IsBool() && v.AsBool(), workflow.Overlay{}, nil
	}
}

func TestRender(t *testing.T) {
	t.Parallel()

	w := workflow.New()
	ci := w.NewNode("ci", identity)
	dev := w.NewNode("dev", identity)
	staging := w.NewNode("staging", identity)
	prod := w.NewNode("prod", identity)
	w.NewEdge("nightly", ci, dev, always)
	w.NewEdge("smoke tests", dev, staging, flag("smoke"))
	w.NewAndEdge("prod gate", staging, prod, map[string]workflow.EdgeFunc{
		"manual approval": flag("approved"),
		"smoke tests":     always,
	})
	w.NewOrEdge("rollback", prod, dev, map[string]workflow.EdgeFunc{
		"break glass":             flag("break"),
		"post deploy smoke tests": flag("bad"),
	})
	w.AddCursor(ci, "release#4", property.Map{})
	w.AddCursor(dev, "release#3", property.NewMap(map[string]property.Value{
		"smoke":    property.New(true),
		"approved": property.New(true),
	}))

	var events []workflow.WorkflowUpdate
	updates := make(chan workflow.WorkflowUpdate)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for u := range updates {
			events = append(events, u)
		}
	}()
	require.NoError(t, w.Progress(t.Context(), fsa.SyncRunner, updates))
	close(updates)
	<-done

	m := display.New()
	applied := 0
	// Apply events through the first one matching pred, then render.
	renderThrough := func(pred func(workflow.WorkflowUpdate) bool) string {
		i := slices.IndexFunc(events[applied:], pred)
		require.NotEqual(t, -1, i)
		for _, u := range events[applied : applied+i+1] {
			m.Apply(u)
		}
		applied += i + 1
		return m.Render()
	}

	assert.Equal(t, `workflow: release#4 @ dev — running dev; release#3 @ dev — parked
  ci
    └─ nightly ─▶ dev
  dev ◀ release#4, release#3
    └─ smoke tests ─▶ staging
  staging
    └─ prod gate (all: manual approval, smoke tests) ─▶ prod
  prod
    └─ rollback (any: break glass, post deploy smoke tests) ─▶ dev
cursors
  release#4 @ dev — running dev
  release#3 @ dev — parked
`, renderThrough(func(u workflow.WorkflowUpdate) bool {
		_, ok := u.(workflow.NodeStarted)
		return ok
	}))

	assert.Equal(t, `workflow: release#4 @ dev — parked; release#3 @ staging — checking prod gate/manual approval
  ci
    └─ nightly ─▶ dev
  dev ◀ release#4
    └─ smoke tests ─▶ staging
  staging ◀ release#3
    └─ prod gate (all: manual approval, smoke tests) ─▶ prod
  prod
    └─ rollback (any: break glass, post deploy smoke tests) ─▶ dev
cursors
  release#4 @ dev — parked
  release#3 @ staging — checking prod gate/manual approval
`, renderThrough(func(u workflow.WorkflowUpdate) bool {
		e, ok := u.(workflow.EdgeStarted)
		return ok && e.Condition != ""
	}))

	assert.Equal(t, `workflow: release#4 @ dev — parked; release#3 @ prod — parked
  ci
    └─ nightly ─▶ dev
  dev ◀ release#4
    └─ smoke tests ─▶ staging
  staging
    └─ prod gate (all: manual approval, smoke tests) ─▶ prod
  prod ◀ release#3
    └─ rollback (any: break glass, post deploy smoke tests) ─▶ dev
cursors
  release#4 @ dev — parked
  release#3 @ prod — parked
`, renderThrough(func(u workflow.WorkflowUpdate) bool {
		_, ok := u.(workflow.NodeUntouched)
		return ok
	}))
	require.Len(t, events, applied)
}
