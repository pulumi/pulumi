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
	"sync"
	"time"

	"github.com/blang/semver"

	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/resource/plugin"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/util/logging"
	"github.com/pulumi/pulumi/pkg/workspace"
)

// UpdateOptions contains all the settings for customizing how an update (deploy, preview, or destroy) is performed.
//
// This structre is embedded in another which uses some of the unexported fields, which trips up the `structcheck`
// linter.
// nolint: structcheck
type UpdateOptions struct {
	// an optional set of analyzers to run as part of this deployment.
	Analyzers []string

	// the degree of parallelism for resource operations (<=1 for serial).
	Parallel int

	// true if debugging output it enabled
	Debug bool

	// true if the plan should refresh before executing.
	Refresh bool

	// true if we should report events for steps that involve default providers.
	reportDefaultProviderSteps bool

	// the plugin host to use for this update
	host plugin.Host
}

// ResourceChanges contains the aggregate resource changes by operation type.
type ResourceChanges map[deploy.StepOp]int

// HasChanges returns true if there are any non-same changes in the resulting summary.
func (changes ResourceChanges) HasChanges() bool {
	var c int
	for op, count := range changes {
		if op != deploy.OpSame {
			c += count
		}
	}
	return c > 0
}

func Update(u UpdateInfo, ctx *Context, opts UpdateOptions, dryRun bool) (ResourceChanges, error) {
	contract.Require(u != nil, "update")
	contract.Require(ctx != nil, "ctx")

	defer func() { ctx.Events <- cancelEvent() }()

	info, err := newPlanContext(u, "update", ctx.ParentSpan)
	if err != nil {
		return nil, err
	}
	defer info.Close()

	emitter, err := makeEventEmitter(ctx.Events, u)
	if err != nil {
		return nil, err
	}
	return update(ctx, info, planOptions{
		UpdateOptions: opts,
		SourceFunc:    newUpdateSource,
		Events:        emitter,
		Diag:          newEventSink(emitter, false),
		StatusDiag:    newEventSink(emitter, true),
	}, dryRun)
}

func newUpdateSource(
	opts planOptions, proj *workspace.Project, pwd, main string,
	target *deploy.Target, plugctx *plugin.Context, dryRun bool) (deploy.Source, error) {

	// Figure out which plugins to load by inspecting the program contents.
	plugins, err := plugctx.Host.GetRequiredPlugins(plugin.ProgInfo{
		Proj:    proj,
		Pwd:     pwd,
		Program: main,
	}, plugin.AllPlugins)
	if err != nil {
		return nil, err
	}

	// Now ensure that we have loaded up any plugins that the program will need in advance.
	const kinds = plugin.AnalyzerPlugins | plugin.LanguagePlugins
	if err = plugctx.Host.EnsurePlugins(plugins, kinds); err != nil {
		return nil, err
	}

	// Collect the version information for default providers.
	defaultProviderVersions := make(map[tokens.Package]*semver.Version)
	for _, p := range plugins {
		if p.Kind != workspace.ResourcePlugin {
			continue
		}
		defaultProviderVersions[tokens.Package(p.Name)] = p.Version
	}

	// If that succeeded, create a new source that will perform interpretation of the compiled program.
	// TODO[pulumi/pulumi#88]: we are passing `nil` as the arguments map; we need to allow a way to pass these.
	return deploy.NewEvalSource(plugctx, &deploy.EvalRunInfo{
		Proj:    proj,
		Pwd:     pwd,
		Program: main,
		Target:  target,
	}, defaultProviderVersions, dryRun), nil
}

func update(ctx *Context, info *planContext, opts planOptions, dryRun bool) (ResourceChanges, error) {
	result, err := plan(ctx, info, opts, dryRun)
	if err != nil {
		return nil, err
	}

	var resourceChanges ResourceChanges
	if result != nil {
		defer contract.IgnoreClose(result)

		// Make the current working directory the same as the program's, and restore it upon exit.
		done, chErr := result.Chdir()
		if chErr != nil {
			return nil, chErr
		}
		defer done()

		if dryRun {
			// If a dry run, just print the plan, don't actually carry out the deployment.
			resourceChanges, err = printPlan(ctx, result, dryRun)
		} else {
			// Otherwise, we will actually deploy the latest bits.
			opts.Events.preludeEvent(dryRun, result.Ctx.Update.GetTarget().Config)

			// Walk the plan, reporting progress and executing the actual operations as we go.
			start := time.Now()
			actions := newUpdateActions(ctx, info.Update, opts)

			err = result.Walk(ctx, actions, false)
			resourceChanges = ResourceChanges(actions.Ops)

			if len(resourceChanges) != 0 {
				// Print out the total number of steps performed (and their kinds), the duration, and any summary info.
				opts.Events.updateSummaryEvent(actions.MaybeCorrupt, time.Since(start), resourceChanges)
			}
		}
	}
	return resourceChanges, err
}

// pluginActions listens for plugin events and persists the set of loaded plugins
// to the snapshot.
type pluginActions struct {
	Context *Context
}

func (p *pluginActions) OnPluginLoad(loadedPlug workspace.PluginInfo) error {
	return p.Context.SnapshotManager.RecordPlugin(loadedPlug)
}

// updateActions pretty-prints the plan application process as it goes.
type updateActions struct {
	Context      *Context
	Steps        int
	Ops          map[deploy.StepOp]int
	Seen         map[resource.URN]deploy.Step
	MapLock      sync.Mutex
	MaybeCorrupt bool
	Update       UpdateInfo
	Opts         planOptions
}

func newUpdateActions(context *Context, u UpdateInfo, opts planOptions) *updateActions {
	return &updateActions{
		Context: context,
		Ops:     make(map[deploy.StepOp]int),
		Seen:    make(map[resource.URN]deploy.Step),
		Update:  u,
		Opts:    opts,
	}
}

func (acts *updateActions) OnResourceStepPre(step deploy.Step) (interface{}, error) {
	// Ensure we've marked this step as observed.
	acts.MapLock.Lock()
	acts.Seen[step.URN()] = step
	acts.MapLock.Unlock()

	// Check for a default provider step and skip reporting if necessary.
	if acts.Opts.reportDefaultProviderSteps || !isDefaultProviderStep(step) {
		acts.Opts.Events.resourcePreEvent(step, false /*planning*/, acts.Opts.Debug)
	}

	// Inform the snapshot service that we are about to perform a step.
	return acts.Context.SnapshotManager.BeginMutation(step)
}

func (acts *updateActions) OnResourceStepPost(ctx interface{},
	step deploy.Step, status resource.Status, err error) error {
	acts.MapLock.Lock()
	assertSeen(acts.Seen, step)
	acts.MapLock.Unlock()

	// If we've already been terminated, exit without writing the checkpoint. We explicitly want to leave the
	// checkpoint in an inconsistent state in this event.
	if acts.Context.Cancel.TerminateErr() != nil {
		return nil
	}

	reportStep := acts.Opts.reportDefaultProviderSteps || !isDefaultProviderStep(step)

	// Report the result of the step.
	if err != nil {
		if status == resource.StatusUnknown {
			acts.MaybeCorrupt = true
		}

		errorURN := resource.URN("")
		if reportStep {
			errorURN = step.URN()
		}

		// Issue a true, bonafide error.
		acts.Opts.Diag.Errorf(diag.GetPlanApplyFailedError(errorURN), err)
		if reportStep {
			acts.Opts.Events.resourceOperationFailedEvent(step, status, acts.Steps, acts.Opts.Debug)
		}
	} else if reportStep {
		op, record := step.Op(), step.Logical()
		if acts.Opts.isRefresh && op == deploy.OpRefresh {
			// Refreshes are handled specially.
			op, record = step.(*deploy.RefreshStep).ResultOp(), true
		}

		if record {
			// Increment the counters.
			acts.MapLock.Lock()
			acts.Steps++
			acts.Ops[op]++
			acts.MapLock.Unlock()
		}

		// Also show outputs here for custom resources, since there might be some from the initial registration. We do
		// not show outputs for component resources at this point: any that exist must be from a previous execution of
		// the Pulumi program, as component resources only report outputs via calls to RegisterResourceOutputs.
		if step.Res().Custom || acts.Opts.Refresh && step.Op() == deploy.OpRefresh {
			acts.Opts.Events.resourceOutputsEvent(op, step, false /*planning*/, acts.Opts.Debug)
		}
	}

	// See pulumi/pulumi#2011 for details. Terraform always returns the existing state with the diff applied to it in
	// the event of an update failure. It's appropriate that we save this new state in the output of the resource, but
	// it is not appropriate to save the inputs, because the resource that exists was not created or updated
	// successfully with those inputs.
	//
	// If we were doing an update and got a `StatusPartialFailure`, the resource that ultimately gets persisted in the
	// snapshot should be old inputs and new outputs. We accomplish that here by clobbering the new resource's inputs
	// with the old inputs.
	//
	// This is a little kludgy given that these resources are global state. However, given the way that we have
	// implemented the snapshot manager and engine today, it's the easiest way to accomplish what we are trying to do.
	if status == resource.StatusPartialFailure && step.Op() == deploy.OpUpdate {
		logging.V(7).Infof(
			"OnResourceStepPost(%s): Step is partially-failed update, saving old inputs instead of new inputs",
			step.URN())
		new := step.New()
		old := step.Old()
		contract.Assert(new != nil)
		contract.Assert(old != nil)
		new.Inputs = make(resource.PropertyMap)
		for key, value := range old.Inputs {
			new.Inputs[key] = value
		}
	}

	// Write out the current snapshot. Note that even if a failure has occurred, we should still have a
	// safe checkpoint.  Note that any error that occurs when writing the checkpoint trumps the error
	// reported above.
	return ctx.(SnapshotMutation).End(step, err == nil || status == resource.StatusPartialFailure)
}

func (acts *updateActions) OnResourceOutputs(step deploy.Step) error {
	acts.MapLock.Lock()
	assertSeen(acts.Seen, step)
	acts.MapLock.Unlock()

	// Check for a default provider step and skip reporting if necessary.
	if acts.Opts.reportDefaultProviderSteps || !isDefaultProviderStep(step) {
		acts.Opts.Events.resourceOutputsEvent(step.Op(), step, false /*planning*/, acts.Opts.Debug)
	}

	// There's a chance there are new outputs that weren't written out last time.
	// We need to perform another snapshot write to ensure they get written out.
	return acts.Context.SnapshotManager.RegisterResourceOutputs(step)
}
