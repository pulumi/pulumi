// Copyright 2017, Pulumi Corporation.  All rights reserved.

package engine

import (
	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

func Preview(update Update, events chan<- Event, opts UpdateOptions) error {
	contract.Require(update != nil, "update")
	contract.Require(events != nil, "events")

	defer func() { events <- cancelEvent() }()

	info, err := planContextFromUpdate(update)
	if err != nil {
		return err
	}
	defer info.Close()

	// Always set opts.DryRun to `true` when processing previews: if we do not do this, the engine will assume that it
	// should elide unknown input/output properties when interacting with the language and resource providers and we
	// will produce unexpected results.
	opts.DryRun = true
	return previewLatest(info, deployOptions{
		UpdateOptions: opts,

		Destroy: false,

		Events: events,
		Diag:   newEventSink(events),
	})
}

func previewLatest(info *planContext, opts deployOptions) error {
	result, err := plan(info, opts)
	if err != nil {
		return err
	}
	if result != nil {
		defer contract.IgnoreClose(result)

		// Make the current working directory the same as the program's, and restore it upon exit.
		done, err := result.Chdir()
		if err != nil {
			return err
		}
		defer done()

		if _, err := printPlan(result); err != nil {
			return err
		}
	}
	if !opts.Diag.Success() {
		// If any error occurred while walking the plan, be sure to let the developer know.  Otherwise,
		// although error messages may have spewed to the output, the final lines of the plan may look fine.
		return errors.New("One or more errors occurred during the creation of this preview")
	}
	return nil
}

type previewActions struct {
	Ops  map[deploy.StepOp]int
	Opts deployOptions
	Seen map[resource.URN]deploy.Step
}

func newPreviewActions(opts deployOptions) *previewActions {
	return &previewActions{
		Ops:  make(map[deploy.StepOp]int),
		Opts: opts,
		Seen: make(map[resource.URN]deploy.Step),
	}
}

func (acts *previewActions) OnResourceStepPre(step deploy.Step) (interface{}, error) {
	acts.Seen[step.URN()] = step

	indent := stepParentIndent(step, acts.Seen)
	summary := getResourcePropertiesSummary(step, indent)
	details := getResourcePropertiesDetails(step, indent, true, acts.Opts.Debug)
	acts.Opts.Events <- resourcePreEvent(step, indent, summary, details)

	return nil, nil
}

func (acts *previewActions) OnResourceStepPost(ctx interface{},
	step deploy.Step, status resource.Status, err error) error {
	assertSeen(acts.Seen, step)

	// We let `printPlan` handle error reporting for now.
	if err == nil {
		// Track the operation if shown and/or if it is a logically meaningful operation.
		if step.Logical() {
			acts.Ops[step.Op()]++
		}

		_ = acts.OnResourceOutputs(step)
	}
	return nil
}

func (acts *previewActions) OnResourceOutputs(step deploy.Step) error {
	assertSeen(acts.Seen, step)

	indent := stepParentIndent(step, acts.Seen)
	text := getResourceOutputsPropertiesString(step, indent, true, acts.Opts.Debug)
	acts.Opts.Events <- resourceOutputsEvent(step, indent, text)
	return nil
}
