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
	"sync"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/util/logging"
)

const (
	stepExecutorLogLevel = 1
)

var (
	ErrStepExecutionFailed = errors.New("deployment failed")
)

// A Chain is a sequence of Steps which are required to execute in order and
// not in parallel.
type Chain = []Step

type stepExecutor struct {
	pendingNews    sync.Map
	abort          chan struct{}
	done           chan struct{}
	plan           *Plan
	opts           Options
	incomingChains chan Chain
	preview        bool
	workers        sync.WaitGroup
}

func (se *stepExecutor) Abort() {
	logging.V(stepExecutorLogLevel).Infof("StepExecutor: signalling external abort")
	close(se.abort)
}

func (se *stepExecutor) Aborted() bool {
	select {
	case <-se.abort:
		return true
	default:
		return false
	}
}

func (se *stepExecutor) Execute(chain Chain) {
	se.incomingChains <- chain
}

func (se *stepExecutor) ExecuteSync(chain Chain) {
	se.executeChain(-1, chain)
}

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
	logging.V(7).Infof("Registered resource outputs %s: old=#%d, new=#%d", urn, len(reg.New().Outputs), len(outs))
	reg.New().Outputs = e.Outputs()

	// If there is an event subscription for finishing the resource, execute them.
	if e := se.opts.Events; e != nil {
		if eventerr := e.OnResourceOutputs(reg); eventerr != nil {
			panic(eventerr) // TODO
		}
	}

	e.Done()
}

func (se *stepExecutor) SignalCompletion() {
	close(se.incomingChains)
	close(se.done)
}

func (se *stepExecutor) Wait() error {
	logging.V(stepExecutorLogLevel).Infoln("StepExecutor: waiting for completion")
	select {
	case <-se.abort:
		logging.V(stepExecutorLogLevel).Infoln("StepExecutor: waiting complete: aborted")
	case <-se.done:
		logging.V(stepExecutorLogLevel).Infoln("StepExecutor: waiting complete: successful")
	}

	se.Close()
	if se.Aborted() {
		return ErrStepExecutionFailed
	}

	return nil
}

func (se *stepExecutor) Close() {
	logging.V(stepExecutorLogLevel).Infof("StepExecutor: waiting for termination of workers")
	se.workers.Wait()
	logging.V(stepExecutorLogLevel).Infof("StepExecutor: workers have all terminated")
}

func (se *stepExecutor) worker(id int) {
	logging.V(stepExecutorLogLevel).Infof("StepExecutor worker(%d): initializing", id)
chain_dispatch:
	for {
		select {
		case <-se.abort:
			logging.V(stepExecutorLogLevel).Infof("StepExecutor worker(%d): exiting due to requested abort", id)
			break chain_dispatch

		case chain := <-se.incomingChains:
			if chain == nil {
				logging.V(stepExecutorLogLevel).Infof("StepExecutor worker(%d): received nil chain, exiting", id)
				break chain_dispatch
			}

			logging.V(stepExecutorLogLevel).Infof("StepExecutor worker(%d): executing chain of size %d", id, len(chain))
			se.executeChain(id, chain)
		}
	}

	logging.V(stepExecutorLogLevel).Infof("StepExecutor worker(%d): exiting", id)
	se.workers.Done()
}

func (se *stepExecutor) executeChain(workerID int, chain Chain) {
	for _, step := range chain {
		if !se.executeStep(workerID, step) {
			break
		}
	}
}

func (se *stepExecutor) executeStep(workerID int, step Step) bool {
	var preCtx interface{}
	urn := step.URN()
	events := se.opts.Events
	if events != nil {
		var eventerr error
		preCtx, eventerr = events.OnResourceStepPre(step)
		if eventerr != nil {
			logging.V(stepExecutorLogLevel).Infof(
				"StepExecutor worker(%d): Step %v on %v failed pre-resource step", workerID, step.Op(), urn)
			close(se.abort)
			return false
		}
	}

	logging.V(stepExecutorLogLevel).Infof(
		"StepExecutor worker(%d): Applying step %v on %v (preview %v)", workerID, step.Op(), urn, se.preview)
	status, err := step.Apply(se.preview)

	// If there is no error, proceed to save the state; otherwise, go straight to the exit codepath.
	if err == nil {
		// If we have a state object, and this is a create or update, remember it, as we may need to update it later.
		if step.Logical() && step.New() != nil {
			if _, has := se.pendingNews.Load(urn); has {
				panic(has) // TODO
			}

			se.pendingNews.Store(urn, step)
		}
	}

	if events != nil {
		if eventerr := events.OnResourceStepPost(preCtx, step, status, err); eventerr != nil {
			logging.V(stepExecutorLogLevel).Infof(
				"StepExecutor worker(%d): Step %v on %v failed post-resource step", workerID, step.Op(), urn)
			close(se.abort)
			return false
		}
	}

	if err != nil {
		logging.V(stepExecutorLogLevel).Infof(
			"StepExecutor worker(%d): Step %v on %v failed with an error, signalling abort", workerID, step.Op(), urn)

		close(se.abort)
		return false
	}

	return true
}

func newStepExecutor(p *Plan, opts Options, preview bool) *stepExecutor {
	logging.V(1).Infoln("initializing plan executor")
	executor := &stepExecutor{
		abort:          make(chan struct{}),
		done:           make(chan struct{}),
		plan:           p,
		opts:           opts,
		incomingChains: make(chan Chain),
		preview:        preview,
	}

	parallelFactor := opts.Parallel
	if parallelFactor < 1 {
		parallelFactor = 1
	}

	logging.V(stepExecutorLogLevel).Infof("StepExecutor: launching %d workers", parallelFactor)
	executor.workers.Add(parallelFactor)
	for i := 0; i < parallelFactor; i++ {
		go executor.worker(i)
	}

	return executor
}
