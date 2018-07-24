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
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/util/logging"
)

const (
	stepExecutorLogLevel = 1
)

var (
	ErrStepExecutionFailed = errors.New("oh no it failed")
)

// A Chain is a sequence of Steps which are required to execute in order and
// not in parallel.
type Chain = []Step

type stepExecutor struct {
	pendingNews    map[resource.URN]Step
	abort          chan struct{}
	done           chan struct{}
	plan           *Plan
	opts           Options
	incomingChains chan Chain
	preview        bool
	workers        sync.WaitGroup
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
	reg, has := se.pendingNews[urn]
	contract.Assertf(has, "cannot complete a resource '%v' whose registration isn't pending", urn)
	contract.Assertf(reg != nil, "expected a non-nil resource step ('%v')", urn)
	delete(se.pendingNews, urn)

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
	select {
	case <-se.abort:
		return ErrStepExecutionFailed
	case <-se.done:
		return nil
	}
}

func (se *stepExecutor) Close() {
	se.workers.Wait()
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
	events := se.opts.Events
	if events != nil {
		var eventerr error
		logging.V(7).Infof("StepExecutor worker(%d)")
		preCtx, eventerr = events.OnResourceStepPre(step)
		if eventerr != nil {
			panic(eventerr) // TODO
		}
	}

	urn := step.URN()
	logging.V(stepExecutorLogLevel).Infof(
		"StepExecutor worker(%d): Applying step %v on %v (preview %v)", workerID, step.Op(), urn, se.preview)
	status, err := step.Apply(se.preview)

	// If there is no error, proceed to save the state; otherwise, go straight to the exit codepath.
	if err == nil {
		// If we have a state object, and this is a create or update, remember it, as we may need to update it later.
		if step.Logical() && step.New() != nil {
			if _, has := se.pendingNews[urn]; has {
				panic(has) // TODO
			}

			se.pendingNews[urn] = step
		}
	}

	if events != nil {
		if eventerr := events.OnResourceStepPost(preCtx, step, status, err); eventerr != nil {
			panic(eventerr) // TODO
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
		pendingNews:    make(map[resource.URN]Step),
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

	executor.workers.Add(parallelFactor)
	for i := 0; i < parallelFactor; i++ {
		go executor.worker(i)
	}

	return executor
}
