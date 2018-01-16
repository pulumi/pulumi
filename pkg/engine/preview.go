// Copyright 2017, Pulumi Corporation.  All rights reserved.

package engine

import (
	"bytes"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/diag"
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

	return previewLatest(info, deployOptions{
		UpdateOptions: opts,

		Create:  false,
		Destroy: false,

		Diag: newEventSink(events, diag.FormatOptions{
			Color: opts.Color,
		}),
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

		if err := printPlan(result); err != nil {
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
	Ops   map[deploy.StepOp]int
	Opts  deployOptions
	Seen  map[resource.URN]deploy.Step
	Shown map[resource.URN]bool
}

func newPreviewActions(opts deployOptions) *previewActions {
	return &previewActions{
		Ops:   make(map[deploy.StepOp]int),
		Opts:  opts,
		Seen:  make(map[resource.URN]deploy.Step),
		Shown: make(map[resource.URN]bool),
	}
}

func (acts *previewActions) OnResourceStepPre(step deploy.Step) (interface{}, error) {
	// Print this step information (resource and all its properties).
	if shouldShow(acts.Seen, step, acts.Opts) || isRootStack(step) {
		var b bytes.Buffer
		printStep(&b, step,
			acts.Seen, acts.Shown, acts.Opts.Summary, acts.Opts.Detailed, true, 0 /*indent*/)
		acts.Opts.Events <- stdOutEventWithColor(&b, acts.Opts.Color)
	}
	return nil, nil
}

func (acts *previewActions) OnResourceStepPost(ctx interface{},
	step deploy.Step, status resource.Status, err error) error {
	// We let `printPlan` handle error reporting for now.
	if err == nil {
		// Track the operation if shown and/or if it is a logically meaningful operation.
		if step.Logical() {
			acts.Ops[step.Op()]++
		}

		// Also show outputs here, since there might be some from the initial registration.
		if shouldShow(acts.Seen, step, acts.Opts) && !acts.Opts.Summary {
			_ = acts.OnResourceOutputs(step)
		}
	}
	return nil
}

func (acts *previewActions) OnResourceOutputs(step deploy.Step) error {
	// Print this step's output properties.
	if (shouldShow(acts.Seen, step, acts.Opts) || isRootStack(step)) && !acts.Opts.Summary {
		var b bytes.Buffer
		printResourceOutputProperties(&b, step, acts.Seen, acts.Shown, true, 0 /*indent*/)
		acts.Opts.Events <- stdOutEventWithColor(&b, acts.Opts.Color)
	}
	return nil
}
