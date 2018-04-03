// Copyright 2017-2018, Pulumi Corporation.
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
	"github.com/pkg/errors"
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

	// Always set opts.DryRun to `true` when processing previews: if we do not do this, the engine will assume that it
	// should elide unknown input/output properties when interacting with the language and resource providers and we
	// will produce unexpected results.
	opts.DryRun = true

	emitter := makeEventEmitter(events, u)
	return preview(ctx, planOptions{
		UpdateOptions: opts,
		SourceFunc:    newUpdateSource,
		Events:        emitter,
		Diag:          newEventSink(emitter),
	})
}

func preview(ctx *planContext, opts planOptions) error {
	result, err := plan(ctx, opts)
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

	acts.Opts.Events.resourceOutputsEvent(step, true /*planning*/, acts.Opts.Debug)

	return nil
}
