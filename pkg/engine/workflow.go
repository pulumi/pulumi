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

package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/pkg/v3/workflow"
	"github.com/pulumi/pulumi/pkg/v3/workflow/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/promise"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// workflowProgressor implements deploy.WorkflowProgressor: it advances every workflow the deployment
// registered by driving pkg/workflow, running node programs as nested deployments. Node programs, edge
// conditions and join merges are closures in the user's program, reached through the callbacks facility.
//
// Each node is a pulumi:index:WorkflowNode component registered under the workflow; its program's
// resources are parented under it and namespaced by a per-node project qualifier in their URNs. The
// node's outputs record its occupant and visit history; the workflow's outputs record its cursors.
type workflowProgressor struct {
	plugctx       *plugin.Context
	backendClient deploy.BackendClient
	resourceHooks *deploy.ResourceHooks
	stackName     tokens.StackName
	organization  tokens.Name
	parallel      int32

	m         sync.Mutex
	callbacks map[string]*deploy.CallbacksClient
}

func newWorkflowProgressor(
	plugctx *plugin.Context,
	backendClient deploy.BackendClient,
	resourceHooks *deploy.ResourceHooks,
	stackName tokens.StackName,
	organization tokens.Name,
	parallel int32,
) *workflowProgressor {
	return &workflowProgressor{
		plugctx:       plugctx,
		backendClient: backendClient,
		resourceHooks: resourceHooks,
		stackName:     stackName,
		organization:  organization,
		parallel:      parallel,
		callbacks:     map[string]*deploy.CallbacksClient{},
	}
}

func (x *workflowProgressor) Progress(ctx context.Context, d *deploy.Deployment, host deploy.WorkflowHost) error {
	var wfs []*pkgresource.State
	d.News().Range(func(_ resource.URN, s *pkgresource.State) bool {
		if s.Type == deploy.WorkflowType {
			wfs = append(wfs, s)
		}
		return true
	})
	slices.SortFunc(wfs, func(a, b *pkgresource.State) int {
		return strings.Compare(string(a.URN), string(b.URN))
	})

	var errs []error
	for _, wf := range wfs {
		if err := x.progressWorkflow(ctx, d, host, wf); err != nil {
			errs = append(errs, fmt.Errorf("workflow %v: %w", wf.URN, err))
		}
	}
	return errors.Join(errs...)
}

// wfCallback identifies a closure in the user's program, reachable via the callbacks service.
type wfCallback struct {
	Target string
	Token  string
}

type wfEdge struct {
	Name, From, To string
	Kind           string                 // "single", "and", "or" or "join"
	Conditions     map[string]wfCallback  // single/and/or: by condition name
	Branches       map[string]*wfCallback // join: by from node; nil for an unconditional branch
	Merge          wfCallback             // join
}

// wfGraph is the workflow's definition, parsed from the resource's inputs.
type wfGraph struct {
	nodes   map[string]*wfCallback // nil for a waypoint (a node without a program)
	edges   []wfEdge               // in definition order
	entries map[string]wfEntry
}

type wfEntry struct {
	Node   string
	Inputs resource.PropertyMap
}

// wfState is the durable state of a workflow, persisted under "state" in the resource's outputs.
type wfState struct {
	Workflow       json.RawMessage   `json:"workflow,omitempty"` // pkg/workflow's saved cursors
	Entries        map[string]wfSeed `json:"entries"`            // the last placement of each entry
	NextGeneration int               `json:"nextGeneration"`
	Nodes          []string          `json:"nodes"` // every node with a resource, for finding them again
}

type wfSeed struct {
	Hash       string `json:"hash"`
	Generation int    `json:"generation"`
}

// A nodeRecord is what a node resource's outputs persist: its occupant and visit history.
type nodeRecord struct {
	Occupant string
	History  []*nodeVisit // newest first
	LastRun  time.Time
}

type nodeVisit struct {
	Cursor        string
	Entered, Left time.Time
	Inputs        property.Map
	Outputs       property.Map
	Ran           bool
	Error         string
}

// workflowRun is the in-flight state of progressing one workflow within one deployment.
type workflowRun struct {
	x       *workflowProgressor
	d       *deploy.Deployment
	host    deploy.WorkflowHost
	wf      *pkgresource.State
	g       *wfGraph
	st      *wfState
	project tokens.PackageName // the deployment's project; slice projects derive from it
	w       workflow.Workflow

	config     map[string]string
	secretKeys []string

	// m guards everything below: node programs run on concurrent goroutines.
	m       sync.Mutex
	nodes   map[string]*nodeRecord
	nodeRes map[string]*pkgresource.State // the registered node resources, by node
	slices  map[string][]*pkgresource.State
	touched map[string]bool // nodes whose program ran this up
	display *display.Model

	reconciling atomic.Bool // Set while pkg/workflow reconciles, so node functions know why they run
}

func (x *workflowProgressor) progressWorkflow(
	ctx context.Context, d *deploy.Deployment, host deploy.WorkflowHost, wf *pkgresource.State,
) error {
	g, err := parseWorkflowGraph(wf.Inputs)
	if err != nil {
		return err
	}
	wf.Lock.Lock()
	st, err := parseWorkflowState(wf.Outputs)
	wf.Lock.Unlock()
	if err != nil {
		return err
	}
	cfg, err := d.Target().Config.Decrypt(d.Target().Decrypter)
	if err != nil {
		return fmt.Errorf("decrypting config for node programs: %w", err)
	}
	run := &workflowRun{
		x: x, d: d, host: host, wf: wf, g: g, st: st,
		project: d.Source().Project(),
		config:  make(map[string]string, len(cfg)),
		nodes:   map[string]*nodeRecord{},
		nodeRes: map[string]*pkgresource.State{},
		slices:  map[string][]*pkgresource.State{},
		touched: map[string]bool{},
		display: display.New(),
	}
	for k, v := range cfg {
		run.config[k.String()] = v
	}
	for _, k := range d.Target().Config.SecureKeys() {
		run.secretKeys = append(run.secretKeys, k.String())
	}
	if d.Options().DryRun {
		return run.preview(ctx)
	}
	return run.progress(ctx)
}

// preview is what a dry run does for a workflow: nothing moves and no callback runs, but the node resources
// are registered so the plan shows them (and the sweep leaves the resources of their programs alone), and
// the diagram of the last run is shown.
func (run *workflowRun) preview(ctx context.Context) error {
	defined := slices.Sorted(maps.Keys(run.g.nodes))
	run.loadPrev()
	var keep []resource.URN
	for _, node := range defined {
		for _, res := range run.slices[node] {
			keep = append(keep, res.URN)
		}
	}
	run.host.Keep(keep...)
	for _, node := range defined {
		if _, err := run.host.RegisterResource(ctx, run.nodeGoal(node)); err != nil {
			return fmt.Errorf("registering node %q: %w", node, err)
		}
	}
	run.wf.Lock.Lock()
	diagram, ok := run.wf.Outputs["diagram"]
	run.wf.Lock.Unlock()
	if ok && diagram.IsString() {
		run.x.plugctx.Diag.Infof(diag.RawMessage(run.wf.URN, diagram.StringValue()))
	}
	return nil
}

func (run *workflowRun) nodeGoal(node string) *pkgresource.Goal {
	return &pkgresource.Goal{
		Type:   deploy.WorkflowNodeType,
		Name:   node,
		Parent: run.wf.URN,
		Properties: property.NewMap(map[string]property.Value{
			"workflow": property.New(string(run.wf.URN)),
			"node":     property.New(node),
		}),
	}
}

func (run *workflowRun) progress(ctx context.Context) error {
	defined := slices.Sorted(maps.Keys(run.g.nodes))

	// 1. Recover what the previous snapshot knows: each node's record and the resources of its program.
	run.loadPrev()
	var keep []resource.URN
	for _, node := range defined {
		for _, res := range run.slices[node] {
			keep = append(keep, res.URN)
		}
	}
	run.host.Keep(keep...)

	// 2. Register the node resources, as the program would have, so the nested deployments can nest
	// their roots under them.
	for _, node := range defined {
		res, err := run.host.RegisterResource(ctx, run.nodeGoal(node))
		if err != nil {
			return fmt.Errorf("registering node %q: %w", node, err)
		}
		run.nodeRes[node] = res
	}

	// 3. Build the workflow: restored cursors, nodes, edges, then the entries that changed.
	if err := run.define(defined); err != nil {
		return err
	}

	// 4. Advance it, then reconcile every existing node whose program did not run.
	updates := make(chan workflow.WorkflowUpdate)
	rendered := make(chan struct{})
	go func() {
		defer close(rendered)
		for u := range updates {
			run.display.Apply(u)
			run.x.plugctx.StatusDiag.Infof(diag.RawMessage(run.wf.URN, run.display.Render()))
		}
	}()
	runner := func(ctx context.Context, f func(context.Context)) error {
		go f(ctx)
		return nil
	}
	var errs []error
	if err := run.w.Progress(ctx, runner, updates); err != nil {
		errs = append(errs, err)
	} else {
		run.reconciling.Store(true)
		if err := run.w.Reconcile(ctx, runner, updates); err != nil {
			errs = append(errs, err)
		}
		if err := run.reconcileVacated(ctx, defined); err != nil {
			errs = append(errs, err)
		}
	}
	close(updates)
	<-rendered

	// 5. Persist: node records (occupants, visit history) and the workflow's own state. Cursor positions
	// survive even a failed run.
	if err := run.persist(defined); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// loadPrev recovers node records and program resources from the previous snapshot.
func (run *workflowRun) loadPrev() {
	prev := run.d.Prev()
	if prev == nil {
		return
	}
	known := map[string]bool{}
	for _, node := range run.st.Nodes {
		known[node] = true
	}
	for node := range run.g.nodes {
		known[node] = true
	}
	projects := map[tokens.PackageName]string{}
	urns := map[resource.URN]string{}
	for node := range known {
		projects[run.sliceProject(node)] = node
		urns[run.nodeURN(node)] = node
	}
	for _, res := range prev.Resources {
		if node, ok := urns[res.URN]; ok {
			run.nodes[node] = parseNodeRecord(res.Outputs)
		}
		if node, ok := projects[res.URN.Project()]; ok {
			run.slices[node] = append(run.slices[node], res)
		}
	}
}

// define builds the pkg/workflow instance for this run.
func (run *workflowRun) define(defined []string) error {
	var err error
	if len(run.st.Workflow) > 0 {
		run.w, err = workflow.FromState(run.st.Workflow)
		if err != nil {
			return err
		}
	} else {
		run.w = workflow.New()
	}
	nodes := map[string]workflow.Node{}
	for _, name := range defined {
		nodes[name] = run.w.NewNode(name, run.nodeFunc(name))
	}
	for _, e := range run.g.edges {
		switch e.Kind {
		case "single":
			for cond, cb := range e.Conditions {
				run.w.NewEdge(e.Name, nodes[e.From], nodes[e.To], run.condFunc(e.Name, cond, cb))
			}
		case "and", "or":
			conds := make(map[string]workflow.EdgeFunc, len(e.Conditions))
			for cond, cb := range e.Conditions {
				conds[cond] = run.condFunc(e.Name, cond, cb)
			}
			if e.Kind == "and" {
				run.w.NewAndEdge(e.Name, nodes[e.From], nodes[e.To], conds)
			} else {
				run.w.NewOrEdge(e.Name, nodes[e.From], nodes[e.To], conds)
			}
		case "join":
			var args []workflow.JoinEdgeArg
			for _, from := range slices.Sorted(maps.Keys(e.Branches)) {
				arg := workflow.JoinEdgeArg{From: nodes[from], Edge: passEdge}
				if cb := e.Branches[from]; cb != nil {
					arg.Edge = run.condFunc(e.Name, from, *cb)
				}
				args = append(args, arg)
			}
			run.w.NewJoinEdge(e.Name, args, nodes[e.To], run.mergeFunc(e))
		}
	}
	for _, name := range slices.Sorted(maps.Keys(run.g.entries)) {
		entry := run.g.entries[name]
		hash := stableHash(entry.Inputs)
		seed, placed := run.st.Entries[name]
		if placed && seed.Hash == hash {
			continue
		}
		run.st.NextGeneration++
		seed = wfSeed{Hash: hash, Generation: run.st.NextGeneration}
		run.st.Entries[name] = seed
		run.w.AddCursor(nodes[entry.Node], cursorLabel(name, seed.Generation), resource.FromResourcePropertyMap(entry.Inputs))
	}
	return nil
}

func passEdge(context.Context, workflow.Workflow, workflow.Cursor, property.Map) (bool, workflow.Overlay, error) {
	return true, workflow.Overlay{}, nil
}

func cursorLabel(name string, generation int) string { return fmt.Sprintf("%s#%d", name, generation) }

// nodeFunc is the pkg/workflow node function for node: it runs the node's program (if any) as a nested
// deployment and records the visit.
func (run *workflowRun) nodeFunc(node string) workflow.NodeFunc {
	return func(ctx context.Context, _ workflow.Workflow, c workflow.Cursor, inputs property.Map) (property.Map, error) {
		reconcile := run.reconciling.Load()
		run.m.Lock()
		rec := run.record(node)
		// A cursor placed on the node by an entry is first run by Reconcile, since placement does not run
		// the node: for the node that is an arrival.
		if reconcile && rec.current(c.Label) == nil {
			reconcile = false
		}
		if !reconcile {
			rec.arrive(c.Label, inputs)
		}
		run.touched[node] = true
		program := run.g.nodes[node]
		run.m.Unlock()

		outputs, err := inputs, error(nil)
		if program != nil {
			outputs, err = run.runNode(ctx, node, *program, c.Label, inputs, reconcile)
		}
		run.m.Lock()
		defer run.m.Unlock()
		rec.LastRun = time.Now().UTC()
		if visit := rec.current(c.Label); visit != nil {
			visit.Ran = true
			if err != nil {
				visit.Error = err.Error()
			} else {
				visit.Outputs, visit.Error = outputs, ""
			}
		}
		if err != nil {
			return property.Map{}, fmt.Errorf("node %q: %w", node, err)
		}
		return outputs, nil
	}
}

// reconcileVacated runs the program of every node that has been visited but holds no cursor and did not
// run this up, from the values of its last visit. What the program sets is discarded: nothing downstream
// of a vacated node can change.
func (run *workflowRun) reconcileVacated(ctx context.Context, defined []string) error {
	var wg sync.WaitGroup
	errs := make([]error, len(defined))
	for i, node := range defined {
		run.m.Lock()
		rec := run.nodes[node]
		program := run.g.nodes[node]
		skip := run.touched[node] || program == nil || rec == nil || len(rec.History) == 0
		run.m.Unlock()
		if skip {
			continue
		}
		last := rec.History[0]
		wg.Go(func() {
			_, err := run.runNode(ctx, node, *program, last.Cursor, last.Inputs, true)
			run.m.Lock()
			defer run.m.Unlock()
			rec.LastRun = time.Now().UTC()
			if err != nil {
				errs[i] = fmt.Errorf("reconciling node %q: %w", node, err)
				last.Error = err.Error()
			} else {
				last.Error = ""
			}
		})
	}
	wg.Wait()
	return errors.Join(errs...)
}

// persist records node records and the workflow's state as resource outputs.
func (run *workflowRun) persist(defined []string) error {
	occupants := map[string][]string{}
	run.w.Cursors(func(c workflow.Cursor, n workflow.Node) bool {
		occupants[n.ID()] = append(occupants[n.ID()], c.Label)
		return true
	})
	run.m.Lock()
	now := time.Now().UTC()
	for node, rec := range run.nodes {
		rec.Occupant = ""
		if occ := occupants[node]; len(occ) > 0 {
			rec.Occupant = occ[0] // The settled one: later arrivals overwrite it once they settle
		}
		if len(rec.History) > 0 {
			if v := rec.History[0]; v.Left.IsZero() && v.Cursor != rec.Occupant {
				v.Left = now
			}
		}
	}
	var errs []error
	for _, node := range defined {
		rec := run.record(node)
		if err := run.host.RegisterResourceOutputs(run.nodeRes[node].URN, rec.properties()); err != nil {
			errs = append(errs, fmt.Errorf("recording node %q: %w", node, err))
		}
	}
	run.st.Workflow = run.w.State()
	run.st.Nodes = defined
	state, err := json.Marshal(run.st)
	contract.AssertNoErrorf(err, "workflow state is always serializable")
	cursors := []property.Value{}
	for _, node := range slices.Sorted(maps.Keys(occupants)) {
		for _, label := range occupants[node] {
			cursors = append(cursors, property.New(map[string]property.Value{
				"name": property.New(label), "node": property.New(node),
			}))
		}
	}
	diagram := run.display.Render()
	run.m.Unlock()

	run.wf.Lock.Lock()
	outputs := run.wf.Outputs.Copy()
	run.wf.Lock.Unlock()
	outputs["state"] = resource.NewProperty(string(state))
	outputs["cursors"] = resource.ToResourcePropertyValue(property.New(cursors))
	outputs["diagram"] = resource.NewProperty(diagram)
	run.x.plugctx.Diag.Infof(diag.RawMessage(run.wf.URN, diagram))
	if err := run.host.RegisterResourceOutputs(run.wf.URN, outputs); err != nil {
		errs = append(errs, fmt.Errorf("persisting workflow state: %w", err))
	}
	return errors.Join(errs...)
}

// record returns node's record, creating it. Caller holds run.m.
func (run *workflowRun) record(node string) *nodeRecord {
	rec, ok := run.nodes[node]
	if !ok {
		rec = &nodeRecord{}
		run.nodes[node] = rec
	}
	return rec
}

// arrive opens a visit for cursor, closing the previous one.
func (rec *nodeRecord) arrive(cursor string, inputs property.Map) {
	now := time.Now().UTC()
	if len(rec.History) > 0 && rec.History[0].Left.IsZero() {
		rec.History[0].Left = now
	}
	rec.History = append([]*nodeVisit{{Cursor: cursor, Entered: now, Inputs: inputs}}, rec.History...)
	rec.Occupant = cursor
}

// current returns the latest visit if it is cursor's.
func (rec *nodeRecord) current(cursor string) *nodeVisit {
	if len(rec.History) > 0 && rec.History[0].Cursor == cursor {
		return rec.History[0]
	}
	return nil
}

func (rec *nodeRecord) properties() resource.PropertyMap {
	visits := make([]resource.PropertyValue, len(rec.History))
	for i, v := range rec.History {
		visit := resource.PropertyMap{
			"cursor":  resource.NewProperty(v.Cursor),
			"entered": resource.NewProperty(v.Entered.Format(time.RFC3339Nano)),
			"inputs":  resource.NewProperty(resource.ToResourcePropertyMap(v.Inputs)),
		}
		if !v.Left.IsZero() {
			visit["left"] = resource.NewProperty(v.Left.Format(time.RFC3339Nano))
		}
		if v.Ran {
			visit["outputs"] = resource.NewProperty(resource.ToResourcePropertyMap(v.Outputs))
		}
		if v.Error != "" {
			visit["error"] = resource.NewProperty(v.Error)
		}
		visits[i] = resource.NewProperty(visit)
	}
	out := resource.PropertyMap{"visits": resource.NewProperty(visits)}
	if rec.Occupant != "" {
		out["occupant"] = resource.NewProperty(rec.Occupant)
	}
	if !rec.LastRun.IsZero() {
		out["lastRun"] = resource.NewProperty(rec.LastRun.Format(time.RFC3339Nano))
	}
	return out
}

func parseNodeRecord(outputs resource.PropertyMap) *nodeRecord {
	rec := &nodeRecord{}
	if v, ok := outputs["occupant"]; ok && v.IsString() {
		rec.Occupant = v.StringValue()
	}
	if v, ok := outputs["lastRun"]; ok && v.IsString() {
		rec.LastRun, _ = time.Parse(time.RFC3339Nano, v.StringValue())
	}
	if v, ok := outputs["visits"]; ok && v.IsArray() {
		for _, vv := range v.ArrayValue() {
			if !vv.IsObject() {
				continue
			}
			o := vv.ObjectValue()
			visit := &nodeVisit{}
			if s, ok := o["cursor"]; ok && s.IsString() {
				visit.Cursor = s.StringValue()
			}
			if s, ok := o["entered"]; ok && s.IsString() {
				visit.Entered, _ = time.Parse(time.RFC3339Nano, s.StringValue())
			}
			if s, ok := o["left"]; ok && s.IsString() {
				visit.Left, _ = time.Parse(time.RFC3339Nano, s.StringValue())
			}
			if m, ok := o["inputs"]; ok && m.IsObject() {
				visit.Inputs = resource.FromResourcePropertyMap(m.ObjectValue())
			}
			if m, ok := o["outputs"]; ok && m.IsObject() {
				visit.Outputs, visit.Ran = resource.FromResourcePropertyMap(m.ObjectValue()), true
			}
			if s, ok := o["error"]; ok && s.IsString() {
				visit.Error = s.StringValue()
			}
			rec.History = append(rec.History, visit)
		}
	}
	return rec
}

// view snapshots every node's record for a callback.
func (run *workflowRun) view() *pulumirpc.WorkflowView {
	run.m.Lock()
	defer run.m.Unlock()
	view := &pulumirpc.WorkflowView{Nodes: map[string]*pulumirpc.WorkflowNodeState{}}
	for node := range run.g.nodes {
		rec := run.nodes[node]
		if rec == nil {
			rec = &nodeRecord{}
		}
		state := &pulumirpc.WorkflowNodeState{Occupant: rec.Occupant}
		if !rec.LastRun.IsZero() {
			state.LastRun = timestamppb.New(rec.LastRun)
		}
		for _, v := range rec.History {
			visit := &pulumirpc.WorkflowVisit{
				Cursor:  v.Cursor,
				Entered: timestamppb.New(v.Entered),
				Inputs:  marshalValues(v.Inputs),
				Error:   v.Error,
			}
			if !v.Left.IsZero() {
				visit.Left = timestamppb.New(v.Left)
			}
			if v.Ran {
				visit.Outputs = marshalValues(v.Outputs)
			}
			state.History = append(state.History, visit)
		}
		view.Nodes[node] = state
	}
	return view
}

// -- Nested deployments --

// sliceProject namespaces a node's resources within the shared snapshot: URNs embed the project, so
// per-node projects make the resources of a node's program (including default providers and its root
// stack) unique by construction.
func (run *workflowRun) sliceProject(node string) tokens.PackageName {
	return tokens.PackageName(fmt.Sprintf("%s-wf-%s-%s", run.project, run.wf.URN.Name(), node))
}

func (run *workflowRun) nodeURN(node string) resource.URN {
	return resource.NewURN(run.wf.URN.Stack(), run.project, run.wf.URN.QualifiedType(), deploy.WorkflowNodeType, node)
}

// sliceSnapshot is the previous state a node's nested deployment runs against: the node's resources plus
// the parent chain they nest under, which the nested dependency graph requires.
func (run *workflowRun) sliceSnapshot(node string) *deploy.Snapshot {
	run.m.Lock()
	slice := run.slices[node]
	nodeRes := run.nodeRes[node]
	run.m.Unlock()
	var anchors []*pkgresource.State
	for res := nodeRes; res != nil; {
		anchors = append([]*pkgresource.State{res}, anchors...)
		parent, _ := run.d.News().Load(res.Parent)
		res = parent
	}
	resources := append(anchors, slice...)
	manifest := deploy.Manifest{}
	manifest.Magic = manifest.NewMagic()
	prev := run.d.Prev()
	if prev == nil {
		return deploy.NewSnapshot(manifest, nil, resources, nil, deploy.SnapshotMetadata{}, nil, nil)
	}
	return deploy.NewSnapshot(
		manifest, prev.SecretsManager, resources, nil, deploy.SnapshotMetadata{}, nil, prev.Extensions)
}

// runNode runs node's program as a nested deployment for cursor, entered with inputs. It returns the
// values the program left on the cursor.
func (run *workflowRun) runNode(
	ctx context.Context, node string, program wfCallback, cursor string, inputs property.Map, reconcile bool,
) (property.Map, error) {
	x := run.x
	project := run.sliceProject(node)
	var outputs property.Map
	runner := func(monitorAddr string) *promise.Promise[struct{}] {
		cs := &promise.CompletionSource[struct{}]{}
		go func() {
			req := &pulumirpc.WorkflowNodeRequest{
				MonitorAddr:      monitorAddr,
				EngineAddr:       x.plugctx.Host.ServerAddr(),
				Project:          string(project),
				Stack:            x.stackName.String(),
				Organization:     string(x.organization),
				Config:           run.config,
				ConfigSecretKeys: run.secretKeys,
				Parallel:         x.parallel,
				Node:             node,
				Cursor:           &pulumirpc.WorkflowCursor{Name: cursor, Values: marshalValues(inputs)},
				View:             run.view(),
				Reconcile:        reconcile,
			}
			var resp pulumirpc.WorkflowNodeResponse
			if err := x.invoke(ctx, program, req, &resp); err != nil {
				cs.Reject(err)
				return
			}
			values, err := unmarshalValues(resp.Outputs)
			if err != nil {
				cs.Reject(err)
				return
			}
			outputs = values
			cs.Fulfill(struct{}{})
		}()
		return cs.Promise()
	}
	err := run.runNested(ctx, node, func(target *deploy.Target, panicErrs chan<- error) deploy.Source {
		runinfo := &deploy.EvalRunInfo{
			Proj: &workspace.Project{
				Name:    project,
				Runtime: workspace.NewProjectRuntimeInfo("workflow-node", nil),
			},
			Pwd:     x.plugctx.Pwd,
			Program: ".",
			Target:  target,
		}
		return deploy.NewEvalSource(x.plugctx, runinfo, nil, x.resourceHooks,
			deploy.EvalSourceOptions{Parallel: x.parallel}, panicErrs, nil, runner)
	})
	if err != nil {
		return property.Map{}, err
	}
	return outputs, nil
}

// runNested executes a nested deployment for node. Its steps flow through the outer deployment's events,
// persisting into the shared snapshot and rendering in the display; they are also journaled locally to
// compute the node's resulting resources, which replace the slice even on failure, so retries see what
// committed.
func (run *workflowRun) runNested(
	ctx context.Context,
	node string,
	makeSource func(target *deploy.Target, panicErrs chan<- error) deploy.Source,
) error {
	x := run.x
	prev := run.sliceSnapshot(node)
	outer := run.d.Target()
	target := &deploy.Target{
		Name:         outer.Name,
		Organization: outer.Organization,
		Config:       outer.Config,
		Decrypter:    outer.Decrypter,
		Snapshot:     prev,
		Tags:         outer.Tags,
	}
	// ponytail: buffered channel bounds panic reports; a hung nested deployment is not recovered here.
	panicErrs := make(chan error, 16)
	source := makeSource(target, panicErrs)

	journal := NewTestJournal()
	journal.SkipVerify = true
	events := &workflowTeeEvents{outer: run.d.Events(), journal: journal}
	opts := &deploy.Options{
		Parallel:           x.parallel,
		RootParent:         run.nodeRes[node].URN,
		WorkflowProgressor: x, // Workflows inside node programs progress with the node
	}
	depl, err := deploy.NewDeployment(
		x.plugctx, opts, events, target, prev, nil, source, x.backendClient, x.resourceHooks)
	if err != nil {
		contract.IgnoreClose(journal)
		return err
	}
	_, execErr := depl.Execute(ctx)
	contract.IgnoreClose(journal)

	select {
	case perr := <-panicErrs:
		execErr = errors.Join(execErr, fmt.Errorf("panic in nested deployment: %v", perr))
	default:
	}

	snap, snapErr := journal.Snap(prev)
	if snap != nil {
		project := run.sliceProject(node)
		var resources []*pkgresource.State
		for _, res := range snap.Resources {
			if res.URN.Project() == project {
				resources = append(resources, res)
			}
		}
		run.m.Lock()
		run.slices[node] = resources
		run.m.Unlock()
	}
	return errors.Join(execErr, snapErr)
}

// workflowTeeEvents forwards nested-deployment events to the outer deployment's events (shared snapshot
// manager and display) while journaling steps locally to rebuild the node's resources.
type workflowTeeEvents struct {
	outer   deploy.Events
	journal *TestJournal
}

type workflowTeeMutation struct {
	outer   any
	journal SnapshotMutation
}

func (t *workflowTeeEvents) OnSnapshotWrite(base *deploy.Snapshot) error {
	return errors.Join(t.outer.OnSnapshotWrite(base), t.journal.Write(base))
}

func (t *workflowTeeEvents) OnRebuiltBaseState() error {
	return errors.Join(t.outer.OnRebuiltBaseState(), t.journal.RebuiltBaseState())
}

func (t *workflowTeeEvents) OnResourceStepPre(step deploy.Step) (any, error) {
	octx, err := t.outer.OnResourceStepPre(step)
	if err != nil {
		return nil, err
	}
	jctx, err := t.journal.BeginMutation(step)
	if err != nil {
		return nil, err
	}
	return workflowTeeMutation{outer: octx, journal: jctx}, nil
}

func (t *workflowTeeEvents) OnResourceStepPost(
	ctx any, step deploy.Step, status resource.Status, err error,
) error {
	tee := ctx.(workflowTeeMutation)
	return errors.Join(
		t.outer.OnResourceStepPost(tee.outer, step, status, err),
		tee.journal.End(step, err == nil || status == resource.StatusPartialFailure),
	)
}

func (t *workflowTeeEvents) OnResourceOutputs(step deploy.Step) error {
	return errors.Join(t.outer.OnResourceOutputs(step), t.journal.RegisterResourceOutputs(step))
}

func (t *workflowTeeEvents) OnPolicyViolation(urn resource.URN, d plugin.AnalyzeDiagnostic) {
	t.outer.OnPolicyViolation(urn, d)
}

func (t *workflowTeeEvents) OnPolicyRemediation(urn resource.URN, r plugin.Remediation,
	before, after property.Map,
) {
	t.outer.OnPolicyRemediation(urn, r, before, after)
}

func (t *workflowTeeEvents) OnPolicyAnalyzeSummary(s plugin.PolicySummary) {
	t.outer.OnPolicyAnalyzeSummary(s)
}

func (t *workflowTeeEvents) OnPolicyRemediateSummary(s plugin.PolicySummary) {
	t.outer.OnPolicyRemediateSummary(s)
}

func (t *workflowTeeEvents) OnPolicyAnalyzeStackSummary(s plugin.PolicySummary) {
	t.outer.OnPolicyAnalyzeStackSummary(s)
}

// -- Callbacks --

func (run *workflowRun) condFunc(edge, cond string, cb wfCallback) workflow.EdgeFunc {
	return func(ctx context.Context, _ workflow.Workflow, c workflow.Cursor, inputs property.Map) (
		bool, workflow.Overlay, error,
	) {
		req := &pulumirpc.WorkflowConditionRequest{
			Cursor:    &pulumirpc.WorkflowCursor{Name: c.Label, Values: marshalValues(inputs)},
			View:      run.view(),
			Edge:      edge,
			Condition: cond,
		}
		var resp pulumirpc.WorkflowConditionResponse
		if err := run.x.invoke(ctx, cb, req, &resp); err != nil {
			return false, workflow.Overlay{}, fmt.Errorf("edge %q condition %q: %w", edge, cond, err)
		}
		values, err := unmarshalValues(resp.Overlay)
		if err != nil {
			return false, workflow.Overlay{}, fmt.Errorf("edge %q condition %q: %w", edge, cond, err)
		}
		return resp.Pass, workflow.Overlay{Values: values, Deleted: resp.Deleted}, nil
	}
}

func (run *workflowRun) mergeFunc(e wfEdge) workflow.MergeFunc {
	return func(ctx context.Context, candidates []workflow.MergeCandidate) (bool, workflow.MergedCursor, error) {
		req := &pulumirpc.WorkflowMergeRequest{View: run.view(), Edge: e.Name}
		for _, c := range candidates {
			req.Candidates = append(req.Candidates, &pulumirpc.WorkflowMergeRequest_Candidate{
				From:   c.From.ID(),
				Cursor: &pulumirpc.WorkflowCursor{Name: c.Cursor.Label, Values: marshalValues(c.Inputs)},
			})
		}
		var resp pulumirpc.WorkflowMergeResponse
		if err := run.x.invoke(ctx, e.Merge, req, &resp); err != nil {
			return false, workflow.MergedCursor{}, fmt.Errorf("join %q: %w", e.Name, err)
		}
		if !resp.Merge {
			return false, workflow.MergedCursor{}, nil
		}
		values, err := unmarshalValues(resp.Values)
		if err != nil {
			return false, workflow.MergedCursor{}, fmt.Errorf("join %q: %w", e.Name, err)
		}
		run.m.Lock()
		run.st.NextGeneration++
		label := cursorLabel(resp.Name, run.st.NextGeneration)
		run.m.Unlock()
		return true, workflow.MergedCursor{Label: label, Inputs: values}, nil
	}
}

// invoke calls a user closure over the callbacks service.
func (x *workflowProgressor) invoke(ctx context.Context, cb wfCallback, req, resp proto.Message) error {
	client, err := x.callbackClient(cb.Target)
	if err != nil {
		return err
	}
	b, err := proto.Marshal(req)
	if err != nil {
		return err
	}
	r, err := client.Invoke(ctx, &pulumirpc.CallbackInvokeRequest{Token: cb.Token, Request: b})
	if err != nil {
		return err
	}
	if err := proto.Unmarshal(r.Response, resp); err != nil {
		return fmt.Errorf("invalid callback response: %w", err)
	}
	return nil
}

func (x *workflowProgressor) callbackClient(target string) (*deploy.CallbacksClient, error) {
	x.m.Lock()
	defer x.m.Unlock()
	if client, ok := x.callbacks[target]; ok {
		return client, nil
	}
	conn, err := grpc.NewClient(target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		rpcutil.GrpcChannelOptions(),
	)
	if err != nil {
		return nil, fmt.Errorf("connecting to callbacks service %s: %w", target, err)
	}
	client := deploy.NewCallbacksClient(conn)
	x.callbacks[target] = client
	return client, nil
}

var valueMarshalOptions = plugin.MarshalOptions{KeepSecrets: true, KeepResources: true, KeepOutputValues: true}

func marshalValues(m property.Map) *structpb.Struct {
	s, err := plugin.MarshalProperties(resource.ToResourcePropertyMap(m), valueMarshalOptions)
	contract.AssertNoErrorf(err, "cursor values are always marshallable")
	return s
}

func unmarshalValues(s *structpb.Struct) (property.Map, error) {
	if s == nil {
		return property.Map{}, nil
	}
	m, err := plugin.UnmarshalProperties(s, valueMarshalOptions)
	if err != nil {
		return property.Map{}, err
	}
	return resource.FromResourcePropertyMap(m), nil
}

// stableHash fingerprints an entry's inputs so that a changed entry admits a new cursor.
func stableHash(m resource.PropertyMap) string {
	b, err := json.Marshal(m.Mappable())
	contract.AssertNoErrorf(err, "entry inputs are always serializable")
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// -- Parsing --

func parseWorkflowGraph(news resource.PropertyMap) (*wfGraph, error) {
	g := &wfGraph{nodes: map[string]*wfCallback{}, entries: map[string]wfEntry{}}

	nodes, err := propObject(news, "nodes")
	if err != nil {
		return nil, err
	}
	for name, v := range nodes {
		if name == "" || strings.ContainsAny(string(name), "/:#") {
			return nil, fmt.Errorf("invalid node name %q", name)
		}
		if !v.IsObject() {
			return nil, fmt.Errorf("node %q: must be an object", name)
		}
		var program *wfCallback
		if p, ok := v.ObjectValue()["program"]; ok && !p.IsNull() {
			cb, err := parseCallback(p)
			if err != nil {
				return nil, fmt.Errorf("node %q: %w", name, err)
			}
			program = &cb
		}
		g.nodes[string(name)] = program
	}

	if edges, ok := news["edges"]; ok {
		if !edges.IsArray() {
			return nil, errors.New(`property "edges" must be an array`)
		}
		for i, v := range edges.ArrayValue() {
			e, err := parseEdge(v, g.nodes)
			if err != nil {
				return nil, fmt.Errorf("edge %d: %w", i, err)
			}
			g.edges = append(g.edges, e)
		}
	}

	if entries, ok := news["entries"]; ok {
		if !entries.IsObject() {
			return nil, errors.New(`property "entries" must be an object`)
		}
		for name, v := range entries.ObjectValue() {
			if !v.IsObject() {
				return nil, fmt.Errorf("entry %q must be an object", name)
			}
			o := v.ObjectValue()
			node, err := propString(o, "node")
			if err != nil {
				return nil, fmt.Errorf("entry %q: %w", name, err)
			}
			if _, ok := g.nodes[node]; !ok {
				return nil, fmt.Errorf("entry %q: unknown node %q", name, node)
			}
			inputs, err := propObject(o, "inputs")
			if err != nil {
				return nil, fmt.Errorf("entry %q: %w", name, err)
			}
			if inputs.ContainsUnknowns() {
				return nil, fmt.Errorf("entry %q: inputs must be known", name)
			}
			g.entries[string(name)] = wfEntry{Node: node, Inputs: inputs}
		}
	}
	return g, nil
}

func parseEdge(v resource.PropertyValue, nodes map[string]*wfCallback) (wfEdge, error) {
	if !v.IsObject() {
		return wfEdge{}, errors.New("must be an object")
	}
	o := v.ObjectValue()
	name, err := propString(o, "name")
	if err != nil {
		return wfEdge{}, err
	}
	to, err := propString(o, "to")
	if err != nil {
		return wfEdge{}, err
	}
	if _, ok := nodes[to]; !ok {
		return wfEdge{}, fmt.Errorf("unknown node %q", to)
	}
	kind, err := propString(o, "kind")
	if err != nil {
		return wfEdge{}, err
	}
	e := wfEdge{Name: name, To: to, Kind: kind}
	switch kind {
	case "single", "and", "or":
		if e.From, err = propString(o, "from"); err != nil {
			return wfEdge{}, err
		}
		if _, ok := nodes[e.From]; !ok {
			return wfEdge{}, fmt.Errorf("unknown node %q", e.From)
		}
		conds, err := propObject(o, "conditions")
		if err != nil {
			return wfEdge{}, err
		}
		if len(conds) == 0 || (kind == "single" && len(conds) != 1) {
			return wfEdge{}, fmt.Errorf("edge %q: unexpected number of conditions", name)
		}
		e.Conditions = map[string]wfCallback{}
		for cond, cv := range conds {
			cb, err := parseCallback(cv)
			if err != nil {
				return wfEdge{}, fmt.Errorf("condition %q: %w", cond, err)
			}
			e.Conditions[string(cond)] = cb
		}
	case "join":
		branches, err := propObject(o, "branches")
		if err != nil {
			return wfEdge{}, err
		}
		if len(branches) == 0 {
			return wfEdge{}, fmt.Errorf("join %q: at least one branch is required", name)
		}
		e.Branches = map[string]*wfCallback{}
		for from, bv := range branches {
			if _, ok := nodes[string(from)]; !ok {
				return wfEdge{}, fmt.Errorf("unknown node %q", from)
			}
			if bv.IsNull() {
				e.Branches[string(from)] = nil
				continue
			}
			cb, err := parseCallback(bv)
			if err != nil {
				return wfEdge{}, fmt.Errorf("branch %q: %w", from, err)
			}
			e.Branches[string(from)] = &cb
		}
		merge, ok := o["merge"]
		if !ok {
			return wfEdge{}, fmt.Errorf("join %q: a merge callback is required", name)
		}
		if e.Merge, err = parseCallback(merge); err != nil {
			return wfEdge{}, fmt.Errorf("merge: %w", err)
		}
	default:
		return wfEdge{}, fmt.Errorf("unknown edge kind %q", kind)
	}
	return e, nil
}

func parseCallback(v resource.PropertyValue) (wfCallback, error) {
	if !v.IsObject() {
		return wfCallback{}, errors.New("callback must be an object")
	}
	o := v.ObjectValue()
	target, err := propString(o, "target")
	if err != nil {
		return wfCallback{}, err
	}
	token, err := propString(o, "token")
	if err != nil {
		return wfCallback{}, err
	}
	return wfCallback{Target: target, Token: token}, nil
}

func parseWorkflowState(olds resource.PropertyMap) (*wfState, error) {
	st := &wfState{Entries: map[string]wfSeed{}}
	v, ok := olds["state"]
	if !ok || !v.IsString() {
		return st, nil
	}
	if err := json.Unmarshal([]byte(v.StringValue()), st); err != nil {
		return nil, fmt.Errorf("invalid workflow state: %w", err)
	}
	if st.Entries == nil {
		st.Entries = map[string]wfSeed{}
	}
	return st, nil
}

func propObject(m resource.PropertyMap, key string) (resource.PropertyMap, error) {
	v, ok := m[resource.PropertyKey(key)]
	if !ok {
		return nil, fmt.Errorf("missing required property %q", key)
	}
	if !v.IsObject() {
		return nil, fmt.Errorf("property %q must be an object", key)
	}
	return v.ObjectValue(), nil
}

func propString(m resource.PropertyMap, key string) (string, error) {
	v, ok := m[resource.PropertyKey(key)]
	if !ok {
		return "", fmt.Errorf("missing required property %q", key)
	}
	if !v.IsString() {
		return "", fmt.Errorf("property %q must be a string", key)
	}
	return v.StringValue(), nil
}
