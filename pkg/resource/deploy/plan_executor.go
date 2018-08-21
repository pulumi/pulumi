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

package deploy

import (
	"context"

	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/util/logging"
)

const (
	// Utility constant for easy debugging.
	planExecutorLogLevel = 4
)

// PlanExecutor is responsible for taking a plan and driving it to completion.
// Its primary responsibility is to own a `stepGenerator` and `stepExecutor`, serving
// as the glue that links the two subsystems together.
type PlanExecutor struct {
	plan *Plan          // The plan that we are executing
	opts Options        // Options for the plan execution
	src  SourceIterator // The iterator that generates SourceEvents

	stepGen  *stepGenerator // step generator owned by this plan
	stepExec *stepExecutor  // step executor owned by this plan

	parentCtx context.Context    // cancellation context for the current CLI session.
	ctx       context.Context    // cancellation context for the current plan. Child of parentCtx.
	cancel    context.CancelFunc // CancelFunc that cancels the above context.
}

// Execute executes a plan to completion, using the given cancellation context and running a preview
// or update.
func (pe *PlanExecutor) Execute() error {
	// We iterate the source in its own goroutine because iteration is blocking and we want the main loop to be able to
	// respond to cancellation requests promptly.
	type nextEvent struct {
		Event SourceEvent
		Error error
	}
	incomingEvents := make(chan nextEvent)
	go func() {
		for {
			event, sourceErr := pe.src.Next()
			select {
			case incomingEvents <- nextEvent{event, sourceErr}:
				if event == nil {
					return
				}
			case <-pe.ctx.Done():
				logging.V(planExecutorLogLevel).Infof("PlanExecutor.Execute(...): incoming events goroutine exiting")
				return
			}
		}
	}()

	// The main loop. We'll continuously select for incoming events and the cancellation signal. There are
	// a three ways we can exit this loop:
	//  1. The SourceIterator sends us a `nil` event. This means that we're done processing source events and
	//     we should begin processing deletes.
	//  2. The SourceIterator sends us an error. This means some error occurred in the source program and we
	//     should bail.
	//  3. The stepExecCancel cancel context gets canceled. This means some error occurred in the step executor
	//     and we need to bail. This can also happen if the user hits Ctrl-C.
	err := func() error {
		for {
			logging.V(planExecutorLogLevel).Infof("PlanExecutor.Execute(...): waiting for incoming events")
			select {
			case event := <-incomingEvents:
				logging.V(planExecutorLogLevel).Infof("PlanExecutor.Execute(...): incoming event (nil? %v, %v)",
					event.Event == nil, event.Error)

				if event.Error != nil {
					logging.V(planExecutorLogLevel).Infof("PlanExecutor.Execute(...): saw incoming error: %v", event.Error)
					pe.cancel()
					pe.plan.Diag().Errorf(diag.RawMessage("" /*urn*/, event.Error.Error()))
					return event.Error
				}

				if event.Event == nil {
					// TODO[pulumi/pulumi#1625] Today we lack the ability to parallelize deletions. We have all the
					// information we need to do so (namely, a dependency graph). `GenerateDeletes` returns a single
					// chain of every delete that needs to be executed.
					deletes := pe.stepGen.GenerateDeletes()
					pe.stepExec.Execute(deletes)

					// Signal completion to the step executor. It'll exit once it's done retiring all of the steps in
					// the chain that we just gave it.
					pe.stepExec.SignalCompletion()
					logging.V(planExecutorLogLevel).Infof("PlanExecutor.Execute(...): issued deletes, exiting loop")

					return nil
				}

				if eventErr := pe.handleSingleEvent(event.Event); eventErr != nil {
					logging.V(planExecutorLogLevel).Infof("PlanExecutor.Execute(...): error handling event: %v",
						eventErr)
					pe.plan.Diag().Errorf(diag.RawMessage(pe.plan.generateEventURN(event.Event), eventErr.Error()))
					pe.cancel()
					return eventErr
				}
			case <-pe.ctx.Done():
				logging.V(planExecutorLogLevel).Infof("PlanExecutor.Execute(...): context finished: %v", pe.ctx.Err())
				return pe.ctx.Err()
			}
		}
	}()

	pe.stepExec.WaitForCompletion()
	logging.V(planExecutorLogLevel).Infof("PlanExecutor.Execute(...): step executor has completed")

	if err == nil && pe.stepExec.Errored() {
		err = errStepApplyFailed
	}

	return err
}

// Summary returns a PlanSummary of the plan that was executed.
func (pe *PlanExecutor) Summary() PlanSummary {
	return pe.stepGen
}

// handleSingleEvent handles a single source event. For all incoming events, it produces a chain that needs
// to be executed and schedules the chain for execution.
func (pe *PlanExecutor) handleSingleEvent(event SourceEvent) error {
	contract.Require(event != nil, "event != nil")

	var steps []Step
	var err error
	switch e := event.(type) {
	case RegisterResourceEvent:
		logging.V(planExecutorLogLevel).Infof("PlanExecutor.handleSingleEvent(...): received RegisterResourceEvent")
		steps, err = pe.stepGen.GenerateSteps(e)
	case ReadResourceEvent:
		logging.V(planExecutorLogLevel).Infof("PlanExecutor.handleSingleEvent(...): received ReadResourceEvent")
		steps, err = pe.stepGen.GenerateReadSteps(e)
	case RegisterResourceOutputsEvent:
		logging.V(planExecutorLogLevel).Infof("PlanExecutor.handleSingleEvent(...): received register resource outputs")
		pe.stepExec.ExecuteRegisterResourceOutputs(e)
		return nil
	}

	if err != nil {
		return err
	}
	pe.stepExec.Execute(steps)
	return nil
}

// NewPlanExecutor creates a new PlanExecutor suitable for executing the given plan.
func NewPlanExecutor(parentCtx context.Context, plan *Plan, opts Options,
	preview bool, src SourceIterator) *PlanExecutor {
	ctx, cancel := context.WithCancel(parentCtx)
	pe := &PlanExecutor{
		plan:      plan,
		opts:      opts,
		src:       src,
		stepGen:   newStepGenerator(plan, opts),
		stepExec:  newStepExecutor(ctx, cancel, plan, opts, preview),
		parentCtx: parentCtx,
		ctx:       ctx,
		cancel:    cancel,
	}
	return pe
}
