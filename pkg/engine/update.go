// Copyright 2018, Pulumi Corporation.  All rights reserved.

package engine

import (
	"time"

	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/resource/plugin"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/workspace"
)

// UpdateOptions contains all the settings for customizing how an update (deploy, preview, or destroy) is performed.
type UpdateOptions struct {
	// an optional set of analyzers to run as part of this deployment.
	Analyzers []string

	// true if we should just perform the update, without any previewing or request for confirmation.
	// Not valid with 'Preview'.
	Force bool

	// true if we should just show the preview and then immediately quit. Not valid with 'force'.
	Preview bool

	// the degree of parallelism for resource operations (<=1 for serial).
	Parallel int

	// true if debugging output it enabled
	Debug bool
}

// ResourceChanges contains the aggregate resource changes by operation type.
type ResourceChanges map[deploy.StepOp]int

func Update(u UpdateInfo, ctx *Context, opts UpdateOptions, dryRun bool) (ResourceChanges, error) {
	contract.Require(u != nil, "update")
	contract.Require(ctx != nil, "ctx")

	defer func() { ctx.Events <- cancelEvent() }()

	info, err := newPlanContext(u)
	if err != nil {
		return nil, err
	}
	defer info.Close()

	emitter := makeEventEmitter(ctx.Events, u)
	return update(ctx, info, planOptions{
		UpdateOptions: opts,
		SourceFunc:    newUpdateSource,
		Events:        emitter,
		Diag:          newEventSink(emitter),
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
	})
	if err != nil {
		return nil, err
	}

	// Now ensure that we have loaded up any plugins that the program will need in advance.
	if err = plugctx.Host.EnsurePlugins(plugins); err != nil {
		return nil, err
	}

	// If that succeeded, create a new source that will perform interpretation of the compiled program.
	// TODO[pulumi/pulumi#88]: we are passing `nil` as the arguments map; we need to allow a way to pass these.
	return deploy.NewEvalSource(plugctx, &deploy.EvalRunInfo{
		Proj:    proj,
		Pwd:     pwd,
		Program: main,
		Target:  target,
	}, dryRun), nil
}

func update(ctx *Context, info *planContext, opts planOptions, dryRun bool) (ResourceChanges, error) {
	result, err := plan(info, opts, dryRun)
	if err != nil {
		return nil, err
	}

	var resourceChanges ResourceChanges
	if result != nil {
		defer contract.IgnoreClose(result)

		// Make the current working directory the same as the program's, and restore it upon exit.
		done, err := result.Chdir()
		if err != nil {
			return nil, err
		}
		defer done()

		if dryRun {
			// If a dry run, just print the plan, don't actually carry out the deployment.
			resourceChanges, err = printPlan(ctx, result, dryRun)
			if err != nil {
				return resourceChanges, err
			}
		} else {
			// Otherwise, we will actually deploy the latest bits.
			opts.Events.preludeEvent(dryRun, result.Ctx.Update.GetTarget().Config)

			// Walk the plan, reporting progress and executing the actual operations as we go.
			start := time.Now()
			actions := newUpdateActions(ctx, info.Update, opts)
			summary, _, _, err := result.Walk(ctx, actions, false)
			if err != nil && summary == nil {
				// Something went wrong, and no changes were made.
				return resourceChanges, err
			}
			contract.Assert(summary != nil)

			// Print out the total number of steps performed (and their kinds), the duration, and any summary info.
			resourceChanges = ResourceChanges(actions.Ops)
			opts.Events.updateSummaryEvent(actions.MaybeCorrupt, time.Since(start), resourceChanges)

			if err != nil {
				return resourceChanges, err
			}
		}
	}

	return resourceChanges, nil
}

// updateActions pretty-prints the plan application process as it goes.
type updateActions struct {
	Context      *Context
	Steps        int
	Ops          map[deploy.StepOp]int
	Seen         map[resource.URN]deploy.Step
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
	acts.Seen[step.URN()] = step

	acts.Opts.Events.resourcePreEvent(step, false /*planning*/, acts.Opts.Debug)

	// Inform the snapshot service that we are about to perform a step.
	return acts.Context.SnapshotManager.BeginMutation()
}

func (acts *updateActions) OnResourceStepPost(ctx interface{},
	step deploy.Step, status resource.Status, err error) error {

	assertSeen(acts.Seen, step)

	// If we've already been terminated, exit without writing the checkpoint. We explicitly want to leave the
	// checkpoint in an inconsistent state in this event.
	if acts.Context.Cancel.TerminateErr() != nil {
		return nil
	}

	// Report the result of the step.
	stepop := step.Op()
	if err != nil {
		if status == resource.StatusUnknown {
			acts.MaybeCorrupt = true
		}

		// Issue a true, bonafide error.
		acts.Opts.Diag.Errorf(diag.GetPlanApplyFailedError(step.URN()), err)
		acts.Opts.Events.resourceOperationFailedEvent(step, status, acts.Steps, acts.Opts.Debug)
	} else {
		if step.Logical() {
			// Increment the counters.
			acts.Steps++
			acts.Ops[stepop]++
		}

		// Also show outputs here for custom resources, since there might be some from the initial registration. We do
		// not show outputs for component resources at this point: any that exist must be from a previous execution of
		// the Pulumi program, as component resources only report outputs via calls to RegisterResourceOutputs.
		if step.Res().Custom {
			acts.Opts.Events.resourceOutputsEvent(step, false /*planning*/, acts.Opts.Debug)
		}
	}

	// Write out the current snapshot. Note that even if a failure has occurred, we should still have a
	// safe checkpoint.  Note that any error that occurs when writing the checkpoint trumps the error reported above.
	return ctx.(SnapshotMutation).End(step.Iterator().Snap())
}

func (acts *updateActions) OnResourceOutputs(step deploy.Step) error {
	assertSeen(acts.Seen, step)

	acts.Opts.Events.resourceOutputsEvent(step, false /*planning*/, acts.Opts.Debug)

	// There's a chance there are new outputs that weren't written out last time.
	// We need to perform another snapshot write to ensure they get written out.
	mutation, err := acts.Context.SnapshotManager.BeginMutation()
	if err != nil {
		return err
	}

	return mutation.End(step.Iterator().Snap())
}
