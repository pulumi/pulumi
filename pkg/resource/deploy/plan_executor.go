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
	"sync/atomic"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/util/logging"
)

const (
	// Utility constant for easy debugging.
	planExecutorLogLevel = 4
)

var (
	// ErrPreviewFailed is returned whenever a preview fails.
	ErrPreviewFailed = errors.New("preview failed")

	// ErrUpdateFailed is returned whenever an update fails.
	ErrUpdateFailed = errors.New("update failed")

	// ErrCanceled is returned whenever a plan is canceled.
	ErrCanceled = errors.New("plan canceled")
)

// PlanExecutor is responsible for taking a plan and driving it to completion.
// Its primary responsibility is to own a `stepGenerator` and `stepExecutor`, serving
// as the glue that links the two subsystems together.
type PlanExecutor struct {
	plan    *Plan          // The plan that we are executing
	opts    Options        // Options for the plan execution
	src     SourceIterator // The iterator that generates SourceEvents
	preview bool           // are we running a preview?

	stepGen  *stepGenerator // step generator owned by this plan
	stepExec *stepExecutor  // step executor owned by this plan

	ctx    context.Context    // cancellation context for the current plan.
	cancel context.CancelFunc // CancelFunc that cancels the above context.

	sawError  atomic.Value // have we seen an error?
	sawCancel atomic.Value // have we seen a cancel?
}

// Execute executes a plan to completion, using the given cancellation context and running a preview
// or update.
func (pe *PlanExecutor) Execute() error {
	// Before heading into the main event loop, we launch two goroutines. The first one links
	// the parent cancellation context (which signals cancellation from the CLI level, i.e. Ctrl+C)
	// to the cancellation context that the plan executor shares with its step executor. This ensures
	// that top-level cancellations result in quick teardown of all worker threads and the plan executor
	// itself.
	go func() {
		<-pe.ctx.Done()
		logging.V(planExecutorLogLevel).Infof("PlanExecutor.Execute(...): received cancel signal")
		pe.sawCancel.Store(true)
	}()

	// The second one polls for incoming source events and writes them to the `incomingEvents` channel,
	// so the man loop can `select` on it.
	type nextEvent struct {
		Event SourceEvent
		Error error
	}

	incomingEvents := make(chan nextEvent)
	go func() {
		for {
			event, sourceErr := pe.src.Next()
			incomingEvents <- nextEvent{event, sourceErr}
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
outer:
	for {
		logging.V(planExecutorLogLevel).Infof("PlanExecutor.Execute(...): waiting for incoming events")
		select {
		case event := <-incomingEvents:
			logging.V(planExecutorLogLevel).Infof("PlanExecutor.Execute(...): incoming event")
			if event.Error != nil {
				logging.V(planExecutorLogLevel).Infof("PlanExecutor.Execute(...): saw incoming error: %v", event.Error)
				pe.cancelDueToError()
				break outer
			}

			if event.Event == nil {
				logging.V(planExecutorLogLevel).Infof("PlanExecutor.Execute(...): saw nil event, beginning termination")

				// TODO[pulumi/pulumi#1625] Today we lack the ability to parallelize deletions. We have all the
				// information we need to do so (namely, a dependency graph). `GenerateDeletes` returns a single
				// chain of every delete that needs to be executed.
				deletes := pe.stepGen.GenerateDeletes()
				pe.stepExec.Execute(deletes)

				// Signal completion to the step executor. It'll exit once it's done retiring all of the steps in
				// the chain that we just gave it.
				pe.stepExec.SignalCompletion()
				logging.V(planExecutorLogLevel).Infof("PlanExecutor.Execute(...): completed deletes, exiting loop")
				break outer
			}

			pe.handleSingleEvent(event.Event)
		case <-pe.ctx.Done():
			logging.V(planExecutorLogLevel).Infof("PlanExecutor.Execute(...): context canceled")
			pe.cancelDueToError()
			break outer
		}
	}

	logging.V(planExecutorLogLevel).Infof("PlanExecutor.Execute(...): exited event loop, waiting for completion")
	pe.stepExec.WaitForCompletion()
	logging.V(planExecutorLogLevel).Infof("PlanExecutor.Execute(...): step executor has completed")

	// To provide the best error message we can, we've kept track of whether or not we were successful and, if we
	// were not, if we failed because of a cancel or because the step executor died.
	if pe.canceled() {
		logging.V(planExecutorLogLevel).Infof("PlanExecutor.Execute(...): observed that the plan was canceled")
		return ErrCanceled
	}

	if pe.errored() {
		logging.V(planExecutorLogLevel).Infof("PlanExecutor.Execute(...): observed that the plan errored")
		if pe.preview {
			return ErrPreviewFailed
		}

		return ErrUpdateFailed
	}

	logging.V(planExecutorLogLevel).Infof("PlanExecutor.Execute(...): observed that the plan was successful")
	return nil
}

// Summary returns a PlanSummary of the plan that was executed.
func (pe *PlanExecutor) Summary() PlanSummary {
	return pe.stepGen
}

// errored returns whether or not this plan failed due to an error in step application.
func (pe *PlanExecutor) errored() bool {
	return pe.sawError.Load().(bool) || pe.stepExec.Errored()
}

// canceled returns whether or not this plan failed because it was canceled through Ctrl-C.
func (pe *PlanExecutor) canceled() bool {
	return pe.sawCancel.Load().(bool)
}

// cancelDueToError cancels the step executor and signals shutdown because the plan executor witnessed
// an error that the step executor would not have witnessed. The main reason this happens is because of errors
// occurring in the source program that can't be translated into chains for the step executor to execute.
func (pe *PlanExecutor) cancelDueToError() {
	pe.sawError.Store(true)
	pe.cancel()
}

// handleSingleEvent handles a single source event. For all incoming events, it produces a chain that needs
// to be executed and schedules the chain for execution.
func (pe *PlanExecutor) handleSingleEvent(event SourceEvent) {
	if event == nil {
		logging.V(planExecutorLogLevel).Infof("PlanExecutor.handleSingleEvent(...): received nil event")
		return
	}

	logging.V(planExecutorLogLevel).Infof("PlanExecutor.handleSingleEvent(...): received event")
	switch e := event.(type) {
	case RegisterResourceEvent:
		step, steperr := pe.stepGen.GenerateSteps(e)
		if steperr != nil {
			logging.V(planExecutorLogLevel).Infof(
				"PlanExecutor.handleSingleEvent(...): received step event error: %v", steperr.Error())
			pe.cancel()
			return
		}

		logging.V(planExecutorLogLevel).Infof("PlanExecutor.handleSingleEvent(...): submitting chain for execution")
		pe.stepExec.Execute(step)
	case ReadResourceEvent:
		step, steperr := pe.stepGen.GenerateReadSteps(e)
		if steperr != nil {
			logging.V(planExecutorLogLevel).Infof(
				"PlanExecutor.handleSingleEvent(...): received step event error: %v", steperr.Error())
			pe.cancel()
			return
		}

		logging.V(planExecutorLogLevel).Infof("PlanExecutor.handleSingleEvent(...): submitting reads for execution")
		pe.stepExec.Execute(step)
	case RegisterResourceOutputsEvent:
		logging.V(planExecutorLogLevel).Infof("PlanExecutor.handleSingleEvent(...): received register resource outputs")
		pe.stepExec.ExecuteRegisterResourceOutputs(e)
	}
}

// NewPlanExecutor creates a new PlanExecutor suitable for executing the given plan.
func NewPlanExecutor(parentCtx context.Context, plan *Plan, opts Options,
	preview bool, src SourceIterator) *PlanExecutor {
	ctx, cancel := context.WithCancel(parentCtx)
	pe := &PlanExecutor{
		plan:     plan,
		opts:     opts,
		src:      src,
		preview:  preview,
		stepGen:  newStepGenerator(plan, opts),
		stepExec: newStepExecutor(ctx, cancel, plan, opts, preview),
		ctx:      ctx,
		cancel:   cancel,
	}

	pe.sawError.Store(false)
	pe.sawCancel.Store(false)
	return pe
}
