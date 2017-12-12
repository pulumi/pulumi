// Copyright 2017, Pulumi Corporation.  All rights reserved.

package engine

import (
	"bytes"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

type PreviewOptions struct {
	Package              string   // the package to compute the preview for
	Analyzers            []string // an optional set of analyzers to run as part of this deployment.
	Parallel             int      // the degree of parallelism for resource operations (<=1 for serial).
	ShowConfig           bool     // true to show the configuration variables being used.
	ShowReplacementSteps bool     // true to show the replacement steps in the plan.
	ShowSames            bool     // true to show the resources that aren't updated, in addition to those that are.
	Summary              bool     // true if we should only summarize resources and operations.
}

func (eng *Engine) Preview(stack tokens.QName, events chan<- Event, opts PreviewOptions) error {
	contract.Require(stack != tokens.QName(""), "stack")
	contract.Require(events != nil, "events")

	defer func() { events <- cancelEvent() }()

	info, err := eng.planContextFromStack(stack, opts.Package)
	if err != nil {
		return err
	}
	defer info.Close()

	return eng.previewLatest(info, deployOptions{
		Destroy:              false,
		DryRun:               true,
		Analyzers:            opts.Analyzers,
		Parallel:             opts.Parallel,
		ShowConfig:           opts.ShowConfig,
		ShowReplacementSteps: opts.ShowReplacementSteps,
		ShowSames:            opts.ShowSames,
		Summary:              opts.Summary,
		Events:               events,
		Diag: newEventSink(events, diag.FormatOptions{
			Colors: true,
		}),
	})
}

func (eng *Engine) previewLatest(info *planContext, opts deployOptions) error {
	result, err := eng.plan(info, opts)
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

		if err := eng.printPlan(result); err != nil {
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
		acts.Opts.Events <- stdOutEventWithColor(&b)
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
		printResourceOutputProperties(&b, step, acts.Seen, acts.Shown, 0 /*indent*/)
		acts.Opts.Events <- stdOutEventWithColor(&b)
	}
	return nil
}
