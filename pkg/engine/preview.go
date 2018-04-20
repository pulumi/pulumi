// Copyright 2018, Pulumi Corporation.  All rights reserved.

package engine

import (
	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

func Preview(u UpdateInfo, ctx *Context, opts UpdateOptions) error {
	contract.Require(u != nil, "u")
	contract.Require(ctx != nil, "ctx")

	defer func() { ctx.Events <- cancelEvent() }()

	info, err := newPlanContext(u)
	if err != nil {
		return err
	}
	defer info.Close()

	emitter := makeEventEmitter(ctx.Events, u)
	return preview(ctx, info, planOptions{
		UpdateOptions: opts,
		SourceFunc:    newUpdateSource,
		Events:        emitter,
		Diag:          newEventSink(emitter),
	})
}

func preview(ctx *Context, info *planContext, opts planOptions) error {
	result, err := plan(info, opts, true /*dryRun*/)
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

		if _, err := printPlan(ctx, result, true /*dryRun*/); err != nil {
			return err
		}
	}

	return nil
}

type previewActions struct {
	Refresh bool
	Ops     map[deploy.StepOp]int
	Opts    planOptions
	Seen    map[resource.URN]deploy.Step
}

func newPreviewActions(opts planOptions) *previewActions {
	return &previewActions{
		Ops:  make(map[deploy.StepOp]int),
		Opts: opts,
		Seen: make(map[resource.URN]deploy.Step),
	}
}

func (acts *previewActions) OnResourceStepPre(step deploy.Step) (interface{}, error) {
	acts.Seen[step.URN()] = step
	acts.Opts.Events.resourcePreEvent(step, true /*planning*/, acts.Opts.Debug)
	return nil, nil
}

func (acts *previewActions) OnResourceStepPost(ctx interface{},
	step deploy.Step, status resource.Status, err error) error {
	assertSeen(acts.Seen, step)

	if err != nil {
		acts.Opts.Diag.Errorf(diag.GetPreviewFailedError(step.URN()), err)
	} else {
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

	// Print the resource outputs separately, unless this is a refresh in which case they are already printed.
	if !acts.Opts.SkipOutputs {
		acts.Opts.Events.resourceOutputsEvent(step, true /*planning*/, acts.Opts.Debug)
	}

	return nil
}
