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
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/util/cancel"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/util/logging"
)

const (
	// Dummy workerID for synchronous operations.
	synchronousWorkerID = -1

	// Utility constant for easy debugging.
	stepExecutorLogLevel = 4
)

// A Chain is a sequence of Steps that must be executed in the given order.
type Chain = []Step

// stepExecutor is the component of the engine responsible for taking steps and executing
// them, possibly in parallel if requested. The step generator operates on the granularity
// of "chains", which are sequences of steps that must be executed exactly in the given order.
// Chains are a simplification of the full dependency graph DAG within Pulumi programs. Since
// Pulumi language hosts can only invoke the resource monitor once all of their dependencies have
// resolved, we (the engine) can assume that any chain given to us by the step generator is already
// ready to execute.
type stepExecutor struct {
	plan        *Plan    // The plan currently being executed.
	opts        Options  // The options for this current plan.
	preview     bool     // Whether or not we are doing a preview.
	pendingNews sync.Map // Resources that have been created but are pending a RegisterResourceOutputs.

	workers        sync.WaitGroup // WaitGroup tracking the worker goroutines that are owned by this step executor.
	incomingChains chan Chain     // Incoming chains that we are to execute

	cancel *cancel.Source // Cancellation source for this step executor.

	sawError uint32 // atomic boolean indicating whether or not the step excecutor saw that there was an error.
}

//
// The stepExecutor communicates with a stepGenerator by listening to a channel. As the step generator
// generates new chains that need to be executed, the step executor will listen to this channel to execute
// those steps.
//

// Execute submits a Chain for asynchronous execution. The execution of the chain will begin as soon as there
// is a worker available to execute it.
func (se *stepExecutor) Execute(chain Chain) {
	select {
	case se.incomingChains <- chain:
	case <-se.cancel.Context().Canceled():
	}
}

// ExecuteRegisterResourceOutputs services a RegisterResourceOutputsEvent synchronously on the calling goroutine.
func (se *stepExecutor) ExecuteRegisterResourceOutputs(e RegisterResourceOutputsEvent) {
	// Look up the final state in the pending registration list.
	urn := e.URN()
	value, has := se.pendingNews.Load(urn)
	reg := value.(Step)
	contract.Assertf(has, "cannot complete a resource '%v' whose registration isn't pending", urn)
	contract.Assertf(reg != nil, "expected a non-nil resource step ('%v')", urn)
	se.pendingNews.Delete(urn)
	// Unconditionally set the resource's outputs to what was provided.  This intentionally overwrites whatever
	// might already be there, since otherwise "deleting" outputs would have no affect.
	outs := e.Outputs()
	se.log(synchronousWorkerID,
		"registered resource outputs %s: old=#%d, new=#%d", urn, len(reg.New().Outputs), len(outs))
	reg.New().Outputs = e.Outputs()
	// If there is an event subscription for finishing the resource, execute them.
	if e := se.opts.Events; e != nil {
		if eventerr := e.OnResourceOutputs(reg); eventerr != nil {
			se.log(synchronousWorkerID, "register resource outputs failed: %s", eventerr.Error())
			se.writeError(reg.URN(), "resource complete event returned an error: %s", eventerr.Error())
			se.cancelDueToError()
			return
		}
	}
	e.Done()
}

// Errored returnes whether or not this step executor saw a step whose execution ended in failure.
func (se *stepExecutor) Errored() bool {
	return atomic.LoadUint32(&se.sawError) == 1
}

// SignalCompletion signals to the stepExecutor that there are no more chains left to execute. All worker
// threads will terminate as soon as they retire all of the work they are currently executing.
func (se *stepExecutor) SignalCompletion() {
	close(se.incomingChains)
}

// WaitForCompletion blocks the calling goroutine until the step executor completes execution of all in-flight
// chains.
func (se *stepExecutor) WaitForCompletion() {
	se.log(synchronousWorkerID, "StepExecutor.waitForCompletion(): waiting for worker threads to exit")
	se.workers.Wait()
	se.log(synchronousWorkerID, "StepExecutor.waitForCompletion(): worker threads all exited")
}

//
// As calls to `Execute` submit chains for execution, some number of worker goroutines will continuously
// read from `incomingChains` and execute any chains that are received. The core execution logic is in
// the next few functions.
//

// executeChain executes a chain, one step at a time. If any step in the chain fails to execute, or if the
// context is canceled, the chain stops execution.
func (se *stepExecutor) executeChain(workerID int, chain Chain) {
	for _, step := range chain {
		select {
		case <-se.cancel.Context().Canceled():
			se.log(workerID, "step %v on %v canceled", step.Op(), step.URN())
			return
		default:
		}

		if !se.executeStep(workerID, step) {
			se.log(workerID, "step %v on %v failed, signalling cancellation", step.Op(), step.URN())
			se.cancelDueToError()
			return
		}
	}
}

func (se *stepExecutor) cancelDueToError() {
	atomic.SwapUint32(&se.sawError, 1)
	se.cancel.Cancel()
}

//
// The next few functions are responsible for executing individual steps. The basic flow of step
// execution is
//   1. The pre-step event is raised, if there are any attached callbacks to the engine
//   2. If successful, the step is executed (if not a preview)
//   3. The post-step event is raised, if there are any attached callbacks to the engine
//
// The pre-step event returns an interface{}, which is some arbitrary context that must be passed
// verbatim to the post-step event.
//

// executeStep executes a single step, returning true if the step execution was successful and
// false if it was not.
func (se *stepExecutor) executeStep(workerID int, step Step) bool {
	payload, ok := se.executePreEvents(workerID, step)
	if !ok {
		return false
	}

	se.log(workerID, "applying step %v on %v (preview %v)", step.Op(), step.URN(), se.preview)
	status, err := step.Apply(se.preview)
	if ok := se.executePostEvents(workerID, payload, step, status, err); !ok {
		return false
	}

	if err != nil {
		se.log(workerID, "step %v on %v failed with an error: %v", step.Op(), step.URN(), err)
		se.cancelDueToError()
		return false
	}

	return true
}

func (se *stepExecutor) executePreEvents(workerID int, step Step) (interface{}, bool) {
	var payload interface{}
	events := se.opts.Events
	if events != nil {
		var err error
		payload, err = events.OnResourceStepPre(step)
		if err != nil {
			se.log(workerID, "step %v on %v failed pre-resource step: %v", step.Op(), step.URN(), err)
			se.writeError(step.URN(), "pre-step event returned an error: %s", err.Error())
			return nil, false
		}
	}

	return payload, true
}

func (se *stepExecutor) executePostEvents(workerID int, payload interface{},
	step Step, status resource.Status, err error) bool {
	if err == nil {
		// If we have a state object, and this is a create or update, remember it, as we may need to update it later.
		if step.Logical() && step.New() != nil {
			if prior, has := se.pendingNews.Load(step.URN()); has {
				se.writeError(step.URN(), "resource '%s' registered twice (%s and %s)", prior.(Step).Op(), step.Op())
				return false
			}

			se.pendingNews.Store(step.URN(), step)
		}
	}

	events := se.opts.Events
	if events != nil {
		if postErr := events.OnResourceStepPost(payload, step, status, err); postErr != nil {
			se.log(workerID, "step %v on %v failed post-resource step: %v", step.Op(), step.URN(), postErr)
			se.writeError(step.URN(), "post-step event returned an error: %s", postErr.Error())
			return false
		}
	}

	return true
}

// log is a simple logging helper for the step executor.
func (se *stepExecutor) log(workerID int, msg string, args ...interface{}) {
	if logging.V(stepExecutorLogLevel) {
		message := fmt.Sprintf(msg, args...)
		logging.V(stepExecutorLogLevel).Infof("StepExecutor worker(%d): %s", workerID, message)
	}
}

// writeError writes an error to the standard diagnostic mechanism, with the given URN and message.
// The message is anchored to the resource with the given URN's row in the CLI output.
func (se *stepExecutor) writeError(urn resource.URN, msg string, args ...interface{}) {
	diagMsg := diag.RawMessage(urn, fmt.Sprintf(msg, args...))
	se.plan.Diag().Errorf(diagMsg)
}

//
// The step executor owns a number of goroutines that it considers to be "workers", responsible for
// executing steps. By default, as we ease into the waters of parallelism, there is at most one worker
// active.
//
// Workers continuously pull from se.incomingChains, executing chains as they are provided to the executor.
// There are two reasons why a worker would exit:
//
//  1. A worker exits if se.ctx is canceled. There are two ways that se.ctx gets canceled: first, if there is
//     a step error in another worker, it will cancel the context. Second, if the plan executor experiences an
//     error when generating steps or doing pre or post-step events, it will cancel the context.
//  2. A worker exits if it experiences an error when running a step.
//

// worker is the base function for all step executor worker goroutines. It continuously polls for new chains
// and executes any that it gets from the channel.
func (se *stepExecutor) worker(workerID int) {
	se.log(workerID, "worker coming online")
	se.workers.Add(1)
	defer se.workers.Done()

outer:
	for {
		se.log(workerID, "worker waiting for incoming chains")
		select {
		case chain := <-se.incomingChains:
			if chain == nil {
				se.log(workerID, "worker received nil chain, exiting")
				break outer
			}

			se.log(workerID, "worker received chain for execution")
			se.executeChain(workerID, chain)
		case <-se.cancel.Context().Canceled():
			se.log(workerID, "worker exiting due to cancellation")
			break outer
		}
	}

	se.log(workerID, "worker terminating")
}

func newStepExecutor(cancel *cancel.Source, plan *Plan, opts Options, preview bool) *stepExecutor {
	exec := &stepExecutor{
		plan:           plan,
		opts:           opts,
		preview:        preview,
		incomingChains: make(chan Chain),
		cancel:         cancel,
		sawError:       0,
	}

	fanout := opts.Parallel
	if fanout < 1 {
		fanout = 1
	}

	for i := 0; i < fanout; i++ {
		go exec.worker(i)
	}

	return exec
}
