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
	"strings"
	"sync"
	"sync/atomic"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/deploy/providers"
	"github.com/pulumi/pulumi/pkg/resource/graph"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/util/logging"
	"github.com/pulumi/pulumi/pkg/util/result"
)

// planExecutor is responsible for taking a plan and driving it to completion.
// Its primary responsibility is to own a `stepGenerator` and `stepExecutor`, serving
// as the glue that links the two subsystems together.
type planExecutor struct {
	plan *Plan // The plan that we are executing

	stepGen  *stepGenerator // step generator owned by this plan
	stepExec *stepExecutor  // step executor owned by this plan
}

// A set is returned of all the target URNs to facilitate later callers.  The set can be 'nil'
// indicating no targets, or will be non-nil and non-empty if there are targets.  Only URNs in the
// original array are in the set.  i.e. it's only checked for containment.  The value of the map is
// unused.
func createTargetMap(targets []resource.URN) map[resource.URN]bool {
	if len(targets) == 0 {
		return nil
	}

	targetMap := make(map[resource.URN]bool)
	for _, target := range targets {
		targetMap[target] = true
	}

	return targetMap
}

// checkTargets validates that all the targets passed in refer to existing resources.  Diagnostics
// are generated for any target that cannot be found.  The target must either have existed in the stack
// prior to running the operation, or it must be the urn for a resource that was created.
func (pe *planExecutor) checkTargets(targets []resource.URN) result.Result {
	if len(targets) == 0 {
		return nil
	}

	olds := pe.plan.olds
	var news map[resource.URN]bool
	if pe.stepGen != nil && pe.stepGen.creates != nil {
		news = pe.stepGen.creates
	}

	hasUnknownTarget := false
	for _, target := range targets {
		hasOld := false
		if _, has := olds[target]; has {
			hasOld = true
		}

		hasNew := news != nil && news[target]
		if !hasOld && !hasNew {
			hasUnknownTarget = true

			logging.V(7).Infof("Resource to delete (%v) could not be found in the stack.", target)
			if strings.Contains(string(target), "$") {
				pe.plan.Diag().Errorf(diag.GetTargetCouldNotBeFoundError(), target)
			} else {
				pe.plan.Diag().Errorf(diag.GetTargetCouldNotBeFoundDidYouForgetError(), target)
			}
		}
	}

	if hasUnknownTarget {
		return result.Bail()
	}

	return nil
}

// reportExecResult issues an appropriate diagnostic depending on went wrong.
func (pe *planExecutor) reportExecResult(message string, preview bool) {
	kind := "update"
	if preview {
		kind = "preview"
	}

	pe.reportError("", errors.New(kind+" "+message))
}

// reportError reports a single error to the executor's diag stream with the indicated URN for context.
func (pe *planExecutor) reportError(urn resource.URN, err error) {
	pe.plan.Diag().Errorf(diag.RawMessage(urn, err.Error()))
}

// Execute executes a plan to completion, using the given cancellation context and running a preview
// or update.
func (pe *planExecutor) Execute(callerCtx context.Context, opts Options, preview bool) result.Result {
	// Set up a goroutine that will signal cancellation to the plan's plugins if the caller context is cancelled. We do
	// not hang this off of the context we create below because we do not want the failure of a single step to cause
	// other steps to fail.
	done := make(chan bool)
	defer close(done)
	go func() {
		select {
		case <-callerCtx.Done():
			logging.V(4).Infof("planExecutor.Execute(...): signalling cancellation to providers...")
			cancelErr := pe.plan.ctx.Host.SignalCancellation()
			if cancelErr != nil {
				logging.V(4).Infof("planExecutor.Execute(...): failed to signal cancellation to providers: %v", cancelErr)
			}
		case <-done:
			logging.V(4).Infof("planExecutor.Execute(...): exiting provider canceller")
		}
	}()

	// Before doing anything else, optionally refresh each resource in the base checkpoint.
	if opts.Refresh {
		if res := pe.refresh(callerCtx, opts, preview); res != nil {
			return res
		}
		if opts.RefreshOnly {
			return nil
		}
	}

	// The set of -t targets provided on hte command line.  'nil' means 'update everything'.
	// Non-nill means 'update only in this set'.  We don't error if the user specifies an target
	// during `update` that we don't know about because it might be the urn for a resource they
	// want to create.
	updateTargetsOpt := createTargetMap(opts.UpdateTargets)
	destroyTargetsOpt := createTargetMap(opts.DestroyTargets)
	if res := pe.checkTargets(opts.DestroyTargets); res != nil {
		return res
	}

	if updateTargetsOpt != nil && destroyTargetsOpt != nil {
		contract.Failf("Should not be possible to have both .DestroyTargets and .UpdateTargets")
	}

	// Begin iterating the source.
	src, res := pe.plan.source.Iterate(callerCtx, opts, pe.plan)
	if res != nil {
		return res
	}

	// Set up a step generator for this plan.
	pe.stepGen = newStepGenerator(pe.plan, opts)

	// Retire any pending deletes that are currently present in this plan.
	if res := pe.retirePendingDeletes(callerCtx, opts, preview); res != nil {
		return res
	}

	// Derive a cancellable context for this plan. We will only cancel this context if some piece of the plan's
	// execution fails.
	ctx, cancel := context.WithCancel(callerCtx)

	// Start a set of goroutines for step generation.
	var genGroup sync.WaitGroup
	var genError atomic.Value
	handleEvent := func(event SourceEvent) {
		if res := pe.handleSingleEvent(updateTargetsOpt, event); res != nil {
			if resErr := res.Error(); resErr != nil {
				logging.V(4).Infof("planExecutor.Execute(...): error handling event: %v", resErr)
				pe.reportError(pe.plan.generateEventURN(event), resErr)
			}
			genError.Store(true)
			cancel()
		}
	}
	worker := func(events <-chan SourceEvent, launchAsync bool) {
		defer genGroup.Done()

		for {
			select {
			case event := <-events:
				if event == nil {
					return
				}

				if launchAsync {
					genGroup.Add(1)
					go func() {
						defer genGroup.Done()
						handleEvent(event)
					}()
				} else {
					handleEvent(event)
				}

			case <-ctx.Done():
				return
			}
		}
	}

	genEvents := make(chan SourceEvent)
	if opts.InfiniteParallelism() {
		genGroup.Add(1)
		go worker(genEvents, true)
	} else {
		for i := 0; i < opts.DegreeOfParallelism(); i++ {
			genGroup.Add(1)
			go worker(genEvents, false)
		}
	}

	// Set up a step executor for this plan.
	pe.stepExec = newStepExecutor(ctx, cancel, pe.plan, opts, preview, false)

	// We iterate the source in its own goroutine because iteration is blocking and we want the main loop to be able to
	// respond to cancellation requests promptly.
	type nextEvent struct {
		Event  SourceEvent
		Result result.Result
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
				logging.V(4).Infof("planExecutor.Execute(...): incoming events goroutine exiting")
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
	canceled, res := func() (bool, result.Result) {
		logging.V(4).Infof("planExecutor.Execute(...): waiting for incoming events")
		for {
			select {
			case event := <-incomingEvents:
				logging.V(4).Infof("planExecutor.Execute(...): incoming event (nil? %v, %v)", event.Event == nil, event.Result)

				if event.Result != nil {
					if !event.Result.IsBail() {
						pe.reportError("", event.Result.Error())
					}
					cancel()

					// We reported any errors above.  So we can just bail now.
					return false, result.Bail()
				}

				if event.Event == nil {
					close(genEvents)
					genGroup.Wait()

					return false, pe.performDeletes(ctx, updateTargetsOpt, destroyTargetsOpt)
				}

				genEvents <- event.Event
			case <-ctx.Done():
				logging.V(4).Infof("planExecutor.Execute(...): context finished: %v", ctx.Err())

				// NOTE: we use the presence of an error in the caller context in order to distinguish caller-initiated
				// cancellation from internally-initiated cancellation.
				return callerCtx.Err() != nil, nil
			}
		}
	}()

	genGroup.Wait()
	pe.stepExec.WaitForCompletion()
	logging.V(4).Infof("planExecutor.Execute(...): step executor has completed")

	if ge := genError.Load(); ge != nil && ge.(bool) {
		return result.Bail()
	}

	// Now that we've performed all steps in the plan, ensure that the list of targets to update was
	// valid.  We have to do this *after* performing the steps as the target list may have referred
	// to a resource that was created in one of hte steps.
	if res == nil {
		res = pe.checkTargets(opts.UpdateTargets)
	}

	if res != nil && res.IsBail() {
		return res
	}

	// Figure out if execution failed and why. Step generation and execution errors trump cancellation.
	if res != nil || pe.stepExec.Errored() || pe.stepGen.hasPolicyViolations {
		// TODO(cyrusn): We seem to be losing any information about the original 'res's errors.  Should
		// we be doing a merge here?
		pe.reportExecResult("failed", preview)
		return result.Bail()
	} else if canceled {
		pe.reportExecResult("canceled", preview)
		return result.Bail()
	}

	return res
}

func (pe *planExecutor) performDeletes(
	ctx context.Context, updateTargetsOpt, destroyTargetsOpt map[resource.URN]bool) result.Result {

	defer func() {
		// We're done here - signal completion so that the step executor knows to terminate.
		pe.stepExec.SignalCompletion()
	}()

	prev := pe.plan.prev
	if prev == nil || len(prev.Resources) == 0 {
		return nil
	}

	logging.V(7).Infof("performDeletes(...): beginning")

	// At this point we have generated the set of resources above that we would normally want to
	// delete.  However, if the user provided -target's we will only actually delete the specific
	// resources that are in the set explicitly asked for.
	var targetsOpt map[resource.URN]bool
	if updateTargetsOpt != nil {
		targetsOpt = updateTargetsOpt
	} else if destroyTargetsOpt != nil {
		targetsOpt = destroyTargetsOpt
	}

	deleteSteps, res := pe.stepGen.GenerateDeletes(targetsOpt)
	if res != nil {
		logging.V(7).Infof("performDeletes(...): generating deletes produced error result")
		return res
	}

	deletes := pe.stepGen.ScheduleDeletes(deleteSteps)

	// ScheduleDeletes gives us a list of lists of steps. Each list of steps can safely be executed
	// in parallel, but each list must execute completes before the next list can safely begin
	// executing.
	//
	// This is not "true" delete parallelism, since there may be resources that could safely begin
	// deleting but we won't until the previous set of deletes fully completes. This approximation
	// is conservative, but correct.
	for _, antichain := range deletes {
		logging.V(4).Infof("planExecutor.Execute(...): beginning delete antichain")
		tok := pe.stepExec.ExecuteParallel(antichain)
		tok.Wait(ctx)
		logging.V(4).Infof("planExecutor.Execute(...): antichain complete")
	}

	// After executing targeted deletes, we may now have resources that depend on the resource that
	// were deleted.  Go through and clean things up accordingly for them.
	if targetsOpt != nil {
		resourceToStep := make(map[*resource.State]Step)
		for _, step := range deleteSteps {
			resourceToStep[pe.plan.olds[step.URN()]] = step
		}

		pe.rebuildBaseState(resourceToStep, false /*refresh*/)
	}

	return nil
}

// handleSingleEvent handles a single source event. For all incoming events, it produces a chain that needs
// to be executed and schedules the chain for execution.
func (pe *planExecutor) handleSingleEvent(updateTargetsOpt map[resource.URN]bool, event SourceEvent) result.Result {
	contract.Require(event != nil, "event != nil")

	var steps []Step
	var res result.Result
	switch e := event.(type) {
	case RegisterResourceEvent:
		logging.V(4).Infof("planExecutor.handleSingleEvent(...): received RegisterResourceEvent")
		steps, res = pe.stepGen.GenerateSteps(updateTargetsOpt, e)
	case ReadResourceEvent:
		logging.V(4).Infof("planExecutor.handleSingleEvent(...): received ReadResourceEvent")
		steps, res = pe.stepGen.GenerateReadSteps(e)
	case RegisterResourceOutputsEvent:
		logging.V(4).Infof("planExecutor.handleSingleEvent(...): received register resource outputs")
		pe.stepExec.ExecuteRegisterResourceOutputs(e)
		return nil
	}

	if res != nil {
		return res
	}

	pe.stepExec.ExecuteSerial(steps)
	return nil
}

// retirePendingDeletes deletes all resources that are pending deletion. Run before the start of a plan, this pass
// ensures that the engine never sees any resources that are pending deletion from a previous plan.
//
// retirePendingDeletes re-uses the plan executor's step generator but uses its own step executor.
func (pe *planExecutor) retirePendingDeletes(callerCtx context.Context, opts Options, preview bool) result.Result {
	contract.Require(pe.stepGen != nil, "pe.stepGen != nil")
	steps := pe.stepGen.GeneratePendingDeletes()
	if len(steps) == 0 {
		logging.V(4).Infoln("planExecutor.retirePendingDeletes(...): no pending deletions")
		return nil
	}

	logging.V(4).Infof("planExecutor.retirePendingDeletes(...): executing %d steps", len(steps))
	ctx, cancel := context.WithCancel(callerCtx)

	stepExec := newStepExecutor(ctx, cancel, pe.plan, opts, preview, false)
	antichains := pe.stepGen.ScheduleDeletes(steps)
	// Submit the deletes for execution and wait for them all to retire.
	for _, antichain := range antichains {
		for _, step := range antichain {
			pe.plan.Ctx().StatusDiag.Infof(diag.RawMessage(step.URN(), "completing deletion from previous update"))
		}

		tok := stepExec.ExecuteParallel(antichain)
		tok.Wait(ctx)
	}

	stepExec.SignalCompletion()
	stepExec.WaitForCompletion()

	// Like Refresh, we use the presence of an error in the caller's context to detect whether or not we have been
	// cancelled.
	canceled := callerCtx.Err() != nil
	if stepExec.Errored() {
		pe.reportExecResult("failed", preview)
		return result.Bail()
	} else if canceled {
		pe.reportExecResult("canceled", preview)
		return result.Bail()
	}
	return nil
}

// refresh refreshes the state of the base checkpoint file for the current plan in memory.
func (pe *planExecutor) refresh(callerCtx context.Context, opts Options, preview bool) result.Result {
	prev := pe.plan.prev
	if prev == nil || len(prev.Resources) == 0 {
		return nil
	}

	// Make sure if there were any targets specified, that they all refer to existing resources.
	targetMapOpt := createTargetMap(opts.RefreshTargets)
	if res := pe.checkTargets(opts.RefreshTargets); res != nil {
		return res
	}

	// If the user did not provide any --target's, create a refresh step for each resource in the
	// old snapshot.  If they did provider --target's then only create refresh steps for those
	// specific targets.
	steps := []Step{}
	resourceToStep := map[*resource.State]Step{}
	for _, res := range prev.Resources {
		if targetMapOpt == nil || targetMapOpt[res.URN] {
			step := NewRefreshStep(pe.plan, res, nil)
			steps = append(steps, step)
			resourceToStep[res] = step
		}
	}

	// Fire up a worker pool and issue each refresh in turn.
	ctx, cancel := context.WithCancel(callerCtx)
	stepExec := newStepExecutor(ctx, cancel, pe.plan, opts, preview, true)
	stepExec.ExecuteParallel(steps)
	stepExec.SignalCompletion()
	stepExec.WaitForCompletion()

	pe.rebuildBaseState(resourceToStep, true /*refresh*/)

	// NOTE: we use the presence of an error in the caller context in order to distinguish caller-initiated
	// cancellation from internally-initiated cancellation.
	canceled := callerCtx.Err() != nil

	if stepExec.Errored() {
		pe.reportExecResult("failed", preview)
		return result.Bail()
	} else if canceled {
		pe.reportExecResult("canceled", preview)
		return result.Bail()
	}
	return nil
}

func (pe *planExecutor) rebuildBaseState(resourceToStep map[*resource.State]Step, refresh bool) {
	// Rebuild this plan's map of old resources and dependency graph, stripping out any deleted
	// resources and repairing dependency lists as necessary. Note that this updates the base
	// snapshot _in memory_, so it is critical that any components that use the snapshot refer to
	// the same instance and avoid reading it concurrently with this rebuild.
	//
	// The process of repairing dependency lists is a bit subtle. Because multiple physical
	// resources may share a URN, the ability of a particular URN to be referenced in a dependency
	// list can change based on the dependent resource's position in the resource list. For example,
	// consider the following list of resources, where each resource is a (URN, ID, Dependencies)
	// tuple:
	//
	//     [ (A, 0, []), (B, 0, [A]), (A, 1, []), (A, 2, []), (C, 0, [A]) ]
	//
	// Let `(A, 0, [])` and `(A, 2, [])` be deleted by the refresh. This produces the following
	// intermediate list before dependency lists are repaired:
	//
	//     [ (B, 0, [A]), (A, 1, []), (C, 0, [A]) ]
	//
	// In order to repair the dependency lists, we iterate over the intermediate resource list,
	// keeping track of which URNs refer to at least one physical resource at each point in the
	// list, and remove any dependencies that refer to URNs that do not refer to any physical
	// resources. This process produces the following final list:
	//
	//     [ (B, 0, []), (A, 1, []), (C, 0, [A]) ]
	//
	// Note that the correctness of this process depends on the fact that the list of resources is a
	// topological sort of its corresponding dependency graph, so a resource always appears in the
	// list after any resources on which it may depend.
	resources := []*resource.State{}
	referenceable := make(map[resource.URN]bool)
	olds := make(map[resource.URN]*resource.State)
	for _, s := range pe.plan.prev.Resources {
		var old, new *resource.State
		if step, has := resourceToStep[s]; has {
			// We produces a refresh step for this specific resource.  Use the new information about
			// its dependencies during the update.
			old = step.Old()
			new = step.New()
		} else {
			// We didn't do anything with this resource.  However, we still may want to update its
			// dependencies.  So use this resource itself as the 'new' one to update.
			old = s
			new = s
		}

		if new == nil {
			if refresh {
				contract.Assert(old.Custom)
				contract.Assert(!providers.IsProviderType(old.Type))
			}
			continue
		}

		// Remove any deleted resources from this resource's dependency list.
		if len(new.Dependencies) != 0 {
			deps := make([]resource.URN, 0, len(new.Dependencies))
			for _, d := range new.Dependencies {
				if referenceable[d] {
					deps = append(deps, d)
				}
			}
			new.Dependencies = deps
		}

		// Add this resource to the resource list and mark it as referenceable.
		resources = append(resources, new)
		referenceable[new.URN] = true

		// Do not record resources that are pending deletion in the "olds" lookup table.
		if !new.Delete {
			olds[new.URN] = new
		}
	}

	pe.plan.prev.Resources = resources
	pe.plan.olds, pe.plan.depGraph = olds, graph.NewDependencyGraph(resources)
}
