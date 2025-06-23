// Copyright 2016-2024, Pulumi Corporation.
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
	"errors"
	"fmt"
	"strings"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/pkg/v3/resource/graph"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/urn"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
)

// deploymentExecutor is responsible for taking a deployment and driving it to completion.
// Its primary responsibility is to own a `stepGenerator` and `stepExecutor`, serving
// as the glue that links the two subsystems together.
type deploymentExecutor struct {
	deployment *Deployment // The deployment that we are executing

	stepGen  *stepGenerator // step generator owned by this deployment
	stepExec *stepExecutor  // step executor owned by this deployment

	skipped mapset.Set[urn.URN] // The set of resources that have failed

	// The number of expected events remaining from step generaton, this tells us we're still expecting events
	// to be posted back to us from async work such as DiffSteps.
	asyncEventsExpected int32
}

// checkTargets validates that all the targets passed in refer to existing resources.  Diagnostics
// are generated for any target that cannot be found.  The target must either have existed in the stack
// prior to running the operation, or it must be the urn for a resource that was created.
func (ex *deploymentExecutor) checkTargets(targets UrnTargets) error {
	if !targets.IsConstrained() {
		return nil
	}

	olds := ex.deployment.olds
	var news map[resource.URN]bool
	if ex.stepGen != nil {
		news = ex.stepGen.urns
	}

	hasUnknownTarget := false
	for _, target := range targets.Literals() {
		hasOld := olds != nil && olds[target] != nil
		hasNew := news != nil && news[target]
		if !hasOld && !hasNew {
			hasUnknownTarget = true

			logging.V(7).Infof("Targeted resource could not be found in the stack [urn=%v]", target)
			if strings.Contains(string(target), "$") {
				ex.deployment.Diag().Errorf(diag.GetTargetCouldNotBeFoundError(), target)
			} else {
				ex.deployment.Diag().Errorf(diag.GetTargetCouldNotBeFoundDidYouForgetError(), target)
			}
		}
	}

	if hasUnknownTarget {
		return result.BailErrorf("one or more targets could not be found in the stack")
	}

	return nil
}

func (ex *deploymentExecutor) printPendingOperationsWarning() {
	pendingOperations := ""
	for _, op := range ex.deployment.prev.PendingOperations {
		pendingOperations = pendingOperations + fmt.Sprintf("  * %s, interrupted while %s\n", op.Resource.URN, op.Type)
	}

	resolutionMessage := "" +
		"These resources are in an unknown state because the Pulumi CLI was interrupted while " +
		"waiting for changes to these resources to complete. You should confirm whether or not the " +
		"operations listed completed successfully by checking the state of the appropriate provider. " +
		"For example, if you are using AWS, you can confirm using the AWS Console.\n" +
		"\n" +
		"Once you have confirmed the status of the interrupted operations, you can repair your stack " +
		"using `pulumi refresh` which will refresh the state from the provider you are using and " +
		"clear the pending operations if there are any.\n" +
		"\n" +
		"Note that `pulumi refresh` will need to be run interactively to clear pending CREATE operations."

	warning := "Attempting to deploy or update resources " +
		fmt.Sprintf("with %d pending operations from previous deployment.\n", len(ex.deployment.prev.PendingOperations)) +
		pendingOperations +
		resolutionMessage

	ex.deployment.Diag().Warningf(diag.RawMessage("" /*urn*/, warning))
}

// reportExecResult issues an appropriate diagnostic depending on went wrong.
func (ex *deploymentExecutor) reportExecResult(message string) {
	kind := "update"
	if ex.deployment.opts.DryRun {
		kind = "preview"
	}

	ex.reportError("", errors.New(kind+" "+message))
}

// reportError reports a single error to the executor's diag stream with the indicated URN for context.
func (ex *deploymentExecutor) reportError(urn resource.URN, err error) {
	ex.deployment.Diag().Errorf(diag.RawMessage(urn, err.Error()))
}

// Execute executes a deployment to completion, using the given cancellation context and running a preview
// or update.
func (ex *deploymentExecutor) Execute(callerCtx context.Context) (_ *Plan, err error) {
	// Set up a goroutine that will signal cancellation to the deployment's plugins if the caller context is cancelled.
	// We do not hang this off of the context we create below because we do not want the failure of a single step to
	// cause other steps to fail.
	ex.skipped = mapset.NewSet[urn.URN]()
	done := make(chan bool)
	defer close(done)
	go func() {
		select {
		case <-callerCtx.Done():
			logging.V(4).Infof("deploymentExecutor.Execute(...): signalling cancellation to providers...")
			cancelErr := ex.deployment.ctx.Host.SignalCancellation()
			if cancelErr != nil {
				logging.V(4).Infof("deploymentExecutor.Execute(...): failed to signal cancellation to providers: %v", cancelErr)
			}
		case <-done:
			logging.V(4).Infof("deploymentExecutor.Execute(...): exiting provider canceller")
		}
	}()

	// Close the deployment when we're finished.
	defer contract.IgnoreClose(ex.deployment)

	// If this deployment is an import, run the imports and exit.
	if ex.deployment.isImport {
		return ex.importResources(callerCtx)
	}

	// Before doing anything else, optionally refresh each resource in the base checkpoint. If we're using
	// refresh programs then we don't do this, we just run the step generator in refresh mode later.
	if ex.deployment.opts.Refresh && !ex.deployment.opts.RefreshProgram {
		if err := ex.refresh(callerCtx, false /*refreshBeforeUpdateOnly*/); err != nil {
			return nil, err
		}
		if ex.deployment.opts.RefreshOnly {
			return nil, nil
		}
	} else if !ex.deployment.opts.Refresh &&
		!ex.deployment.opts.RefreshProgram &&
		ex.deployment.hasRefreshBeforeUpdateResources {
		// If there are resources that require a refresh before update, run a refresh for those resources.
		if err := ex.refresh(callerCtx, true /*refreshBeforeUpdateOnly*/); err != nil {
			return nil, err
		}
	} else if ex.deployment.prev != nil && len(ex.deployment.prev.PendingOperations) > 0 && !ex.deployment.opts.DryRun {
		// Print a warning for users that there are pending operations.
		// Explain that these operations can be cleared using pulumi refresh (except for CREATE operations)
		// since these require user intevention:
		ex.printPendingOperationsWarning()
	}

	if err := ex.checkTargets(ex.deployment.opts.ReplaceTargets); err != nil {
		return nil, err
	}

	// Begin iterating the source.
	src, err := ex.deployment.source.Iterate(callerCtx, ex.deployment)
	if err != nil {
		return nil, err
	}
	defer func() {
		// If the context was canceled, we want to return quickly here and not wait for the program to complete.
		closeErr := src.Cancel(callerCtx)
		if closeErr != nil {
			logging.V(4).Infof("deploymentExecutor.Execute(...): source iterator closed with error: %s", closeErr)
			if err == nil {
				// If we didn't see any earlier error, report the close error and bail.
				ex.reportError("", closeErr)
				err = result.BailError(closeErr)
			}
		}
	}()

	// Set up a step generator for this deployment.
	mode := updateMode
	if ex.deployment.opts.DestroyProgram {
		mode = destroyMode
	}
	// If we're doing a program-based refresh we'll get down to here and we need to put the step generator into
	// refresh mode.
	refresh := ex.deployment.opts.RefreshProgram && ex.deployment.opts.Refresh
	if ex.deployment.opts.RefreshOnly {
		mode = refreshMode
	}
	// As well as generating steps from events produced by the source, the step generator can also generate
	// events in order to support concurrency during step generation (e.g. for parallel diffing). We thus pass
	// a channel that the step generator can write to in order to yield/resume at these points. We then pump
	// these events back to the step gen in the main loop with program events.
	stepGenEvents := make(chan SourceEvent)
	ex.stepGen = newStepGenerator(ex.deployment, refresh, mode, stepGenEvents)

	// Derive a cancellable context for this deployment. We will only cancel this context if some piece of the
	// deployment's execution fails.
	ctx, cancel := context.WithCancel(callerCtx)

	// Set up a step generator and executor for this deployment.
	ex.stepExec = newStepExecutor(ctx, cancel, ex.deployment, false)

	// We iterate the source in its own goroutine because iteration is blocking and we want the main loop to be able to
	// respond to cancellation requests promptly.
	type nextEvent struct {
		Event SourceEvent
		Error error
	}
	incomingEvents := make(chan nextEvent)
	go func() {
		for {
			event, err := src.Next()
			select {
			case incomingEvents <- nextEvent{event, err}:
				if event == nil {
					return
				}
			case <-done:
				logging.V(4).Infof("deploymentExecutor.Execute(...): incoming events goroutine exiting")
				return
			}
		}
	}()

	// The main loop. We'll continuously select for incoming events and the cancellation signal. There are three ways
	// we can exit this loop:
	//  1. The SourceIterator sends us a `nil` event and the step generator has completed all its async work.
	//     This means that we're done processing source events and we should begin processing deletes.
	//  2. The SourceIterator sends us an error. This means some error occurred in the source program and we
	//     should bail.
	//  3. The stepExecCancel cancel context gets canceled. This means some error occurred in the step executor
	//     and we need to bail. This can also happen if the user hits Ctrl-C.
	canceled, err := func() (bool, error) {
		logging.V(4).Infof("deploymentExecutor.Execute(...): waiting for incoming events")

		// We're ingesting events from two sources: the source iterator and the step generator. We need to make sure
		// that both are done before we exit the loop. The source iterator is done when it sends us nil. The step
		// generator is done when its async counter is 0, i.e. for each async event it said it was going to do we've
		// seen and posted that event back to it.
		seenNil := false
		for {
			select {
			case event := <-stepGenEvents:
				logging.V(4).Infof("deploymentExecutor.Execute(...): incoming async event")

				if err := ex.handleSingleEvent(event); err != nil {
					if !result.IsBail(err) {
						logging.V(4).Infof("deploymentExecutor.Execute(...): error handling event: %v", err)
						ex.reportError(ex.deployment.generateEventURN(event), err)
					}
					cancel()
					return false, result.BailError(err)
				}

			case event := <-incomingEvents:
				logging.V(4).Infof("deploymentExecutor.Execute(...): incoming source event (nil? %v, %v)", event.Event == nil,
					event.Error)

				if event.Error != nil {
					if !result.IsBail(event.Error) {
						ex.reportError("", event.Error)
					}
					cancel()

					// We reported any errors above.  So we can just bail now.
					return false, result.BailError(event.Error)
				}

				if event.Event == nil {
					seenNil = true
				} else {
					if err := ex.handleSingleEvent(event.Event); err != nil {
						if !result.IsBail(err) {
							logging.V(4).Infof("deploymentExecutor.Execute(...): error handling event: %v", err)
							ex.reportError(ex.deployment.generateEventURN(event.Event), err)
						}
						cancel()
						return false, result.BailError(err)
					}
				}
			case <-ctx.Done():
				logging.V(4).Infof("deploymentExecutor.Execute(...): context finished: %v", ctx.Err())

				// NOTE: we use the presence of an error in the caller context in order to distinguish caller-initiated
				// cancellation from internally-initiated cancellation.
				return callerCtx.Err() != nil, nil
			}

			// Exit if we've seen a nil event and the step generator has no more async work to do. See the comment at
			// the top of the loop for more details.
			if seenNil && ex.asyncEventsExpected == 0 {
				// Check targets before performDeletes mutates the initial Snapshot.
				targetErr := ex.checkTargets(ex.deployment.opts.Targets)

				err := ex.performPostSteps(ctx, ex.deployment.opts.Targets, ex.deployment.opts.Excludes)
				if err != nil {
					if !result.IsBail(err) {
						logging.V(4).Infof("deploymentExecutor.Execute(...): error performing deletes: %v", err)
						ex.reportError("", err)
						return false, result.BailError(err)
					}
				}

				if targetErr != nil {
					// Propagate the target error as it hasn't been reported yet.
					return false, targetErr
				}
				return false, nil
			}
		}
	}()

	ex.stepExec.WaitForCompletion()

	stepExecutorError := ex.stepExec.Errored()

	// Finalize the stack outputs.
	if e := ex.stepExec.stackOutputsEvent; e != nil {
		errored := err != nil || stepExecutorError != nil || ex.stepGen.Errored()
		finalizingStackOutputs := true
		if err := ex.stepExec.executeRegisterResourceOutputs(e, errored, finalizingStackOutputs); err != nil {
			return nil, result.BailError(err)
		}
	}

	logging.V(4).Infof("deploymentExecutor.Execute(...): step executor has completed")

	// Check that we did operations for everything expected in the plan. We mutate ResourcePlan.Ops as we run
	// so by the time we get here everything in the map should have an empty ops list (except for unneeded
	// deletes). We skip this check if we already have an error, chances are if the deployment failed lots of
	// operations wouldn't have got a chance to run so we'll spam errors about all of those failed operations
	// making it less clear to the user what the root cause error was.
	if err == nil && ex.deployment.plan != nil {
		for urn, resourcePlan := range ex.deployment.plan.ResourcePlans {
			if len(resourcePlan.Ops) != 0 {
				if len(resourcePlan.Ops) == 1 && resourcePlan.Ops[0] == OpDelete {
					// We haven't done a delete for this resource check if it was in the snapshot,
					// if it's already gone this wasn't done because it wasn't needed
					found := false
					for i := range ex.deployment.prev.Resources {
						if ex.deployment.prev.Resources[i].URN == urn {
							found = true
							break
						}
					}

					// Didn't find the resource in the old snapshot so this was just an unneeded delete
					if !found {
						continue
					}
				}

				rErr := fmt.Errorf("expected resource operations for %v but none were seen", urn)
				logging.V(4).Infof("deploymentExecutor.Execute(...): error handling event: %v", rErr)
				ex.reportError(urn, rErr)
				err = errors.Join(err, rErr)
			}
		}
		// If we made any errors above wrap it in a bail
		if err != nil {
			err = result.BailError(err)
		}
	}

	if err != nil && result.IsBail(err) {
		return nil, err
	}

	// If the step generator and step executor were both successful, then we send all the resources
	// observed to be analyzed. Otherwise, this step is skipped.
	if err == nil && stepExecutorError == nil {
		err := ex.stepGen.AnalyzeResources()
		if err != nil {
			if !result.IsBail(err) {
				logging.V(4).Infof("deploymentExecutor.Execute(...): error analyzing resources: %v", err)
				ex.reportError("", err)
			}
			return nil, result.BailErrorf("failed to analyze resources: %v", err)
		}
	}

	// Figure out if execution failed and why. Step generation and execution errors trump cancellation.
	if err != nil || stepExecutorError != nil || ex.stepGen.Errored() {
		// TODO(cyrusn): We seem to be losing any information about the original 'res's errors.  Should
		// we be doing a merge here?
		ex.reportExecResult("failed")
		if err != nil {
			return nil, result.BailError(err)
		}
		if stepExecutorError != nil {
			return nil, result.BailErrorf("step executor errored: %w", stepExecutorError)
		}
		return nil, result.BailErrorf("step generator errored")
	} else if canceled {
		ex.reportExecResult("canceled")
		return nil, result.BailErrorf("canceled")
	}

	return ex.deployment.newPlans.plan(), err
}

// performPostSteps either generates and schedules deletes or refreshes based on what resources were left in
// state after the source program finished registering resources.
func (ex *deploymentExecutor) performPostSteps(
	ctx context.Context, targetsOpt UrnTargets, excludesOpt UrnTargets,
) error {
	signaled := false
	defer func() {
		// We're done here - signal completion so that the step executor knows to terminate.
		if !signaled {
			ex.stepExec.SignalCompletion()
		}
	}()

	prev := ex.deployment.prev
	if prev == nil || len(prev.Resources) == 0 {
		return nil
	}

	logging.V(7).Infof("performPostSteps(...): beginning")

	// GenerateDeletes/Refreshes mutates state we need to lock the step executor while we do this.
	ex.stepExec.Lock()

	if ex.stepGen.mode == refreshMode {
		steps, resourceToStep, err := ex.stepGen.GenerateRefreshes(targetsOpt, excludesOpt)
		// Regardless of if this error'd or not the step executor needs unlocking
		ex.stepExec.Unlock()
		if err != nil {
			logging.V(7).Infof("performPostSteps(...): generating refreshes produced error result")
			return err
		}

		// Fire up a worker pool and issue each refresh in turn.
		ex.stepExec.ExecuteParallel(steps)
		ex.stepExec.SignalCompletion()
		signaled = true
		ex.stepExec.WaitForCompletion()
		ex.rebuildBaseState(resourceToStep)
	} else {
		// At this point we have generated the set of resources above that we would normally want to
		// delete.  However, if the user provided -target's we will only actually delete the specific
		// resources that are in the set explicitly asked for.
		deleteSteps, err := ex.stepGen.GenerateDeletes(targetsOpt, excludesOpt)
		// Regardless of if this error'd or not the step executor needs unlocking
		ex.stepExec.Unlock()
		if err != nil {
			logging.V(7).Infof("performPostSteps(...): generating deletes produced error result")
			return err
		}

		deleteChains := ex.stepGen.ScheduleDeletes(deleteSteps)

		// ScheduleDeletes gives us a list of lists of steps. Each list of steps can safely be executed
		// in parallel, but each list must execute completes before the next list can safely begin
		// executing.
		//
		// This is not "true" delete parallelism, since there may be resources that could safely begin
		// deleting but we won't until the previous set of deletes fully completes. This approximation
		// is conservative, but correct.
		erroredDeps := mapset.NewSet[*resource.State]()
		seenErrors := mapset.NewSet[Step]()
		for _, antichain := range deleteChains {
			erroredSteps := ex.stepExec.GetErroredSteps()
			for _, step := range erroredSteps {
				// If we've already seen this error or the step isn't in the graph we can skip it.
				//
				// We also skip checking for dependencies of the error if it is not in the dependency graph.
				// This can happen if an earlier create failed, thus the resource wouldn't have been added
				// to the graph.  Since the resource was just tried to be  created it couldn't have any dependencies
				// that should be deleted either.
				if seenErrors.Contains(step) {
					continue
				}
				for _, r := range []*resource.State{step.Res(), step.Old()} {
					if r != nil && ex.deployment.depGraph.Contains(r) {
						deps := ex.deployment.depGraph.TransitiveDependenciesOf(r)
						erroredDeps = erroredDeps.Union(deps)
					}
				}
			}
			seenErrors.Append(erroredSteps...)
			newChain := make([]Step, 0, len(antichain))
			for _, step := range antichain {
				if !erroredDeps.Contains(step.Res()) {
					newChain = append(newChain, step)
				}
			}
			antichain = newChain

			logging.V(4).Infof("deploymentExecutor.Execute(...): beginning antichain")
			tok := ex.stepExec.ExecuteParallel(antichain)
			tok.Wait(ctx)
			logging.V(4).Infof("deploymentExecutor.Execute(...): antichain complete")
		}
	}

	return nil
}

func doesStepDependOn(step Step, skipped mapset.Set[urn.URN]) bool {
	_, allDeps := step.Res().GetAllDependencies()
	for _, dep := range allDeps {
		if skipped.Contains(dep.URN) {
			return true
		}
	}

	return false
}

// handleSingleEvent handles a single source event. For all incoming events, it produces a chain that needs
// to be executed and schedules the chain for execution.
func (ex *deploymentExecutor) handleSingleEvent(event SourceEvent) error {
	contract.Requiref(event != nil, "event", "must not be nil")

	var steps []Step
	var err error
	switch e := event.(type) {
	case ContinueResourceImportEvent:
		logging.V(4).Infof("deploymentExecutor.handleSingleEvent(...): received ContinueResourceImportEvent")
		ex.asyncEventsExpected--
		var async bool
		steps, async, err = ex.stepGen.ContinueStepsFromImport(e)
		if async {
			ex.asyncEventsExpected++
		}
	case ContinueResourceRefreshEvent:
		logging.V(4).Infof("deploymentExecutor.handleSingleEvent(...): received ContinueResourceRefreshEvent")
		ex.asyncEventsExpected--
		var async bool
		steps, async, err = ex.stepGen.ContinueStepsFromRefresh(e)
		if async {
			ex.asyncEventsExpected++
		}
	case ContinueResourceDiffEvent:
		logging.V(4).Infof("deploymentExecutor.handleSingleEvent(...): received ContinueResourceDiffEvent")
		ex.asyncEventsExpected--
		steps, err = ex.stepGen.ContinueStepsFromDiff(e)
	case RegisterResourceEvent:
		logging.V(4).Infof("deploymentExecutor.handleSingleEvent(...): received RegisterResourceEvent")
		var async bool
		steps, async, err = ex.stepGen.GenerateSteps(e)
		if async {
			ex.asyncEventsExpected++
		}
	case ReadResourceEvent:
		logging.V(4).Infof("deploymentExecutor.handleSingleEvent(...): received ReadResourceEvent")
		steps, err = ex.stepGen.GenerateReadSteps(e)
	case RegisterResourceOutputsEvent:
		logging.V(4).Infof("deploymentExecutor.handleSingleEvent(...): received register resource outputs")
		return ex.stepExec.ExecuteRegisterResourceOutputs(e)
	}

	if err != nil {
		return err
	}
	// Exclude the steps that depend on errored steps if ContinueOnError is set.
	newSteps := slice.Prealloc[Step](len(steps))
	for _, errored := range ex.stepExec.GetErroredSteps() {
		ex.skipped.Add(errored.Res().URN)
	}
	for _, step := range steps {
		if doesStepDependOn(step, ex.skipped) {
			step.Skip()
			ex.skipped.Add(step.Res().URN)
			continue
		}
		newSteps = append(newSteps, step)
	}

	// Don't bother passing an empty chain to the step executor. This might be if we just skipped a step
	// because its dependencies errored out, or because the step gen returned no steps. Return early in that
	// case.
	if len(newSteps) == 0 {
		return nil
	}

	ex.stepExec.ExecuteSerial(newSteps)
	return nil
}

// import imports a list of resources into a stack.
func (ex *deploymentExecutor) importResources(callerCtx context.Context) (*Plan, error) {
	if len(ex.deployment.imports) == 0 {
		return nil, nil
	}

	// Create an executor for this import.
	ctx, cancel := context.WithCancel(callerCtx)
	stepExec := newStepExecutor(ctx, cancel, ex.deployment, true)

	importer := &importer{
		deployment: ex.deployment,
		executor:   stepExec,
	}
	err := importer.importResources(ctx)
	stepExec.SignalCompletion()
	stepExec.WaitForCompletion()

	// NOTE: we use the presence of an error in the caller context in order to distinguish caller-initiated
	// cancellation from internally-initiated cancellation.
	canceled := callerCtx.Err() != nil

	stepExecutorError := stepExec.Errored()
	if err != nil || stepExecutorError != nil {
		if err != nil && !result.IsBail(err) {
			ex.reportExecResult(fmt.Sprintf("failed: %s", err))
		} else {
			ex.reportExecResult("failed")
		}
		if err != nil {
			return nil, result.BailError(err)
		}
		return nil, result.BailErrorf("step executor errored: %w", stepExecutorError)
	} else if canceled {
		ex.reportExecResult("canceled")
		return nil, result.BailErrorf("canceled")
	}
	return ex.deployment.newPlans.plan(), nil
}

// refresh refreshes the state of the base checkpoint file for the current deployment in memory.
func (ex *deploymentExecutor) refresh(callerCtx context.Context, refreshBeforeUpdateOnly bool) error {
	prev := ex.deployment.prev
	if prev == nil || len(prev.Resources) == 0 {
		return nil
	}

	// Make sure if there were any targets specified, that they all refer to existing resources.
	if err := ex.checkTargets(ex.deployment.opts.Targets); err != nil {
		return err
	}

	// Make sure all specified excludes refer to existing resources.
	if err := ex.checkTargets(ex.deployment.opts.Excludes); err != nil {
		return err
	}

	// If the user did not provide any --target's, create a refresh step for each resource in the
	// old snapshot.  If they did provider --target's then only create refresh steps for those
	// specific targets.
	steps := []Step{}
	resourceToStep := map[*resource.State]Step{}

	// We also keep track of dependents as we find them in order to exclude
	// transitive dependents as well.
	if ex.deployment.opts.Excludes.IsConstrained() {
		excludesActual := ex.deployment.opts.Excludes

		for _, res := range prev.Resources {
			// If we're only doing RefreshBeforeUpdate refreshes and the resource isn't
			// marked as such, skip it.
			if refreshBeforeUpdateOnly && !res.RefreshBeforeUpdate {
				continue
			}

			// If the resource is known to be excluded, we can skip this step
			// entirely at this point.
			if excludesActual.Contains(res.URN) {
				continue
			}

			// If the resource is a view, skip it. Only the owning resource
			// should have a refresh step.
			if res.ViewOf != "" {
				continue
			}

			knownToBeExcluded := false

			// In the case of `--exclude-dependents`, we need to check through all
			// the dependencies to see if they have already been marked as excluded.
			// If so, this dependent is also to be excluded, and we add it to the
			// list of known excludes to catch transitive excludes as well.
			if ex.deployment.opts.ExcludeDependents {
				_, allDeps := res.GetAllDependencies()

				for _, dep := range allDeps {
					if excludesActual.Contains(dep.URN) {
						excludesActual.addLiteral(res.URN)

						knownToBeExcluded = true
						break
					}
				}
			}

			if !knownToBeExcluded {
				// For each resource we're going to refresh we need to ensure we have a provider for it
				err := ex.deployment.EnsureProvider(res.Provider)
				if err != nil {
					return fmt.Errorf("could not load provider for resource %v: %w", res.URN, err)
				}

				oldViews := ex.deployment.GetOldViews(res.URN)
				step := NewRefreshStep(ex.deployment, nil, res, oldViews, nil)
				steps = append(steps, step)
				resourceToStep[res] = step
			}
		}
	} else {
		targetsActual := ex.deployment.opts.Targets

		for _, res := range prev.Resources {
			// If we're only doing RefreshBeforeUpdate refreshes and the resource isn't
			// marked as such, skip it.
			if refreshBeforeUpdateOnly && !res.RefreshBeforeUpdate {
				continue
			}

			// If the resource is a view, skip it. Only the owning resource
			// should have a refresh step.
			if res.ViewOf != "" {
				continue
			}

			if targetsActual.Contains(res.URN) {
				// For each resource we're going to refresh we need to ensure we have a provider for it
				err := ex.deployment.EnsureProvider(res.Provider)
				if err != nil {
					return fmt.Errorf("could not load provider for resource %v: %w", res.URN, err)
				}

				oldViews := ex.deployment.GetOldViews(res.URN)
				step := NewRefreshStep(ex.deployment, nil, res, oldViews, nil)
				steps = append(steps, step)
				resourceToStep[res] = step
			} else if ex.deployment.opts.TargetDependents {
				// The provider reference is already ensured.
				_, allDeps := res.GetAllDependencies()

				// Because we always visit a target before its dependents, these
				// dependents will all be caught by the check at the start of this
				// loop.
				for _, dep := range allDeps {
					if targetsActual.Contains(dep.URN) {
						oldViews := ex.deployment.GetOldViews(res.URN)
						step := NewRefreshStep(ex.deployment, nil, res, oldViews, nil)
						steps = append(steps, step)
						resourceToStep[res] = step

						targetsActual.addLiteral(res.URN)
						break
					}
				}
			}
		}
	}

	// Fire up a worker pool and issue each refresh in turn.
	ctx, cancel := context.WithCancel(callerCtx)

	stepExec := newStepExecutor(ctx, cancel, ex.deployment, true)

	stepExec.ExecuteParallel(steps)
	stepExec.SignalCompletion()
	stepExec.WaitForCompletion()

	// Apply view refresh steps published to the resource status server, if any.
	viewRefreshSteps := ex.deployment.resourceStatus.RefreshSteps()
	for _, s := range ex.deployment.prev.Resources {
		if step, has := viewRefreshSteps[s.URN]; has {
			resourceToStep[s] = step
		}
	}

	ex.rebuildBaseState(resourceToStep)

	// NOTE: we use the presence of an error in the caller context in order to distinguish caller-initiated
	// cancellation from internally-initiated cancellation.
	canceled := callerCtx.Err() != nil

	stepExecutorError := stepExec.Errored()
	if stepExecutorError != nil {
		ex.reportExecResult("failed")
		return result.BailErrorf("step executor errored: %w", stepExecutorError)
	} else if canceled {
		ex.reportExecResult("canceled")
		return result.BailErrorf("canceled")
	}
	return nil
}

func (ex *deploymentExecutor) rebuildBaseState(resourceToStep map[*resource.State]Step) {
	// Rebuild this deployment's map of old resources and dependency graph, stripping out any deleted
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
	oldViews := make(map[resource.URN][]*resource.State)
	for _, s := range ex.deployment.prev.Resources {
		var old, new *resource.State
		if step, has := resourceToStep[s]; has {
			// We produced a refresh step for this specific resource.  Use the new information about
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
			contract.Assertf(old.Custom || old.ViewOf != "", "expected custom or view resource")
			contract.Assertf(!providers.IsProviderType(old.Type), "expected non-provider resource")
			continue
		}

		newDeps := []resource.URN{}
		newPropDeps := map[resource.PropertyKey][]resource.URN{}

		_, allDeps := new.GetAllDependencies()
		for _, dep := range allDeps {
			switch dep.Type {
			case resource.ResourceParent:
				// We handle parents separately later on (see undangleParentResources),
				// so we'll skip over them here.
				continue
			case resource.ResourceDependency:
				if referenceable[dep.URN] {
					newDeps = append(newDeps, dep.URN)
				}
			case resource.ResourcePropertyDependency:
				if referenceable[dep.URN] {
					newPropDeps[dep.Key] = append(newPropDeps[dep.Key], dep.URN)
				}
			case resource.ResourceDeletedWith:
				if !referenceable[dep.URN] {
					new.DeletedWith = ""
				}
			}
		}

		// Since we can only have shrunk the sets of dependencies and property
		// dependencies, we'll only update them if they were non empty to begin
		// with. This is to avoid e.g. replacing a nil input with an non-nil but
		// empty output, which while equivalent in many cases is not the same and
		// could result in subtly different behaviour in some parts of the engine.
		if len(new.Dependencies) > 0 {
			new.Dependencies = newDeps
		}
		if len(new.PropertyDependencies) > 0 {
			new.PropertyDependencies = newPropDeps
		}

		// Add this resource to the resource list and mark it as referenceable.
		resources = append(resources, new)
		referenceable[new.URN] = true

		// Do not record resources that are pending deletion in the "olds" lookup table.
		if !new.Delete {
			olds[new.URN] = new

			// If this resource is a view of another resource, add it to the list of views for that resource.
			if new.ViewOf != "" {
				oldViews[new.ViewOf] = append(oldViews[new.ViewOf], new)
			}
		}
	}

	undangleParentResources(olds, resources)

	ex.deployment.prev.Resources = resources
	ex.deployment.depGraph = graph.NewDependencyGraph(resources)
	ex.deployment.olds = olds
	ex.deployment.oldViews = oldViews
}

func undangleParentResources(undeleted map[resource.URN]*resource.State, resources []*resource.State) {
	// Since a refresh may delete arbitrary resources, we need to handle the case where
	// the parent of a still existing resource is deleted.
	//
	// Invalid parents need to be fixed since otherwise they leave the state invalid, and
	// the user sees an error:
	// ```
	// snapshot integrity failure; refusing to use it: child resource ${validURN} refers to missing parent ${deletedURN}
	// ```
	// To solve the problem we traverse the topologically sorted list of resources in
	// order, setting newly invalidated parent URNS to the URN of the parent's parent.
	//
	// This can be illustrated by an example. Consider the graph of resource parents:
	//
	//         A            xBx
	//       /   \           |
	//    xCx      D        xEx
	//     |     /   \       |
	//     F    G     xHx    I
	//
	// When a capital letter is marked for deletion, it is bracketed by `x`s.
	// We can obtain a topological sort by reading left to right, top to bottom.
	//
	// A..D -> valid parents, so we do nothing
	// E -> The parent of E is marked for deletion, so set E.Parent to E.Parent.Parent.
	//      Since B (E's parent) has no parent, we set E.Parent to "".
	// F -> The parent of F is marked for deletion, so set F.Parent to F.Parent.Parent.
	//      We set F.Parent to "A"
	// G, H -> valid parents, do nothing
	// I -> The parent of I is marked for deletion, so set I.Parent to I.Parent.Parent.
	//      The parent of I has parent "", (since we addressed the parent of E
	//      previously), so we set I.Parent = "".
	//
	// The new graph looks like this:
	//
	//         A        xBx   xEx   I
	//       / | \
	//     xCx F  D
	//          /   \
	//         G    xHx
	// We observe that it is perfectly valid for deleted nodes to be leaf nodes, but they
	// cannot be intermediary nodes.
	_, hasEmptyValue := undeleted[""]
	contract.Assertf(!hasEmptyValue, "the zero value for an URN is not a valid URN")
	availableParents := map[resource.URN]resource.URN{}
	for _, r := range resources {
		if _, ok := undeleted[r.Parent]; !ok {
			// Since existing must obey a topological sort, we have already addressed
			// p.Parent. Since we know that it doesn't dangle, and that r.Parent no longer
			// exists, we set r.Parent as r.Parent.Parent.
			r.Parent = availableParents[r.Parent]
		}
		availableParents[r.URN] = r.Parent
	}
}
