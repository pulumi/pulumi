// Copyright 2018, Pulumi Corporation.  All rights reserved.

package engine

import (
	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

func Preview(u UpdateInfo, events chan<- Event, opts UpdateOptions) error {
	contract.Require(u != nil, "u")
	contract.Require(events != nil, "events")

	defer func() { events <- cancelEvent() }()

	ctx, err := newPlanContext(u)
	if err != nil {
		return err
	}
	defer ctx.Close()

	emitter := makeEventEmitter(events, u)
	return preview(ctx, planOptions{
		UpdateOptions: opts,
		SourceFunc:    newUpdateSource,
		Events:        emitter,
		Diag:          newEventSink(emitter),
	})
}

func preview(ctx *planContext, opts planOptions) error {
	result, err := plan(ctx, opts, true /*dryRun*/)
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

		if _, err := printPlan(result, true /*dryRun*/); err != nil {
			return err
		}
	}

	return nil
}

type previewActions struct {
	Ops  map[deploy.StepOp]int
	Opts planOptions
	Seen map[resource.URN]deploy.Step
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

	acts.Opts.Events.resourceOutputsEvent(step, true /*planning*/, acts.Opts.Debug)

	return nil
}
