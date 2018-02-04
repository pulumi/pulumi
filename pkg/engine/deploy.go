// Copyright 2017, Pulumi Corporation.  All rights reserved.

package engine

import (
	"time"

	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/util/contract"
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

func Deploy(update Update, events chan<- Event, opts UpdateOptions) (ResourceChanges, error) {
	contract.Require(update != nil, "update")
	contract.Require(events != nil, "events")

	defer func() { events <- cancelEvent() }()

	info, err := planContextFromUpdate(update)
	if err != nil {
		return nil, err
	}
	defer info.Close()

	return deployLatest(info, deployOptions{
		UpdateOptions: opts,

		Destroy: false,

		Events: events,
		Diag:   newEventSink(events),
	})
}

type deployOptions struct {
	UpdateOptions

	Destroy bool // true if we are destroying the stack.

	DOT    bool         // true if we should print the DOT file for this plan.
	Events chan<- Event // the channel to write events from the engine to.
	Diag   diag.Sink    // the sink to use for diag'ing.
}

func deployLatest(info *planContext, opts deployOptions) (ResourceChanges, error) {
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
			opts.Events <- preludeEvent(opts.DryRun, result.Info.Update.GetTarget().Config)

			// Walk the plan, reporting progress and executing the actual operations as we go.
			start := time.Now()
			actions := newDeployActions(info.Update, opts)
			summary, _, _, err := result.Walk(actions, false)
			if err != nil && summary == nil {
				// Something went wrong, and no changes were made.
				return resourceChanges, err
			}
			contract.Assert(summary != nil)

			// Print out the total number of steps performed (and their kinds), the duration, and any summary info.
			resourceChanges = ResourceChanges(actions.Ops)
			opts.Events <- updateSummaryEvent(actions.MaybeCorrupt, time.Since(start), resourceChanges)

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

// deployActions pretty-prints the plan application process as it goes.
type deployActions struct {
	Steps        int
	Ops          map[deploy.StepOp]int
	Seen         map[resource.URN]deploy.Step
	MaybeCorrupt bool
	Update       Update
	Opts         deployOptions
}

func newDeployActions(update Update, opts deployOptions) *deployActions {
	return &deployActions{
		Ops:    make(map[deploy.StepOp]int),
		Seen:   make(map[resource.URN]deploy.Step),
		Update: update,
		Opts:   opts,
	}
}

func (acts *deployActions) OnResourceStepPre(step deploy.Step) (interface{}, error) {
	// Ensure we've marked this step as observed.
	acts.Seen[step.URN()] = step

	indent := getIndent(step, acts.Seen)
	summary := getResourcePropertiesSummary(step, indent)
	details := getResourcePropertiesDetails(step, indent, false, acts.Opts.Debug)
	acts.Opts.Events <- resourcePreEvent(step, indent, summary, details)

	// Inform the snapshot service that we are about to perform a step.
	return acts.Update.BeginMutation()
}

func (acts *deployActions) OnResourceStepPost(ctx interface{},
	step deploy.Step, status resource.Status, err error) error {
	assertSeen(acts.Seen, step)

	// Report the result of the step.
	stepop := step.Op()
	if err != nil {
		if status == resource.StatusUnknown {
			acts.MaybeCorrupt = true
		}

		// Issue a true, bonafide error.
		acts.Opts.Diag.Errorf(diag.ErrorPlanApplyFailed, err)
		acts.Opts.Events <- resourceOperationFailedEvent(step, status, acts.Steps)
	} else {
		if step.Logical() {
			// Increment the counters.
			acts.Steps++
			acts.Ops[stepop]++
		}

		// Also show outputs here, since there might be some from the initial registration.
		indent := getIndent(step, acts.Seen)
		text := getResourceOutputsPropertiesString(step, indent, false, acts.Opts.Debug)
		acts.Opts.Events <- resourceOutputsEvent(step, indent, text)
	}

	// Write out the current snapshot. Note that even if a failure has occurred, we should still have a
	// safe checkpoint.  Note that any error that occurs when writing the checkpoint trumps the error reported above.
	return ctx.(SnapshotMutation).End(step.Iterator().Snap())
}

func (acts *deployActions) OnResourceOutputs(step deploy.Step) error {
	assertSeen(acts.Seen, step)

	indent := getIndent(step, acts.Seen)
	text := getResourceOutputsPropertiesString(step, indent, false, acts.Opts.Debug)
	acts.Opts.Events <- resourceOutputsEvent(step, indent, text)

	// There's a chance there are new outputs that weren't written out last time.
	// We need to perform another snapshot write to ensure they get written out.
	mutation, err := acts.Update.BeginMutation()
	if err != nil {
		return err
	}

	return mutation.End(step.Iterator().Snap())
}
