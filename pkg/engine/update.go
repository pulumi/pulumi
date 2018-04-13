// Copyright 2018, Pulumi Corporation.  All rights reserved.

package engine

import (
	"time"

	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/resource/plugin"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/workspace"
)

// UpdateOptions contains all the settings for customizing how an update (deploy, preview, or destroy) is performed.
type UpdateOptions struct {
	Analyzers []string // an optional set of analyzers to run as part of this deployment.
	DryRun    bool     // true if we should just print the plan without performing it.
	Parallel  int      // the degree of parallelism for resource operations (<=1 for serial).
	Debug     bool     // true if debugging output it enabled
}

// ResourceChanges contains the aggregate resource changes by operation type.
type ResourceChanges map[deploy.StepOp]int

func Update(u UpdateInfo, manager SnapshotManager,
	events chan<- Event, opts UpdateOptions) (ResourceChanges, error) {
	contract.Require(u != nil, "update")
	contract.Require(events != nil, "events")

	defer func() { events <- cancelEvent() }()

	ctx, err := newPlanContext(u, manager)
	if err != nil {
		return nil, err
	}
	defer ctx.Close()

	emitter := makeEventEmitter(events, u)
	return update(ctx, planOptions{
		UpdateOptions: opts,
		SourceFunc:    newUpdateSource,
		Events:        emitter,
		Diag:          newEventSink(emitter),
	})
}

func newUpdateSource(opts planOptions, proj *workspace.Project, pwd, main string,
	target *deploy.Target, plugctx *plugin.Context) (deploy.Source, error) {
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
	}, opts.DryRun), nil
}

func update(info *planContext, opts planOptions) (ResourceChanges, error) {
	result, err := plan(info, opts)
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

		if opts.DryRun {
			// If a dry run, just print the plan, don't actually carry out the deployment.
			resourceChanges, err = printPlan(result)
			if err != nil {
				return resourceChanges, err
			}
		} else {
			// Otherwise, we will actually deploy the latest bits.
			opts.Events.preludeEvent(opts.DryRun, result.Ctx.Update.GetTarget().Config)

			// Walk the plan, reporting progress and executing the actual operations as we go.
			start := time.Now()
			actions := newUpdateActions(info.Update, info.SnapshotManager, opts)
			summary, _, _, err := result.Walk(actions, false)
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

	if !opts.Diag.Success() {
		// If any error that wasn't printed above, be sure to make it evident in the output.
		return resourceChanges, errors.New("One or more errors occurred during this update")
	}
	return resourceChanges, nil
}

// updateActions pretty-prints the plan application process as it goes.
type updateActions struct {
	Steps           int
	Ops             map[deploy.StepOp]int
	Seen            map[resource.URN]deploy.Step
	MaybeCorrupt    bool
	Update          UpdateInfo
	Opts            planOptions
	SnapshotManager SnapshotManager
}

func newUpdateActions(u UpdateInfo, manager SnapshotManager, opts planOptions) *updateActions {
	return &updateActions{
		Ops:             make(map[deploy.StepOp]int),
		Seen:            make(map[resource.URN]deploy.Step),
		Update:          u,
		Opts:            opts,
		SnapshotManager: manager,
	}
}

func (acts *updateActions) OnResourceStepPre(step deploy.Step) (interface{}, error) {
	// Ensure we've marked this step as observed.
	acts.Seen[step.URN()] = step

	acts.Opts.Events.resourcePreEvent(step, false /*planning*/, acts.Opts.Debug)

	// Inform the snapshot service that we are about to perform a step.
	return acts.SnapshotManager.BeginMutation(step)
}

func (acts *updateActions) OnResourceStepPost(ctx interface{},
	step deploy.Step, status resource.Status, err error) error {

	assertSeen(acts.Seen, step)

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

		// Also show outputs here, since there might be some from the initial registration.
		acts.Opts.Events.resourceOutputsEvent(step, false /*planning*/, acts.Opts.Debug)
		return ctx.(SnapshotMutation).End(step)
	}

	return nil
}

func (acts *updateActions) OnResourceOutputs(step deploy.Step) error {
	assertSeen(acts.Seen, step)

	acts.Opts.Events.resourceOutputsEvent(step, false /* planning */, acts.Opts.Debug)
	return acts.SnapshotManager.RegisterResourceOutputs(step)
}
