// Copyright 2016-2018, Pulumi Corporation.
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
	"sync"

	"github.com/opentracing/opentracing-go"
	"github.com/pulumi/pulumi/pkg/v2/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v2/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v2/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/fsutil"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v2/go/common/workspace"
)

// ProjectInfoContext returns information about the current project, including its pwd, main, and plugin context.
func ProjectInfoContext(projinfo *Projinfo, host plugin.Host, config plugin.ConfigSource,
	diag, statusDiag diag.Sink, tracingSpan opentracing.Span) (string, string, *plugin.Context, error) {
	contract.Require(projinfo != nil, "projinfo")

	// If the package contains an override for the main entrypoint, use it.
	pwd, main, err := projinfo.GetPwdMain()
	if err != nil {
		return "", "", nil, err
	}

	// Create a context for plugins.
	ctx, err := plugin.NewContext(diag, statusDiag, host, config, pwd,
		projinfo.Proj.Runtime.Options(), tracingSpan)
	if err != nil {
		return "", "", nil, err
	}

	return pwd, main, ctx, nil
}

// newPlanContext creates a context for a subsequent planning operation.  Callers must call Close on the
// resulting context object once they have completed the associated planning operation.
func newPlanContext(u UpdateInfo, opName string, parentSpan opentracing.SpanContext) (*planContext, error) {
	contract.Require(u != nil, "u")

	// Create a root span for the operation
	opts := []opentracing.StartSpanOption{}
	if opName != "" {
		opts = append(opts, opentracing.Tag{Key: "operation", Value: opName})
	}
	if parentSpan != nil {
		opts = append(opts, opentracing.ChildOf(parentSpan))
	}
	tracingSpan := opentracing.StartSpan("pulumi-plan", opts...)

	return &planContext{
		Update:      u,
		TracingSpan: tracingSpan,
	}, nil
}

type planContext struct {
	Update      UpdateInfo       // The update being processed.
	TracingSpan opentracing.Span // An OpenTracing span to parent plan operations within.
}

func (ctx *planContext) Close() {
	ctx.TracingSpan.Finish()
}

// planOptions includes a full suite of options for performing a plan and/or deploy operation.
type planOptions struct {
	UpdateOptions

	// SourceFunc is a factory that returns an EvalSource to use during planning.  This is the thing that
	// creates resources to compare against the current checkpoint state (e.g., by evaluating a program, etc).
	SourceFunc planSourceFunc

	DOT        bool         // true if we should print the DOT file for this plan.
	Events     eventEmitter // the channel to write events from the engine to.
	Diag       diag.Sink    // the sink to use for diag'ing.
	StatusDiag diag.Sink    // the sink to use for diag'ing status messages.

	// true if we're planning a refresh.
	isRefresh bool

	// true if we should trust the dependency graph reported by the language host. Not all Pulumi-supported languages
	// correctly report their dependencies, in which case this will be false.
	trustDependencies bool
}

// planSourceFunc is a callback that will be used to prepare for, and evaluate, the "new" state for a stack.
type planSourceFunc func(
	client deploy.BackendClient, opts planOptions, proj *workspace.Project, pwd, main string,
	target *deploy.Target, plugctx *plugin.Context, dryRun bool) (deploy.Source, error)

// plan just uses the standard logic to parse arguments, options, and to create a snapshot and plan.
func plan(ctx *Context, info *planContext, opts planOptions, dryRun bool) (*planResult, error) {
	contract.Assert(info != nil)
	contract.Assert(info.Update != nil)
	contract.Assert(opts.SourceFunc != nil)

	// First, load the package metadata and the deployment target in preparation for executing the package's program
	// and creating resources.  This includes fetching its pwd and main overrides.
	proj, target := info.Update.GetProject(), info.Update.GetTarget()
	contract.Assert(proj != nil)
	contract.Assert(target != nil)
	projinfo := &Projinfo{Proj: proj, Root: info.Update.GetRoot()}
	pwd, main, plugctx, err := ProjectInfoContext(projinfo, opts.host, target,
		opts.Diag, opts.StatusDiag, info.TracingSpan)
	if err != nil {
		return nil, err
	}

	opts.trustDependencies = proj.TrustResourceDependencies()
	// Now create the state source.  This may issue an error if it can't create the source.  This entails,
	// for example, loading any plugins which will be required to execute a program, among other things.
	source, err := opts.SourceFunc(ctx.BackendClient, opts, proj, pwd, main, target, plugctx, dryRun)
	if err != nil {
		contract.IgnoreClose(plugctx)
		return nil, err
	}

	// Generate a plan; this API handles all interesting cases (create, update, delete).
	localPolicyPackPaths := ConvertLocalPolicyPacksToPaths(opts.LocalPolicyPacks)
	plan, err := deploy.NewPlan(
		plugctx, target, target.Snapshot, source, localPolicyPackPaths, dryRun, ctx.BackendClient)
	if err != nil {
		contract.IgnoreClose(plugctx)
		return nil, err
	}
	return &planResult{
		Ctx:     info,
		Plugctx: plugctx,
		Plan:    plan,
		Options: opts,
	}, nil
}

type planResult struct {
	Ctx     *planContext    // plan context information.
	Plugctx *plugin.Context // the context containing plugins and their state.
	Plan    *deploy.Plan    // the plan created by this command.
	Options planOptions     // the options used during planning.
}

// Chdir changes the directory so that all operations from now on are relative to the project we are working with.
// It returns a function that, when run, restores the old working directory.
func (planResult *planResult) Chdir() (func(), error) {
	return fsutil.Chdir(planResult.Plugctx.Pwd)
}

// Walk enumerates all steps in the plan, calling out to the provided action at each step.  It returns four things: the
// resulting Snapshot, no matter whether an error occurs or not; an error, if something went wrong; the step that
// failed, if the error is non-nil; and finally the state of the resource modified in the failing step.
func (planResult *planResult) Walk(cancelCtx *Context, events deploy.Events, preview bool) result.Result {
	ctx, cancelFunc := context.WithCancel(context.Background())

	// Inject our opentracing span into the context.
	if planResult.Ctx.TracingSpan != nil {
		ctx = opentracing.ContextWithSpan(ctx, planResult.Ctx.TracingSpan)
	}

	done := make(chan bool)
	var walkResult result.Result
	go func() {
		opts := deploy.Options{
			Events:            events,
			Parallel:          planResult.Options.Parallel,
			Refresh:           planResult.Options.Refresh,
			RefreshOnly:       planResult.Options.isRefresh,
			RefreshTargets:    planResult.Options.RefreshTargets,
			ReplaceTargets:    planResult.Options.ReplaceTargets,
			DestroyTargets:    planResult.Options.DestroyTargets,
			UpdateTargets:     planResult.Options.UpdateTargets,
			TargetDependents:  planResult.Options.TargetDependents,
			TrustDependencies: planResult.Options.trustDependencies,
			UseLegacyDiff:     planResult.Options.UseLegacyDiff,
		}
		walkResult = planResult.Plan.Execute(ctx, opts, preview)
		close(done)
	}()

	// Asynchronously listen for cancellation, and deliver that signal to plan.
	go func() {
		select {
		case <-cancelCtx.Cancel.Canceled():
			// Cancel the plan's execution context, so it begins to shut down.
			cancelFunc()
		case <-done:
			return
		}
	}()

	select {
	case <-cancelCtx.Cancel.Terminated():
		return result.WrapIfNonNil(cancelCtx.Cancel.TerminateErr())

	case <-done:
		return walkResult
	}
}

func (planResult *planResult) Close() error {
	return planResult.Plugctx.Close()
}

// printPlan prints the plan's result to the plan's Options.Events stream.
func printPlan(ctx *Context, planResult *planResult, dryRun bool, policies map[string]string,
) (ResourceChanges, result.Result) {

	planResult.Options.Events.preludeEvent(dryRun, planResult.Ctx.Update.GetTarget().Config)

	// Walk the plan's steps and and pretty-print them out.
	actions := newPlanActions(planResult.Options)
	res := planResult.Walk(ctx, actions, true)

	// Emit an event with a summary of operation counts.
	changes := ResourceChanges(actions.Ops)
	planResult.Options.Events.previewSummaryEvent(changes, policies)

	if res != nil {

		if res.IsBail() {
			return nil, res
		}

		return nil, result.Error("an error occurred while advancing the preview")
	}

	return changes, nil
}

type planActions struct {
	Ops     map[deploy.StepOp]int
	Opts    planOptions
	Seen    map[resource.URN]deploy.Step
	MapLock sync.Mutex
}

func shouldReportStep(step deploy.Step, opts planOptions) bool {
	return step.Op() != deploy.OpRemovePendingReplace &&
		(opts.reportDefaultProviderSteps || !isDefaultProviderStep(step))
}

func newPlanActions(opts planOptions) *planActions {
	return &planActions{
		Ops:  make(map[deploy.StepOp]int),
		Opts: opts,
		Seen: make(map[resource.URN]deploy.Step),
	}
}

func (acts *planActions) OnResourceStepPre(step deploy.Step) (interface{}, error) {
	acts.MapLock.Lock()
	acts.Seen[step.URN()] = step
	acts.MapLock.Unlock()

	// Skip reporting if necessary.
	if !shouldReportStep(step, acts.Opts) {
		return nil, nil
	}

	acts.Opts.Events.resourcePreEvent(step, true /*planning*/, acts.Opts.Debug)

	return nil, nil
}

func (acts *planActions) OnResourceStepPost(ctx interface{},
	step deploy.Step, status resource.Status, err error) error {
	acts.MapLock.Lock()
	assertSeen(acts.Seen, step)
	acts.MapLock.Unlock()

	reportStep := shouldReportStep(step, acts.Opts)

	if err != nil {
		// We always want to report a failure. If we intend to elide this step overall, though, we report it as a
		// global message.
		reportedURN := resource.URN("")
		if reportStep {
			reportedURN = step.URN()
		}

		acts.Opts.Diag.Errorf(diag.GetPreviewFailedError(reportedURN), err)
	} else if reportStep {
		op, record := step.Op(), step.Logical()
		if acts.Opts.isRefresh && op == deploy.OpRefresh {
			// Refreshes are handled specially.
			op, record = step.(*deploy.RefreshStep).ResultOp(), true
		}

		if step.Op() == deploy.OpRead {
			record = ShouldRecordReadStep(step)
		}

		// Track the operation if shown and/or if it is a logically meaningful operation.
		if record {
			acts.MapLock.Lock()
			acts.Ops[op]++
			acts.MapLock.Unlock()
		}

		acts.Opts.Events.resourceOutputsEvent(op, step, true /*planning*/, acts.Opts.Debug)
	}

	return nil
}

func ShouldRecordReadStep(step deploy.Step) bool {
	contract.Assertf(step.Op() == deploy.OpRead, "Only call this on a Read step")

	// If reading a resource didn't result in any change to the resource, we then want to
	// record this as a 'same'.  That way, when things haven't actually changed, but a user
	// app did any 'reads' these don't show up in the resource summary at the end.
	return step.Old() != nil &&
		step.New() != nil &&
		step.Old().Outputs != nil &&
		step.New().Outputs != nil &&
		step.Old().Outputs.Diff(step.New().Outputs) != nil
}

func (acts *planActions) OnResourceOutputs(step deploy.Step) error {
	acts.MapLock.Lock()
	assertSeen(acts.Seen, step)
	acts.MapLock.Unlock()

	// Skip reporting if necessary.
	if !shouldReportStep(step, acts.Opts) {
		return nil
	}

	// Print the resource outputs separately.
	acts.Opts.Events.resourceOutputsEvent(step.Op(), step, true /*planning*/, acts.Opts.Debug)

	return nil
}

func (acts *planActions) OnPolicyViolation(urn resource.URN, d plugin.AnalyzeDiagnostic) {
	acts.Opts.Events.policyViolationEvent(urn, d)
}

func assertSeen(seen map[resource.URN]deploy.Step, step deploy.Step) {
	_, has := seen[step.URN()]
	contract.Assertf(has, "URN '%v' had not been marked as seen", step.URN())
}

func isDefaultProviderStep(step deploy.Step) bool {
	return providers.IsDefaultProvider(step.URN())
}
