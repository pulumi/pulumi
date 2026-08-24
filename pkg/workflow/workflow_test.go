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

package workflow_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/util/fsa"
	"github.com/pulumi/pulumi/pkg/v3/workflow"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

// A recorder drives a workflow with the sync runner and records every event, interleaved with test
// annotations, for comparison against a golden file.
type recorder struct {
	t      *testing.T
	w      workflow.Workflow
	events []recorded
}

type recorded struct {
	Type  string `json:"type"`
	Event any    `json:"event,omitempty"`
	Note  string `json:"note,omitempty"`
	Error string `json:"error,omitempty"`
}

func (r *recorder) note(note string) {
	r.events = append(r.events, recorded{Type: "Note", Note: note})
}

// Run Progress once, recording its events and its result.
func (r *recorder) progress() error {
	updates := make(chan workflow.WorkflowUpdate)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for u := range updates {
			r.events = append(r.events, recorded{Type: strings.TrimPrefix(fmt.Sprintf("%T", u), "workflow."), Event: u})
		}
	}()
	err := r.w.Progress(r.t.Context(), fsa.SyncRunner, updates)
	close(updates)
	<-done
	rec := recorded{Type: "ProgressReturned"}
	if err != nil {
		rec.Error = err.Error()
	}
	r.events = append(r.events, rec)
	return err
}

func (r *recorder) golden(name string) {
	b, err := json.MarshalIndent(r.events, "", "  ")
	require.NoError(r.t, err)
	golden(r.t, name, b)
}

func golden(t *testing.T, name string, got []byte) {
	path := filepath.Join("testdata", name+".json")
	if cmdutil.IsTruthy(os.Getenv("PULUMI_ACCEPT")) {
		require.NoError(t, os.WriteFile(path, got, 0o600))
		return
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err, "run with PULUMI_ACCEPT=1 to create the golden file")
	// A Windows checkout with autocrlf leaves the golden with CRLF line endings.
	want = bytes.ReplaceAll(want, []byte("\r\n"), []byte("\n"))
	require.Equal(t, string(want), string(got))
}

func str(s string) property.Value                   { return property.New(s) }
func num(n float64) property.Value                  { return property.New(n) }
func boolean(b bool) property.Value                 { return property.New(b) }
func pmap(m map[string]property.Value) property.Map { return property.NewMap(m) }

func mustNode(t *testing.T, w workflow.Workflow, id string) workflow.Node {
	n, ok := w.NodeByID(id)
	require.True(t, ok, "node %q is not defined", id)
	return n
}

// An edge condition that sets nothing on the cursor.
func cond(f func(in property.Map) (bool, error)) workflow.EdgeFunc {
	return func(_ context.Context, _ workflow.Workflow, _ workflow.Cursor, in property.Map) (
		bool, workflow.Overlay, error,
	) {
		pass, err := f(in)
		return pass, workflow.Overlay{}, err
	}
}

func always(context.Context, workflow.Workflow, workflow.Cursor, property.Map) (bool, workflow.Overlay, error) {
	return true, workflow.Overlay{}, nil
}

// Passes when the value at key is a bool that is true.
func flag(key string) workflow.EdgeFunc {
	return cond(func(in property.Map) (bool, error) {
		v := in.Get(key)
		return v.IsBool() && v.AsBool(), nil
	})
}

func not(f workflow.EdgeFunc) workflow.EdgeFunc {
	return func(ctx context.Context, w workflow.Workflow, c workflow.Cursor, in property.Map) (
		bool, workflow.Overlay, error,
	) {
		ok, out, err := f(ctx, w, c, in)
		return !ok, out, err
	}
}

// Passes when *v is true; the test flips v to simulate the world changing between Progress calls.
func external(v *bool) workflow.EdgeFunc {
	return cond(func(property.Map) (bool, error) { return *v, nil })
}

// A node that merges extra keys into its inputs.
func with(extra map[string]property.Value) workflow.NodeFunc {
	return func(_ context.Context, _ workflow.Workflow, _ workflow.Cursor, in property.Map) (property.Map, error) {
		out := in.AsMap()
		maps.Copy(out, extra)
		return property.NewMap(out), nil
	}
}

func conds(m map[string]workflow.EdgeFunc) map[string]workflow.EdgeFunc { return m }

// A blue/green deployment: deploy the green stack, verify it, wait for a human to approve the traffic
// switch, then decommission blue. An unhealthy green stack (or an approval that times out) rolls back.
func TestBlueGreen(t *testing.T) {
	t.Parallel()
	var approved, timedOut bool
	w := workflow.New()

	// AddCursor does not run the node it places the cursor on, so releases start on a node of their own.
	requested := w.NewNode("release-requested", with(nil))
	deploy := w.NewNode("deploy-green", func(_ context.Context, _ workflow.Workflow, _ workflow.Cursor, in property.Map) (
		property.Map, error,
	) {
		return pmap(map[string]property.Value{
			"version": in.Get("version"),
			"url":     str("https://green.example.com/" + in.Get("version").AsString()),
		}), nil
	})
	verify := w.NewNode("verify-green", func(_ context.Context, _ workflow.Workflow, _ workflow.Cursor, in property.Map) (
		property.Map, error,
	) {
		// Only version 2.x is healthy in this simulation.
		healthy := strings.HasPrefix(in.Get("version").AsString(), "2.")
		return in.Set("healthy", boolean(healthy)), nil
	})
	switchTraffic := w.NewNode("switch-traffic", with(map[string]property.Value{"live": str("green")}))
	decommission := w.NewNode("decommission-blue", with(map[string]property.Value{"blue": str("deleted")}))
	rollback := w.NewNode("rollback", with(map[string]property.Value{"live": str("blue")}))

	w.NewEdge("start", requested, deploy, always)
	w.NewEdge("deployed", deploy, verify, always)
	w.NewAndEdge("healthy-and-approved", verify, switchTraffic, conds(map[string]workflow.EdgeFunc{
		"healthy": flag("healthy"), "approved": external(&approved),
	}))
	w.NewOrEdge("unhealthy-or-timeout", verify, rollback, conds(map[string]workflow.EdgeFunc{
		"unhealthy": not(flag("healthy")), "timeout": external(&timedOut),
	}))
	w.NewEdge("switched", switchTraffic, decommission, always)

	r := &recorder{t: t, w: w}
	w.AddCursor(requested, "release 2.0", pmap(map[string]property.Value{"version": str("2.0")}))
	require.NoError(t, r.progress())
	r.note("operator approves the traffic switch")
	approved = true
	require.NoError(t, r.progress())

	r.note("release 2.0 done; a broken release 3.0 starts")
	approved = false
	w.AddCursor(requested, "release 3.0", pmap(map[string]property.Value{"version": str("3.0")}))
	require.NoError(t, r.progress())

	r.note("release 3.1 stalls waiting for approval until the approval window times out")
	w.AddCursor(requested, "release 3.1", pmap(map[string]property.Value{"version": str("3.1")}))
	require.NoError(t, r.progress())
	timedOut = true
	require.NoError(t, r.progress())

	assert.Equal(t, pmap(map[string]property.Value{
		"version": str("2.0"),
		"url":     str("https://green.example.com/2.0"),
		"healthy": boolean(true),
		"live":    str("green"),
		"blue":    str("deleted"),
	}), w.GetState(decommission).Outputs)
	r.golden("blue-green")
}

// A phased rollout: a planning node fans out one cursor per US region, each region deploys and verifies,
// and only once every US region's metrics look good (a join) does the EU deployment start.
type rollout struct {
	metricsGood map[string]*bool // region -> whether its post-deploy metrics are acceptable
	broken      map[string]bool  // region -> whether deploying there fails
	usRegions   []string
	merge       workflow.MergeFunc
}

func newRollout() *rollout {
	return &rollout{
		metricsGood: map[string]*bool{"us-west": new(bool), "us-east": new(bool), "eu": new(bool)},
		broken:      map[string]bool{},
		usRegions:   []string{"us-west", "us-east"},
		merge:       nestByFrom,
	}
}

// A merge that nests each candidate's inputs under its From node's ID.
func nestByFrom(_ context.Context, candidates []workflow.MergeCandidate) (bool, workflow.MergedCursor, error) {
	merged := map[string]property.Value{}
	for _, c := range candidates {
		merged[c.From.ID()] = property.New(c.Inputs)
	}
	return true, workflow.MergedCursor{Label: "merged", Inputs: property.NewMap(merged)}, nil
}

func (ro *rollout) define(w workflow.Workflow) (start, done workflow.Node) {
	queue := map[string]workflow.Node{} // Where plan places each region's cursor; AddCursor does not run the node
	deploy := map[string]workflow.Node{}
	verify := map[string]workflow.Node{}
	for _, region := range append(ro.usRegions, "eu") {
		queue[region] = w.NewNode("queue-"+region, with(nil))
		deploy[region] = w.NewNode("deploy-"+region, func(
			ctx context.Context, w workflow.Workflow, c workflow.Cursor, in property.Map,
		) (property.Map, error) {
			if ro.broken[region] {
				return property.Map{}, fmt.Errorf("deploy to %s failed", region)
			}
			return with(map[string]property.Value{"region": str(region)})(ctx, w, c, in)
		})
		w.NewEdge("queued", queue[region], deploy[region], always)
		verify[region] = w.NewNode("verify-"+region, with(map[string]property.Value{"verified": boolean(true)}))
		w.NewEdge("deployed", deploy[region], verify[region], always)
	}
	start = w.NewNode("start", with(nil))
	plan := w.NewNode("plan", func(
		_ context.Context, w workflow.Workflow, _ workflow.Cursor, in property.Map,
	) (property.Map, error) {
		for _, region := range ro.usRegions {
			w.AddCursor(queue[region], region, in)
		}
		return in.Set("phase", num(1)), nil
	})
	w.NewEdge("planning", start, plan, always)
	branches := make([]workflow.JoinEdgeArg, 0, len(ro.usRegions))
	for _, region := range ro.usRegions {
		branches = append(branches, workflow.JoinEdgeArg{From: verify[region], Edge: external(ro.metricsGood[region])})
	}
	w.NewJoinEdge("us-metrics-good", branches, deploy["eu"], ro.merge)
	done = w.NewNode("done", with(nil))
	w.NewEdge("eu-metrics-good", verify["eu"], done, external(ro.metricsGood["eu"]))
	return start, done
}

func TestPhasedRollout(t *testing.T) {
	t.Parallel()
	ro := newRollout()
	w := workflow.New()
	start, done := ro.define(w)
	r := &recorder{t: t, w: w}

	w.AddCursor(start, "v7", pmap(map[string]property.Value{"version": str("7")}))
	require.NoError(t, r.progress())
	r.note("us-west metrics look good")
	*ro.metricsGood["us-west"] = true
	require.NoError(t, r.progress())
	r.note("us-east metrics look good")
	*ro.metricsGood["us-east"] = true
	require.NoError(t, r.progress())
	r.note("eu metrics look good")
	*ro.metricsGood["eu"] = true
	require.NoError(t, r.progress())

	assert.Equal(t, pmap(map[string]property.Value{
		"verify-us-west": property.New(pmap(map[string]property.Value{
			"version": str("7"), "region": str("us-west"), "verified": boolean(true),
		})),
		"verify-us-east": property.New(pmap(map[string]property.Value{
			"version": str("7"), "region": str("us-east"), "verified": boolean(true),
		})),
		"region":   str("eu"),
		"verified": boolean(true),
	}), w.GetState(done).Outputs)
	r.golden("phased-rollout")
}

// The phased rollout, saved to State while us-west waits on the join and resumed in a fresh workflow.
func TestPhasedRolloutResume(t *testing.T) {
	t.Parallel()
	ro := newRollout()
	w := workflow.New()
	start, _ := ro.define(w)
	r := &recorder{t: t, w: w}
	w.AddCursor(start, "v7", pmap(map[string]property.Value{"version": str("7")}))
	require.NoError(t, r.progress())
	*ro.metricsGood["us-west"] = true
	require.NoError(t, r.progress())

	state := w.State()
	golden(t, "phased-rollout-state", state)

	resumed, err := workflow.FromState(state)
	require.NoError(t, err)
	r2 := &recorder{t: t, w: resumed}
	require.NoError(t, r2.progress()) // Reports the undefined nodes; nothing else happens
	ro.define(resumed)

	r2.note("resumed; us-east metrics look good")
	*ro.metricsGood["us-east"] = true
	require.NoError(t, r2.progress())
	r2.note("eu metrics look good")
	*ro.metricsGood["eu"] = true
	require.NoError(t, r2.progress())
	r2.golden("phased-rollout-resume")

	// Driving the original workflow to completion ends in the same state.
	require.NoError(t, r.progress())
	require.NoError(t, r.progress())
	// Restored cursors are re-created in node definition order, so State lists them in a different order.
	var before, after map[string]any
	require.NoError(t, json.Unmarshal(w.State(), &before))
	require.NoError(t, json.Unmarshal(resumed.State(), &after))
	assert.Equal(t, before["nextCursor"], after["nextCursor"])
	assert.ElementsMatch(t, before["cursors"], after["cursors"])
}

func TestNodeFailure(t *testing.T) {
	t.Parallel()
	w := workflow.New()
	start := w.NewNode("start", with(nil))
	boom := w.NewNode("boom", func(
		context.Context, workflow.Workflow, workflow.Cursor, property.Map,
	) (property.Map, error) {
		return property.Map{}, errors.New("deploy exploded")
	})
	w.NewEdge("go", start, boom, always)
	r := &recorder{t: t, w: w}
	w.AddCursor(start, "", property.Map{})
	err := r.progress()
	assert.EqualError(t, err, "deploy exploded")
	r.golden("node-failure")
}

func TestFromStateRejectsInvalid(t *testing.T) {
	t.Parallel()
	_, err := workflow.FromState([]byte(`{"nextCursor": 1, "cursors": [], "bogus": true}`))
	assert.EqualError(t, err, `invalid workflow state: json: unknown field "bogus"`)
	_, err = workflow.FromState([]byte(`not json`))
	assert.ErrorContains(t, err, "invalid workflow state")
}

// The join must resolve correctly when branches are evaluated concurrently.
func TestPhasedRolloutAsync(t *testing.T) {
	t.Parallel()
	for range 50 {
		ro := newRollout()
		for _, good := range ro.metricsGood {
			*good = true
		}
		w := workflow.New()
		start, done := ro.define(w)
		w.AddCursor(start, "v7", pmap(map[string]property.Value{"version": str("7")}))
		var wg sync.WaitGroup
		runner := func(ctx context.Context, f func(context.Context)) error {
			wg.Go(func() { f(ctx) })
			return nil
		}
		require.NoError(t, w.Progress(t.Context(), runner, nil))
		wg.Wait()
		assert.Equal(t, str("eu"), w.GetState(done).Outputs.Get("region"))
	}
}

// A canary: traffic shifts 10% → 50% → 100% through a loop, each step waiting on the metrics observed at
// that traffic level; bad metrics at any level roll back.
func TestCanaryLoop(t *testing.T) {
	t.Parallel()
	metrics := map[float64]string{} // traffic percent -> "" (not yet observed), "good", or "bad"
	steps := []float64{10, 50, 100}
	w := workflow.New()

	start := w.NewNode("start", with(nil))
	shift := w.NewNode("shift-traffic", func(_ context.Context, _ workflow.Workflow, _ workflow.Cursor, in property.Map) (
		property.Map, error,
	) {
		percent := steps[0]
		if in.Get("percent").IsNumber() {
			percent = steps[slices.Index(steps, in.Get("percent").AsNumber())+1]
		}
		return in.Set("percent", num(percent)), nil
	})
	observe := w.NewNode("observe", with(nil))
	done := w.NewNode("done", with(nil))
	rollback := w.NewNode("rollback", with(map[string]property.Value{"percent": num(0)}))

	metricsAre := func(want string) workflow.EdgeFunc {
		return cond(func(in property.Map) (bool, error) {
			return metrics[in.Get("percent").AsNumber()] == want, nil
		})
	}
	fullyShifted := cond(func(in property.Map) (bool, error) {
		return in.Get("percent").AsNumber() == 100, nil
	})
	w.NewEdge("start", start, shift, always)
	w.NewEdge("shifted", shift, observe, always)
	w.NewAndEdge("healthy-and-more-to-shift", observe, shift, conds(map[string]workflow.EdgeFunc{
		"healthy": metricsAre("good"), "more-to-shift": not(fullyShifted),
	}))
	w.NewAndEdge("healthy-and-fully-shifted", observe, done, conds(map[string]workflow.EdgeFunc{
		"healthy": metricsAre("good"), "fully-shifted": fullyShifted,
	}))
	w.NewEdge("unhealthy", observe, rollback, metricsAre("bad"))

	r := &recorder{t: t, w: w}
	w.AddCursor(start, "canary", pmap(map[string]property.Value{"version": str("8")}))
	require.NoError(t, r.progress())
	r.note("10% looks good")
	metrics[10] = "good"
	require.NoError(t, r.progress())
	r.note("50% looks good")
	metrics[50] = "good"
	require.NoError(t, r.progress())
	r.note("100% looks bad")
	metrics[100] = "bad"
	require.NoError(t, r.progress())

	assert.Equal(t, pmap(map[string]property.Value{"version": str("8"), "percent": num(0)}),
		w.GetState(rollback).Outputs)
	r.golden("canary-loop")
}

// A flaky deploy retried up to maxAttempts times. A failure that should be retried is a node output, not
// an error: a node error aborts the whole run.
func TestRetry(t *testing.T) {
	t.Parallel()
	w := workflow.New()
	start := w.NewNode("start", with(nil))
	deploy := w.NewNode("deploy", func(
		_ context.Context, _ workflow.Workflow, _ workflow.Cursor, in property.Map,
	) (property.Map, error) {
		attempts := float64(1)
		if in.Get("attempts").IsNumber() {
			attempts = in.Get("attempts").AsNumber() + 1
		}
		// The simulated deploy succeeds on its third attempt.
		return in.Set("attempts", num(attempts)).Set("ok", boolean(attempts >= 3)), nil
	})
	done := w.NewNode("done", with(nil))
	giveUp := w.NewNode("give-up", with(nil))

	exhausted := cond(func(in property.Map) (bool, error) {
		return in.Get("attempts").AsNumber() >= in.Get("maxAttempts").AsNumber(), nil
	})
	w.NewEdge("start", start, deploy, always)
	w.NewEdge("succeeded", deploy, done, flag("ok"))
	w.NewAndEdge("retry", deploy, deploy, conds(map[string]workflow.EdgeFunc{
		"failed": not(flag("ok")), "attempts-left": not(exhausted),
	}))
	w.NewAndEdge("exhausted", deploy, giveUp, conds(map[string]workflow.EdgeFunc{
		"failed": not(flag("ok")), "exhausted": exhausted,
	}))

	r := &recorder{t: t, w: w}
	w.AddCursor(start, "three attempts", pmap(map[string]property.Value{"maxAttempts": num(3)}))
	require.NoError(t, r.progress())
	r.note("a second release only gets two attempts")
	w.AddCursor(start, "two attempts", pmap(map[string]property.Value{"maxAttempts": num(2)}))
	require.NoError(t, r.progress())

	assert.Equal(t, num(3), w.GetState(done).Outputs.Get("attempts"))
	assert.Equal(t, num(2), w.GetState(giveUp).Outputs.Get("attempts"))
	r.golden("retry")
}

// Releases contend for a single soak environment. A release that arrives while another is soaking
// replaces it if the incumbent cannot move on; an incumbent that can move on gets to first.
func TestContendedEnvironment(t *testing.T) {
	t.Parallel()
	soaked := map[string]bool{}
	w := workflow.New()
	queue := w.NewNode("queue", with(nil))
	staging := w.NewNode("staging", with(map[string]property.Value{"env": str("staging")}))
	soak := w.NewNode("soak", with(nil))
	prod := w.NewNode("prod", with(map[string]property.Value{"env": str("prod")}))
	w.NewEdge("deploy", queue, staging, always)
	w.NewEdge("deployed", staging, soak, always)
	w.NewEdge("soaked", soak, prod, cond(func(in property.Map) (bool, error) {
		return soaked[in.Get("version").AsString()], nil
	}))
	release := func(v string) property.Map { return pmap(map[string]property.Value{"version": str(v)}) }

	r := &recorder{t: t, w: w}
	w.AddCursor(queue, "A", release("A"))
	require.NoError(t, r.progress())
	r.note("B arrives while A is still soaking: A is superseded")
	w.AddCursor(queue, "B", release("B"))
	require.NoError(t, r.progress())
	r.note("A finishes soaking, but it is gone")
	soaked["A"] = true
	require.NoError(t, r.progress())
	r.note("B finishes soaking as C arrives: B moves on before C takes the soak environment")
	soaked["B"] = true
	w.AddCursor(queue, "C", release("C"))
	require.NoError(t, r.progress())

	golden(t, "contended-environment-state", w.State())
	assert.Equal(t, str("B"), w.GetState(prod).Outputs.Get("version"))
	r.golden("contended-environment")
}

// A second rollout through the same join. The cursors merged away by the first rollout linger on their
// verify nodes until the second rollout's cursors overwrite them; that is not a second end for them, so
// no CursorReplaced is reported.
func TestJoinReused(t *testing.T) {
	t.Parallel()
	ro := newRollout()
	for _, good := range ro.metricsGood {
		*good = true
	}
	w := workflow.New()
	start, _ := ro.define(w)
	r := &recorder{t: t, w: w}
	w.AddCursor(start, "v7", pmap(map[string]property.Value{"version": str("7")}))
	require.NoError(t, r.progress())
	r.note("v8 follows v7 through the same graph")
	w.AddCursor(start, "v8", pmap(map[string]property.Value{"version": str("8")}))
	require.NoError(t, r.progress())
	r.golden("join-reused")
}

// A hotfix for us-west arrives while us-west v7 is waiting at verify-us-west for us-east. The hotfix
// replaces the waiting v7 cursor; the join then merges the hotfix with us-east v7.
func TestJoinBranchRetaken(t *testing.T) {
	t.Parallel()
	ro := newRollout()
	w := workflow.New()
	start, done := ro.define(w)
	r := &recorder{t: t, w: w}
	w.AddCursor(start, "v7", pmap(map[string]property.Value{"version": str("7")}))
	*ro.metricsGood["us-west"] = true
	require.NoError(t, r.progress())
	r.note("us-west needs a hotfix before us-east's metrics come in")
	w.AddCursor(mustNode(t, w, "queue-us-west"), "us-west hotfix", pmap(map[string]property.Value{"version": str("7.1")}))
	require.NoError(t, r.progress())
	r.note("us-east and eu metrics look good")
	*ro.metricsGood["us-east"] = true
	*ro.metricsGood["eu"] = true
	require.NoError(t, r.progress())

	assert.Equal(t, str("7.1"), w.GetState(done).Outputs.Get("verify-us-west").AsMap().Get("version"))
	assert.Equal(t, str("7"), w.GetState(done).Outputs.Get("verify-us-east").AsMap().Get("version"))
	r.golden("join-branch-retaken")
}

// The graph is grown while it runs: each deploy node defines the next region's node and the edge to
// it from the cursor's inputs.
func TestDynamicGraph(t *testing.T) {
	t.Parallel()
	w := workflow.New()
	done := w.NewNode("done", with(nil))
	var expand func(context.Context, workflow.Workflow, workflow.Cursor, property.Map) (property.Map, error)
	expand = func(_ context.Context, w workflow.Workflow, _ workflow.Cursor, in property.Map) (property.Map, error) {
		remaining := in.Get("remaining").AsArray().AsSlice()
		from, ok := w.NodeByID(in.Get("here").AsString())
		require.True(t, ok)
		if len(remaining) == 0 {
			w.NewEdge("finished", from, done, always)
			return in, nil
		}
		next := "deploy-" + remaining[0].AsString()
		w.NewEdge("next", from, w.NewNode(next, expand), always)
		return in.Set("remaining", property.New(remaining[1:])).Set("here", str(next)), nil
	}
	start := w.NewNode("start", with(nil))
	plan := w.NewNode("plan", expand)
	w.NewEdge("planning", start, plan, always)

	r := &recorder{t: t, w: w}
	w.AddCursor(start, "expanding", pmap(map[string]property.Value{
		"here":      str("plan"),
		"remaining": property.New([]property.Value{str("us"), str("eu")}),
	}))
	require.NoError(t, r.progress())
	assert.Equal(t, str("deploy-eu"), w.GetState(done).Outputs.Get("here"))
	r.golden("dynamic-graph")
}

// One release's node fails while another release is mid-move. The failure aborts the run, but the
// bystander's node function succeeded, so its move is committed and the next Progress carries on from
// there rather than running the node again.
func TestNodeFailureWithBystander(t *testing.T) {
	t.Parallel()
	broken := true
	w := workflow.New()
	// Two releases at one node that both move at once is a race, so each release has its own start.
	startBad := w.NewNode("start-bad", with(nil))
	startGood := w.NewNode("start-good", with(nil))
	deploy := w.NewNode("deploy", func(
		_ context.Context, _ workflow.Workflow, _ workflow.Cursor, in property.Map,
	) (property.Map, error) {
		if broken {
			return property.Map{}, errors.New("deploy exploded")
		}
		return in.Set("deployed", boolean(true)), nil
	})
	build := w.NewNode("build", with(map[string]property.Value{"built": boolean(true)}))
	done := w.NewNode("done", with(nil))
	w.NewEdge("go", startBad, deploy, always)
	w.NewEdge("go", startGood, build, always)
	w.NewEdge("built", build, done, always)

	r := &recorder{t: t, w: w}
	w.AddCursor(startBad, "bad", pmap(map[string]property.Value{"version": str("bad")}))
	w.AddCursor(startGood, "good", pmap(map[string]property.Value{"version": str("good")}))
	assert.EqualError(t, r.progress(), "deploy exploded")
	golden(t, "node-failure-bystander-state", w.State())
	r.note("the bad release is fixed")
	broken = false
	require.NoError(t, r.progress())
	r.golden("node-failure-bystander")
}

// An edge condition that errors aborts the run; once it stops erroring the workflow continues.
func TestEdgeFailure(t *testing.T) {
	t.Parallel()
	metricsDown := true
	w := workflow.New()
	start := w.NewNode("start", with(nil))
	done := w.NewNode("done", with(nil))
	w.NewEdge("metrics-ok", start, done, cond(func(property.Map) (bool, error) {
		if metricsDown {
			return false, errors.New("metrics backend unreachable")
		}
		return true, nil
	}))

	r := &recorder{t: t, w: w}
	w.AddCursor(start, "", property.Map{})
	assert.EqualError(t, r.progress(), "metrics backend unreachable")
	r.note("metrics backend is back")
	metricsDown = false
	require.NoError(t, r.progress())
	r.golden("edge-failure")
}

// us-east fails verification, so the join can never complete. Because the waiting cursors stay on their
// verify nodes, an abort edge out of each verify node lets the rollout halt instead of waiting forever.
func TestJoinAbort(t *testing.T) {
	t.Parallel()
	ro := newRollout()
	metricsBad := map[string]*bool{"us-west": new(bool), "us-east": new(bool)}
	anyBad := cond(func(property.Map) (bool, error) {
		return *metricsBad["us-west"] || *metricsBad["us-east"], nil
	})
	w := workflow.New()
	start, done := ro.define(w)
	// Both regions aborting into one node in the same step would be a race, so each has its own.
	halt := map[string]workflow.Node{}
	for _, region := range ro.usRegions {
		halt[region] = w.NewNode("halt-"+region, with(map[string]property.Value{"halted": boolean(true)}))
		w.NewEdge("abort", mustNode(t, w, "verify-"+region), halt[region], anyBad)
	}

	r := &recorder{t: t, w: w}
	w.AddCursor(start, "v7", pmap(map[string]property.Value{"version": str("7")}))
	*ro.metricsGood["us-west"] = true
	require.NoError(t, r.progress())
	r.note("us-east metrics look bad: both regions abort")
	*metricsBad["us-east"] = true
	require.NoError(t, r.progress())

	assert.Equal(t, workflow.NodeState{}, w.GetState(done))
	for _, region := range ro.usRegions {
		assert.Equal(t, str(region), w.GetState(halt[region]).Outputs.Get("region"))
	}
	golden(t, "join-abort-state", w.State())
	r.golden("join-abort")
}

// The join is decided, but deploying to eu fails, aborting the run before the merged cursor's move is
// committed. The decision stands: the next Progress completes the move without re-evaluating the join.
func TestJoinCommitRetried(t *testing.T) {
	t.Parallel()
	ro := newRollout()
	for _, good := range ro.metricsGood {
		*good = true
	}
	ro.broken["eu"] = true
	w := workflow.New()
	start, done := ro.define(w)

	r := &recorder{t: t, w: w}
	w.AddCursor(start, "v7", pmap(map[string]property.Value{"version": str("7")}))
	assert.EqualError(t, r.progress(), "deploy to eu failed")
	golden(t, "join-commit-retried-state", w.State())
	r.note("eu is fixed")
	ro.broken["eu"] = false
	require.NoError(t, r.progress())
	assert.Equal(t, str("eu"), w.GetState(done).Outputs.Get("region"))
	r.golden("join-commit-retried")
}

// The merge function rejects regions running different versions, so a hotfix to one region holds the
// join until the other region catches up. A merge function error fails the run.
func TestJoinMergeRejected(t *testing.T) {
	t.Parallel()
	ro := newRollout()
	policyDown := false
	ro.merge = func(ctx context.Context, candidates []workflow.MergeCandidate) (bool, workflow.MergedCursor, error) {
		if policyDown {
			return false, workflow.MergedCursor{}, errors.New("merge policy unavailable")
		}
		for _, c := range candidates[1:] {
			if !c.Inputs.Get("version").Equals(candidates[0].Inputs.Get("version")) {
				return false, workflow.MergedCursor{}, nil
			}
		}
		return nestByFrom(ctx, candidates)
	}
	*ro.metricsGood["us-west"] = true
	*ro.metricsGood["eu"] = true
	w := workflow.New()
	start, done := ro.define(w)
	r := &recorder{t: t, w: w}
	w.AddCursor(start, "v7", pmap(map[string]property.Value{"version": str("7")}))
	require.NoError(t, r.progress())
	r.note("us-west is hotfixed while us-east's metrics are pending")
	w.AddCursor(mustNode(t, w, "queue-us-west"), "us-west hotfix", pmap(map[string]property.Value{"version": str("7.1")}))
	require.NoError(t, r.progress())
	r.note("us-east metrics look good, but the versions differ")
	*ro.metricsGood["us-east"] = true
	require.NoError(t, r.progress())
	r.note("us-east gets the hotfix too, but the merge policy is down")
	policyDown = true
	w.AddCursor(mustNode(t, w, "queue-us-east"), "us-east hotfix", pmap(map[string]property.Value{"version": str("7.1")}))
	assert.EqualError(t, r.progress(), "merge policy unavailable")
	r.note("the merge policy is back")
	policyDown = false
	require.NoError(t, r.progress())

	assert.Equal(t, str("7.1"), w.GetState(done).Outputs.Get("verify-us-west").AsMap().Get("version"))
	assert.Equal(t, str("7.1"), w.GetState(done).Outputs.Get("verify-us-east").AsMap().Get("version"))
	r.golden("join-merge-rejected")
}

// A failing run leaves an unsettled cursor beside one that committed after it. That arrival order
// survives State/FromState: on the restored workflow the earlier cursor is overwritten once it settles,
// exactly as on the original.
func TestArrivalOrderSurvivesRestore(t *testing.T) {
	t.Parallel()
	broken := true
	define := func(w workflow.Workflow) (startGood, startBad workflow.Node) {
		startGood = w.NewNode("start-good", with(nil))
		startBad = w.NewNode("start-bad", with(nil))
		staging := w.NewNode("staging", with(map[string]property.Value{"env": str("staging")}))
		prod := w.NewNode("prod", with(nil))
		// Failing, this deploy first queues another release straight onto staging; the abort leaves that
		// release unsettled there while the good release's entry into staging is committed beside it.
		w.NewNode("deploy-bad", func(
			_ context.Context, w workflow.Workflow, _ workflow.Cursor, in property.Map,
		) (property.Map, error) {
			if !broken {
				return in, nil
			}
			w.AddCursor(staging, "queued", pmap(map[string]property.Value{"version": str("queued")}))
			return property.Map{}, errors.New("deploy exploded")
		})
		w.NewEdge("go", startGood, staging, always)
		w.NewEdge("go", startBad, mustNode(t, w, "deploy-bad"), always)
		w.NewEdge("promote", staging, prod, flag("never"))
		return startGood, startBad
	}

	w := workflow.New()
	startGood, startBad := define(w)
	r := &recorder{t: t, w: w}
	w.AddCursor(startGood, "good", pmap(map[string]property.Value{"version": str("good")}))
	w.AddCursor(startBad, "bad", pmap(map[string]property.Value{"version": str("bad")}))
	assert.EqualError(t, r.progress(), "deploy exploded")
	state := w.State()
	golden(t, "arrival-order-state", state)

	broken = false
	resumed, err := workflow.FromState(state)
	require.NoError(t, err)
	define(resumed)
	r2 := &recorder{t: t, w: resumed}
	r2.note("resumed: the queued release settles and is overwritten by the release that committed beside it")
	require.NoError(t, r2.progress())
	r2.golden("arrival-order-resume")

	require.NoError(t, r.progress())
	// Restored cursors are re-created in node definition order, so State lists them in a different order.
	var before, after map[string]any
	require.NoError(t, json.Unmarshal(w.State(), &before))
	require.NoError(t, json.Unmarshal(resumed.State(), &after))
	assert.Equal(t, before["nextCursor"], after["nextCursor"])
	assert.ElementsMatch(t, before["cursors"], after["cursors"])
}

// An edge condition that passes, setting extra on the cursor and deleting the given keys.
func setting(extra map[string]property.Value, deleting ...string) workflow.EdgeFunc {
	return func(context.Context, workflow.Workflow, workflow.Cursor, property.Map) (bool, workflow.Overlay, error) {
		return true, workflow.Overlay{Values: pmap(extra), Deleted: deleting}, nil
	}
}

// Edge overlays: what a condition sets or deletes reaches the next node only if the cursor crosses that
// edge because of it. Conditions of an And compose with the first name winning; only the passing condition
// of an Or counts.
func TestEdgeOverlays(t *testing.T) {
	t.Parallel()
	w := workflow.New()
	start := w.NewNode("start", with(nil))
	viaAnd := w.NewNode("via-and", with(nil))
	viaOr := w.NewNode("via-or", with(nil))
	end := w.NewNode("end", with(nil))
	// The failing condition's overlay is discarded with the edge.
	w.NewAndEdge("blocked", start, end, conds(map[string]workflow.EdgeFunc{
		"sets": setting(map[string]property.Value{"blocked": str("yes")}, "start"), "never": flag("never"),
	}))
	// a wins over b: "who" is a's, "gone" stays deleted although b sets it, and b's deletion of "keep" goes
	// through since a leaves it alone.
	w.NewAndEdge("and", start, viaAnd, conds(map[string]workflow.EdgeFunc{
		"b": setting(map[string]property.Value{"who": str("b"), "b": str("b"), "gone": str("b")}, "keep"),
		"a": setting(map[string]property.Value{"who": str("a"), "a": str("a")}, "gone"),
	}))
	w.NewOrEdge("or", viaAnd, viaOr, conds(map[string]workflow.EdgeFunc{
		"first":  cond(func(property.Map) (bool, error) { return false, nil }),
		"second": setting(map[string]property.Value{"or": str("second")}),
		"third":  setting(map[string]property.Value{"or": str("third")}),
	}))
	w.NewEdge("done", viaOr, end, setting(map[string]property.Value{"edge": str("done")}, "start"))

	r := &recorder{t: t, w: w}
	w.AddCursor(start, "", pmap(map[string]property.Value{"start": str("yes"), "gone": num(1), "keep": num(2)}))
	require.NoError(t, r.progress())
	assert.Equal(t, pmap(map[string]property.Value{
		"who": str("a"), "a": str("a"), "b": str("b"), "or": str("second"), "edge": str("done"),
	}), w.GetState(end).Outputs)
	r.golden("edge-overlays")
}

// Reconcile re-runs the node function for parked cursors from the values they entered with, so a node
// whose function depends on the world converges on every run, and its edges see the fresh outputs.
func TestReconcile(t *testing.T) {
	t.Parallel()
	healthy := false
	w := workflow.New()
	start := w.NewNode("start", with(nil))
	deploy := w.NewNode("deploy", func(_ context.Context, _ workflow.Workflow, _ workflow.Cursor, in property.Map) (
		property.Map, error,
	) {
		return in.Set("healthy", boolean(healthy)).Set("runs", num(in.Get("runs").AsNumber()+1)), nil
	})
	done := w.NewNode("done", with(nil))
	w.NewEdge("start", start, deploy, always)
	w.NewEdge("healthy", deploy, done, flag("healthy"))

	r := &recorder{t: t, w: w}
	reconcile := func() {
		updates := make(chan workflow.WorkflowUpdate)
		finished := make(chan struct{})
		go func() {
			defer close(finished)
			for u := range updates {
				r.events = append(r.events, recorded{Type: strings.TrimPrefix(fmt.Sprintf("%T", u), "workflow."), Event: u})
			}
		}()
		err := w.Reconcile(t.Context(), fsa.SyncRunner, updates)
		close(updates)
		<-finished
		rec := recorded{Type: "ReconcileReturned"}
		if err != nil {
			rec.Error = err.Error()
		}
		r.events = append(r.events, rec)
	}
	w.AddCursor(start, "", pmap(map[string]property.Value{"runs": num(0)}))
	require.NoError(t, r.progress())
	r.note("deploy ran on arrival, so it is not reconciled; start has no cursor")
	reconcile()
	r.note("run 2: the cursor is parked on deploy, which is reconciled from the values it entered with")
	require.NoError(t, r.progress())
	reconcile()
	r.note("run 3: the world healed, but the edge sees the previous run's outputs; reconcile sees the world")
	healthy = true
	require.NoError(t, r.progress())
	reconcile()
	r.note("run 4: the edge takes the fresh outputs")
	require.NoError(t, r.progress())
	assert.Equal(t, pmap(map[string]property.Value{"runs": num(1), "healthy": boolean(true)}), w.GetState(done).Outputs)

	var cursors []string
	w.Cursors(func(c workflow.Cursor, n workflow.Node) bool {
		cursors = append(cursors, c.ID+"@"+n.ID())
		return true
	})
	assert.Equal(t, []string{"c1@done"}, cursors)
	r.golden("reconcile")
}

// A self-loop: an edge from a node to itself re-enters the node with its overlay applied, giving the new
// visit fresh entered values — how a failed release rolls a node back to the last good one.
func TestSelfLoop(t *testing.T) {
	t.Parallel()
	w := workflow.New()
	start := w.NewNode("start", with(nil))
	prod := w.NewNode("prod", with(nil))
	w.NewEdge("enter", start, prod, always)
	w.NewEdge("rollback", prod, prod, func(
		_ context.Context, _ workflow.Workflow, _ workflow.Cursor, in property.Map,
	) (bool, workflow.Overlay, error) {
		if in.Get("sha").AsString() != "bad" {
			return false, workflow.Overlay{}, nil
		}
		return true, workflow.Overlay{Values: pmap(map[string]property.Value{"sha": str("good")})}, nil
	})
	r := &recorder{t: t, w: w}
	w.AddCursor(start, "release", pmap(map[string]property.Value{"sha": str("bad")}))
	require.NoError(t, r.progress())
	cursors := map[string]string{}
	w.Cursors(func(c workflow.Cursor, n workflow.Node) bool {
		cursors[c.Label] = n.ID()
		return true
	})
	assert.Equal(t, map[string]string{"release": "prod"}, cursors)
	assert.Equal(t, "good", w.GetState(prod).Outputs.Get("sha").AsString())
	r.golden("self-loop")
}
