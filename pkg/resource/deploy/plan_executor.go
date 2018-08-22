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

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/graph"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/util/logging"
)

// planExecutor is responsible for taking a plan and driving it to completion.
// Its primary responsibility is to own a `stepGenerator` and `stepExecutor`, serving
// as the glue that links the two subsystems together.
type planExecutor struct {
	plan *Plan // The plan that we are executing

	stepGen  *stepGenerator // step generator owned by this plan
	stepExec *stepExecutor  // step executor owned by this plan
}

// Utility for convenient logging.
var log = logging.V(4)

// execError creates an error appropriate for returning from planExecutor.Execute.
func execError(message string, preview bool) error {
	kind := "update"
	if preview {
		kind = "preview"
	}
	return errors.New(kind + " " + message)
}

// reportError reports a single error to the executor's diag stream with the indicated URN for context.
func (pe *planExecutor) reportError(urn resource.URN, err error) {
	pe.plan.Diag().Errorf(diag.RawMessage(urn, err.Error()))
}

// Execute executes a plan to completion, using the given cancellation context and running a preview
// or update.
func (pe *planExecutor) Execute(callerCtx context.Context, opts Options, preview bool) error {
	// Set up a goroutine that will signal cancellation to the plan's plugins if the caller context is cancelled. We do
	// not hang this off of the context we create below because we do not want the failure of a single step to cause
	// other steps to fail.
	done := make(chan bool)
	go func() {
		select {
		case <-callerCtx.Done():
			cancelErr := pe.plan.ctx.Host.SignalCancellation()
			if cancelErr != nil {
				log.Infof("planExecutor.Execute(...): failed to signal cancellation to providers: %v", cancelErr)
			}
		case <-done:
		}
	}()

	// Before doing anything else, optionally refresh each resource in the base checkpoint.
	if opts.Refresh {
		if err := pe.refresh(callerCtx, opts, preview); err != nil {
			return err
		}
		if opts.RefreshOnly {
			return nil
		}
	}

	// Begin iterating the source.
	src, err := pe.plan.source.Iterate(callerCtx, opts, pe.plan)
	if err != nil {
		return err
	}

	// Derive a cancellable context for this plan. We will only cancel this context if some piece of the plan's
	// execution fails.
	ctx, cancel := context.WithCancel(callerCtx)

	// Set up a step generator and executor for this plan.
	pe.stepGen = newStepGenerator(pe.plan, opts)
	pe.stepExec = newStepExecutor(ctx, cancel, pe.plan, opts, preview)

	// We iterate the source in its own goroutine because iteration is blocking and we want the main loop to be able to
	// respond to cancellation requests promptly.
	type nextEvent struct {
		Event SourceEvent
		Error error
	}
	incomingEvents := make(chan nextEvent)
	go func() {
		for {
			event, sourceErr := src.Next()
			select {
			case incomingEvents <- nextEvent{event, sourceErr}:
				if event == nil {
					return
				}
			case <-done:
				log.Infof("planExecutor.Execute(...): incoming events goroutine exiting")
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
	canceled, err := func() (bool, error) {
		log.Infof("planExecutor.Execute(...): waiting for incoming events")
		for {
			select {
			case event := <-incomingEvents:
				log.Infof("planExecutor.Execute(...): incoming event (nil? %v, %v)", event.Event == nil, event.Error)

				if event.Error != nil {
					pe.reportError("", event.Error)
					cancel()
					return false, event.Error
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
					log.Infof("planExecutor.Execute(...): issued deletes")

					return false, nil
				}

				if eventErr := pe.handleSingleEvent(event.Event); eventErr != nil {
					log.Infof("planExecutor.Execute(...): error handling event: %v", eventErr)
					pe.reportError(pe.plan.generateEventURN(event.Event), eventErr)
					cancel()
					return false, eventErr
				}
			case <-ctx.Done():
				log.Infof("planExecutor.Execute(...): context finished: %v", ctx.Err())

				// NOTE: we use the presence of an error in the caller context in order to distinguish caller-initiated
				// cancellation from internally-initiated cancellation.
				return callerCtx.Err() != nil, nil
			}
		}
	}()
	close(done)

	pe.stepExec.WaitForCompletion()
	log.Infof("planExecutor.Execute(...): step executor has completed")

	// Figure out if execution failed and why. Step generation and execution errors trump cancellation.
	if err != nil || pe.stepExec.Errored() {
		err = execError("failed", preview)
	} else if canceled {
		err = execError("canceled", preview)
	}
	return err
}

// handleSingleEvent handles a single source event. For all incoming events, it produces a chain that needs
// to be executed and schedules the chain for execution.
func (pe *planExecutor) handleSingleEvent(event SourceEvent) error {
	contract.Require(event != nil, "event != nil")

	var steps []Step
	var err error
	switch e := event.(type) {
	case RegisterResourceEvent:
		log.Infof("planExecutor.handleSingleEvent(...): received RegisterResourceEvent")
		steps, err = pe.stepGen.GenerateSteps(e)
	case ReadResourceEvent:
		log.Infof("planExecutor.handleSingleEvent(...): received ReadResourceEvent")
		steps, err = pe.stepGen.GenerateReadSteps(e)
	case RegisterResourceOutputsEvent:
		log.Infof("planExecutor.handleSingleEvent(...): received register resource outputs")
		pe.stepExec.ExecuteRegisterResourceOutputs(e)
		return nil
	}

	if err != nil {
		return err
	}
	pe.stepExec.Execute(steps)
	return nil
}

// refresh refreshes the state of the base checkpoint file for the current plan in memory.
func (pe *planExecutor) refresh(callerCtx context.Context, opts Options, preview bool) error {
	prev := pe.plan.prev
	if prev == nil || len(prev.Resources) == 0 {
		return nil
	}

	ctx, cancel := context.WithCancel(callerCtx)
	stepExec := newStepExecutor(ctx, cancel, pe.plan, opts, preview)

	canceled := false
	steps := make([]Step, len(prev.Resources))
	for i := range prev.Resources {
		if ctx.Err() != nil {
			// NOTE: we use the presence of an error in the caller context in order to distinguish caller-initiated
			// cancellation from internally-initiated cancellation.
			canceled = callerCtx.Err() != nil
			break
		}

		steps[i] = NewRefreshStep(pe.plan, prev.Resources[i], nil)
		stepExec.Execute([]Step{steps[i]})
	}

	stepExec.SignalCompletion()
	stepExec.WaitForCompletion()

	if stepExec.Errored() {
		return execError("failed", preview)
	} else if canceled {
		return execError("canceled", preview)
	}

	// Rebuild this plan's map of old resources and dependency graph, stripping out any deleted resources and repairing
	// dependency lists as necessary.
	resources := make([]*resource.State, 0, len(prev.Resources))
	deleted := make(map[resource.URN]struct{})
	olds := make(map[resource.URN]*resource.State)
	for _, s := range steps {
		new := s.New()
		if new == nil {
			// Note that this URN no longer exists.
			deleted[s.URN()] = struct{}{}
			continue
		}
		// If this URN referred to a deleted resource, note that it exists again.
		delete(deleted, s.URN())

		// Remove any deleted resource from this resource's dependency list.
		if len(new.Dependencies) != 0 {
			deps := make([]resource.URN, 0, len(new.Dependencies))
			for _, d := range new.Dependencies {
				if _, gone := deleted[d]; !gone {
					deps = append(deps, d)
				}
			}
			new.Dependencies = deps
		}

		resources = append(resources, new)

		// Do not record resources that are pending deletion in the lookup table.
		if !new.Delete {
			olds[new.URN] = new
		}
	}
	pe.plan.prev.Resources = resources
	pe.plan.olds, pe.plan.depGraph = olds, graph.NewDependencyGraph(resources)
	return nil
}
