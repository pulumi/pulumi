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
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/util/logging"
	"github.com/pulumi/pulumi/pkg/util/result"
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
	plan            *Plan    // The plan currently being executed.
	opts            Options  // The options for this current plan.
	preview         bool     // Whether or not we are doing a preview.
	pendingNews     sync.Map // Resources that have been created but are pending a RegisterResourceOutputs.
	continueOnError bool     // True if we want to continue the plan after a step error.

	workers        sync.WaitGroup      // WaitGroup tracking the worker goroutines that are owned by this step executor.
	results        chan *result.Result // Channel for collecting Results from worker goroutines
	incomingChains chan Chain          // Incoming chains that we are to execute

	ctx      context.Context    // cancellation context for the current plan.
	cancel   context.CancelFunc // CancelFunc that cancels the above context.
	sawError atomic.Value       // atomic boolean indicating whether or not the step excecutor saw that there was an error.
}

//
// The stepExecutor communicates with a stepGenerator by listening to a channel. As the step generator
// generates new chains that need to be executed, the step executor will listen to this channel to execute
// those steps.
//

// Execute submits a Chain for asynchronous execution. The execution of the chain will begin as soon as there
// is a worker available to execute it.
func (se *stepExecutor) Execute(chain Chain) {
	// The select here is to avoid blocking on a send to se.incomingChains if a cancellation is pending.
	// If one is pending, we should exit early - we will shortly be tearing down the engine and exiting.
	select {
	case se.incomingChains <- chain:
	case <-se.ctx.Done():
	}
}

// ExecuteRegisterResourceOutputs services a RegisterResourceOutputsEvent synchronously on the calling goroutine.
func (se *stepExecutor) ExecuteRegisterResourceOutputs(e RegisterResourceOutputsEvent) {
	// Look up the final state in the pending registration list.
	urn := e.URN()
	value, has := se.pendingNews.Load(urn)
	contract.Assertf(has, "cannot complete a resource '%v' whose registration isn't pending", urn)
	reg := value.(Step)
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

			// This is a bit of a kludge, but ExecuteRegisterResourceOutputs is an odd duck
			// in that it doesn't execute on worker goroutines. Arguably, it should, but today it's
			// not possible to express RegisterResourceOutputs as a step. We could 1) more generally allow
			// clients of stepExecutor to do work on worker threads by e.g. scheduling arbitrary callbacks
			// or 2) promote RRE to be step-like so that it can be scheduled as if it were a step. Neither
			// of these are particularly appealing right now.
			outErr := errors.Wrap(eventerr, "resource complete event returned an error")
			diagMsg := diag.RawMessage(reg.URN(), outErr.Error())
			se.plan.Diag().Errorf(diagMsg)
			se.cancelDueToError()
			return
		}
	}
	e.Done()
}

// Errored returnes whether or not this step executor saw a step whose execution ended in failure.
func (se *stepExecutor) errored() bool {
	return se.sawError.Load().(bool)
}

// SignalCompletion signals to the stepExecutor that there are no more chains left to execute. All worker
// threads will terminate as soon as they retire all of the work they are currently executing.
func (se *stepExecutor) SignalCompletion() {
	close(se.incomingChains)
}

// WaitForCompletion blocks the calling goroutine until the step executor completes execution of all in-flight
// chains.
func (se *stepExecutor) WaitForCompletion() *result.Result {
	se.log(synchronousWorkerID, "StepExecutor.waitForCompletion(): waiting for worker threads to exit")
	se.workers.Wait()
	se.log(synchronousWorkerID, "StepExecutor.waitForCompletion(): worker threads all exited, gathering results")
	var results []*result.Result
	for i := 0; i < se.opts.DegreeOfParallelism(); i++ {
		results = append(results, <-se.results)
	}

	// Special case - if we're continuing on errors but an error occurred, return a non-nil Result.
	res := result.All(results)
	if res == nil && se.continueOnError && se.errored() {
		return result.Bail()
	}

	return res
}

//
// As calls to `Execute` submit chains for execution, some number of worker goroutines will continuously
// read from `incomingChains` and execute any chains that are received. The core execution logic is in
// the next few functions.
//

// executeChain executes a chain, one step at a time. If any step in the chain fails to execute, or if the
// context is canceled, the chain stops execution.
func (se *stepExecutor) executeChain(workerID int, chain Chain) *result.Result {
	for _, step := range chain {
		select {
		case <-se.ctx.Done():
			se.log(workerID, "step %v on %v canceled", step.Op(), step.URN())
			return nil
		default:
		}

		if res := se.executeStep(workerID, step); res != nil {
			return result.Wrapf(res, "when executing step '%s' on '%s'", step.Op(), step.URN())
		}
	}

	return nil
}

func (se *stepExecutor) cancelDueToError() {
	se.sawError.Store(true)
	if !se.continueOnError {
		se.cancel()
	}
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
func (se *stepExecutor) executeStep(workerID int, step Step) *result.Result {
	var payload interface{}
	events := se.opts.Events
	if events != nil {
		var err error
		payload, err = events.OnResourceStepPre(step)
		if err != nil {
			se.log(workerID, "step %v on %v failed pre-resource step: %v", step.Op(), step.URN(), err)
			err = errors.Wrap(err, "pre-step event returned an error")
			se.plan.Diag().Errorf(diag.RawMessage(step.URN(), err.Error()))
			return result.Bail()
		}
	}

	se.log(workerID, "applying step %v on %v (preview %v)", step.Op(), step.URN(), se.preview)
	status, stepComplete, err := step.Apply(se.preview)

	if err == nil {
		// If we have a state object, and this is a create or update, remember it, as we may need to update it later.
		if step.Logical() && step.New() != nil {
			if prior, has := se.pendingNews.Load(step.URN()); has {
				return result.Errorf(
					"resource '%s' registered twice (%s and %s)", step.URN(), prior.(Step).Op(), step.Op())
			}

			se.pendingNews.Store(step.URN(), step)
		}
	}

	if events != nil {
		if postErr := events.OnResourceStepPost(payload, step, status, err); postErr != nil {
			se.log(workerID, "step %v on %v failed post-resource step: %v", step.Op(), step.URN(), postErr)
			postErr = errors.Wrap(postErr, "pre-step event returned an error")
			se.plan.Diag().Errorf(diag.RawMessage(step.URN(), postErr.Error()))
			return result.Bail()
		}
	}

	// Calling stepComplete allows steps that depend on this step to continue. OnResourceStepPost saved the results
	// of the step in the snapshot, so we are ready to go.
	if stepComplete != nil {
		se.log(workerID, "step %v on %v retired", step.Op(), step.URN())
		stepComplete()
	}

	if err != nil {
		se.log(workerID, "step %v on %v failed with an error: %v", step.Op(), step.URN(), err)
		return result.Bail()
	}

	return nil
}

// log is a simple logging helper for the step executor.
func (se *stepExecutor) log(workerID int, msg string, args ...interface{}) {
	if logging.V(stepExecutorLogLevel) {
		message := fmt.Sprintf(msg, args...)
		logging.V(stepExecutorLogLevel).Infof("StepExecutor worker(%d): %s", workerID, message)
	}
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
	defer se.workers.Done()

	for {
		se.log(workerID, "worker waiting for incoming chains")
		select {
		case chain := <-se.incomingChains:
			if chain == nil {
				se.log(workerID, "worker received nil chain, exiting")
				se.results <- nil
				return
			}

			se.log(workerID, "worker received chain for execution")
			if res := se.executeChain(workerID, chain); res != nil {
				se.cancelDueToError()
				if !se.continueOnError {
					se.log(workerID, "chain failed, signalling cancellation")
					se.results <- res
					return
				}
			}
		case <-se.ctx.Done():
			se.log(workerID, "worker exiting due to cancellation")
			se.results <- nil
			return
		}
	}
}

func newStepExecutor(ctx context.Context, cancel context.CancelFunc, plan *Plan, opts Options,
	preview, continueOnError bool) *stepExecutor {
	fanout := opts.DegreeOfParallelism()
	exec := &stepExecutor{
		plan:            plan,
		opts:            opts,
		preview:         preview,
		continueOnError: continueOnError,
		incomingChains:  make(chan Chain),
		results:         make(chan *result.Result, fanout),
		ctx:             ctx,
		cancel:          cancel,
	}

	exec.sawError.Store(false)
	for i := 0; i < fanout; i++ {
		exec.workers.Add(1)
		go exec.worker(i)
	}

	return exec
}
