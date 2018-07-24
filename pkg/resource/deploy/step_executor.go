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
	"github.com/pulumi/pulumi/pkg/util/cancel"
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
	cancelCtx      *cancel.Context
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

func (se *stepExecutor) SignalCompletion() {
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

func (se *stepExecutor) worker(id int) {
	logging.V(stepExecutorLogLevel).Infof("StepExecutor worker(%d): initializing", id)
chain_dispatch:
	for {
		select {
		case <-se.abort:
			logging.V(stepExecutorLogLevel).Infof("StepExecutor worker(%d): exiting due to requested abort", id)
			break chain_dispatch

		case chain := <-se.incomingChains:
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
	executor := &stepExecutor{
		plan:           p,
		opts:           opts,
		incomingChains: make(chan Chain),
		preview:        preview,
	}

	executor.workers.Add(opts.Parallel)
	for i := 0; i < opts.Parallel; i++ {
		go executor.worker(i)
	}

	return executor
}
