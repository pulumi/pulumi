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
	"strings"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/v2/resource/deploy/providers"
	"github.com/pulumi/pulumi/pkg/v2/resource/graph"
	"github.com/pulumi/pulumi/sdk/v2/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/result"
)

// deploymentExecutor is responsible for taking a deployment and driving it to completion.
// Its primary responsibility is to own a `stepGenerator` and `stepExecutor`, serving
// as the glue that links the two subsystems together.
type deploymentExecutor struct {
	deployment *Deployment // The deployment that we are executing

	stepGen  *stepGenerator // step generator owned by this deployment
	stepExec *stepExecutor  // step executor owned by this deployment
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
func (ex *deploymentExecutor) checkTargets(targets []resource.URN, op StepOp) result.Result {
	if len(targets) == 0 {
		return nil
	}

	olds := ex.deployment.olds
	var news map[resource.URN]bool
	if ex.stepGen != nil {
		news = ex.stepGen.urns
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

			logging.V(7).Infof("Resource to %v (%v) could not be found in the stack.", op, target)
			if strings.Contains(string(target), "$") {
				ex.deployment.Diag().Errorf(diag.GetTargetCouldNotBeFoundError(), target)
			} else {
				ex.deployment.Diag().Errorf(diag.GetTargetCouldNotBeFoundDidYouForgetError(), target)
			}
		}
	}

	if hasUnknownTarget {
		return result.Bail()
	}

	return nil
}

// reportExecResult issues an appropriate diagnostic depending on went wrong.
func (ex *deploymentExecutor) reportExecResult(message string, preview bool) {
	kind := "update"
	if preview {
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
func (ex *deploymentExecutor) Execute(callerCtx context.Context, opts Options, preview bool) result.Result {
	// Set up a goroutine that will signal cancellation to the deployment's plugins if the caller context is cancelled.
	// We do not hang this off of the context we create below because we do not want the failure of a single step to
	// cause other steps to fail.
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

	// If this deployment is an import, run the imports and exit.
	if ex.deployment.isImport {
		return ex.importResources(callerCtx, opts, preview)
	}

	// Before doing anything else, optionally refresh each resource in the base checkpoint.
	if opts.Refresh {
		if res := ex.refresh(callerCtx, opts, preview); res != nil {
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
	replaceTargetsOpt := createTargetMap(opts.ReplaceTargets)
	destroyTargetsOpt := createTargetMap(opts.DestroyTargets)
	if res := ex.checkTargets(opts.ReplaceTargets, OpReplace); res != nil {
		return res
	}
	if res := ex.checkTargets(opts.DestroyTargets, OpDelete); res != nil {
		return res
	}

	if (updateTargetsOpt != nil || replaceTargetsOpt != nil) && destroyTargetsOpt != nil {
		contract.Failf("Should not be possible to have both .DestroyTargets and .UpdateTargets or .ReplaceTargets")
	}

	// Begin iterating the source.
	src, res := ex.deployment.source.Iterate(callerCtx, opts, ex.deployment)
	if res != nil {
		return res
	}

	// Set up a step generator for this deployment.
	ex.stepGen = newStepGenerator(ex.deployment, opts, updateTargetsOpt, replaceTargetsOpt)

	// Retire any pending deletes that are currently present in this deployment.
	if res := ex.retirePendingDeletes(callerCtx, opts, preview); res != nil {
		return res
	}

	// Derive a cancellable context for this deployment. We will only cancel this context if some piece of the
	// deployment's execution fails.
	ctx, cancel := context.WithCancel(callerCtx)

	// Set up a step generator and executor for this deployment.
	ex.stepExec = newStepExecutor(ctx, cancel, ex.deployment, opts, preview, false)

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
				logging.V(4).Infof("deploymentExecutor.Execute(...): incoming events goroutine exiting")
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
		logging.V(4).Infof("deploymentExecutor.Execute(...): waiting for incoming events")
		for {
			select {
			case event := <-incomingEvents:
				logging.V(4).Infof("deploymentExecutor.Execute(...): incoming event (nil? %v, %v)", event.Event == nil,
					event.Result)

				if event.Result != nil {
					if !event.Result.IsBail() {
						ex.reportError("", event.Result.Error())
					}
					cancel()

					// We reported any errors above.  So we can just bail now.
					return false, result.Bail()
				}

				if event.Event == nil {
					return false, ex.performDeletes(ctx, updateTargetsOpt, destroyTargetsOpt)
				}

				if res := ex.handleSingleEvent(event.Event); res != nil {
					if resErr := res.Error(); resErr != nil {
						logging.V(4).Infof("deploymentExecutor.Execute(...): error handling event: %v", resErr)
						ex.reportError(ex.deployment.generateEventURN(event.Event), resErr)
					}
					cancel()
					return false, result.Bail()
				}
			case <-ctx.Done():
				logging.V(4).Infof("deploymentExecutor.Execute(...): context finished: %v", ctx.Err())

				// NOTE: we use the presence of an error in the caller context in order to distinguish caller-initiated
				// cancellation from internally-initiated cancellation.
				return callerCtx.Err() != nil, nil
			}
		}
	}()

	ex.stepExec.WaitForCompletion()
	logging.V(4).Infof("deploymentExecutor.Execute(...): step executor has completed")

	// Now that we've performed all steps in the deployment, ensure that the list of targets to update was
	// valid.  We have to do this *after* performing the steps as the target list may have referred
	// to a resource that was created in one of hte steps.
	if res == nil {
		res = ex.checkTargets(opts.UpdateTargets, OpUpdate)
	}

	if res != nil && res.IsBail() {
		return res
	}

	// If the step generator and step executor were both successful, then we send all the resources
	// observed to be analyzed. Otherwise, this step is skipped.
	if res == nil && !ex.stepExec.Errored() {
		res := ex.stepGen.AnalyzeResources()
		if res != nil {
			if resErr := res.Error(); resErr != nil {
				logging.V(4).Infof("deploymentExecutor.Execute(...): error analyzing resources: %v", resErr)
				ex.reportError("", resErr)
			}
			return result.Bail()
		}
	}

	// Figure out if execution failed and why. Step generation and execution errors trump cancellation.
	if res != nil || ex.stepExec.Errored() || ex.stepGen.Errored() {
		// TODO(cyrusn): We seem to be losing any information about the original 'res's errors.  Should
		// we be doing a merge here?
		ex.reportExecResult("failed", preview)
		return result.Bail()
	} else if canceled {
		ex.reportExecResult("canceled", preview)
		return result.Bail()
	}

	return res
}

func (ex *deploymentExecutor) performDeletes(
	ctx context.Context, updateTargetsOpt, destroyTargetsOpt map[resource.URN]bool) result.Result {

	defer func() {
		// We're done here - signal completion so that the step executor knows to terminate.
		ex.stepExec.SignalCompletion()
	}()

	prev := ex.deployment.prev
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

	deleteSteps, res := ex.stepGen.GenerateDeletes(targetsOpt)
	if res != nil {
		logging.V(7).Infof("performDeletes(...): generating deletes produced error result")
		return res
	}

	deletes := ex.stepGen.ScheduleDeletes(deleteSteps)

	// ScheduleDeletes gives us a list of lists of steps. Each list of steps can safely be executed
	// in parallel, but each list must execute completes before the next list can safely begin
	// executing.
	//
	// This is not "true" delete parallelism, since there may be resources that could safely begin
	// deleting but we won't until the previous set of deletes fully completes. This approximation
	// is conservative, but correct.
	for _, antichain := range deletes {
		logging.V(4).Infof("deploymentExecutor.Execute(...): beginning delete antichain")
		tok := ex.stepExec.ExecuteParallel(antichain)
		tok.Wait(ctx)
		logging.V(4).Infof("deploymentExecutor.Execute(...): antichain complete")
	}

	// After executing targeted deletes, we may now have resources that depend on the resource that
	// were deleted.  Go through and clean things up accordingly for them.
	if targetsOpt != nil {
		resourceToStep := make(map[*resource.State]Step)
		for _, step := range deleteSteps {
			resourceToStep[ex.deployment.olds[step.URN()]] = step
		}

		ex.rebuildBaseState(resourceToStep, false /*refresh*/)
	}

	return nil
}

// handleSingleEvent handles a single source event. For all incoming events, it produces a chain that needs
// to be executed and schedules the chain for execution.
func (ex *deploymentExecutor) handleSingleEvent(event SourceEvent) result.Result {
	contract.Require(event != nil, "event != nil")

	var steps []Step
	var res result.Result
	switch e := event.(type) {
	case RegisterResourceEvent:
		logging.V(4).Infof("deploymentExecutor.handleSingleEvent(...): received RegisterResourceEvent")
		steps, res = ex.stepGen.GenerateSteps(e)
	case ReadResourceEvent:
		logging.V(4).Infof("deploymentExecutor.handleSingleEvent(...): received ReadResourceEvent")
		steps, res = ex.stepGen.GenerateReadSteps(e)
	case RegisterResourceOutputsEvent:
		logging.V(4).Infof("deploymentExecutor.handleSingleEvent(...): received register resource outputs")
		ex.stepExec.ExecuteRegisterResourceOutputs(e)
		return nil
	}

	if res != nil {
		return res
	}

	ex.stepExec.ExecuteSerial(steps)
	return nil
}

// retirePendingDeletes deletes all resources that are pending deletion. Run before the start of a deployment, this pass
// ensures that the engine never sees any resources that are pending deletion from a previous deployment.
//
// retirePendingDeletes re-uses the deployment executor's step generator but uses its own step executor.
func (ex *deploymentExecutor) retirePendingDeletes(callerCtx context.Context, opts Options,
	preview bool) result.Result {

	contract.Require(ex.stepGen != nil, "ex.stepGen != nil")
	steps := ex.stepGen.GeneratePendingDeletes()
	if len(steps) == 0 {
		logging.V(4).Infoln("deploymentExecutor.retirePendingDeletes(...): no pending deletions")
		return nil
	}

	logging.V(4).Infof("deploymentExecutor.retirePendingDeletes(...): executing %d steps", len(steps))
	ctx, cancel := context.WithCancel(callerCtx)

	stepExec := newStepExecutor(ctx, cancel, ex.deployment, opts, preview, false)
	antichains := ex.stepGen.ScheduleDeletes(steps)
	// Submit the deletes for execution and wait for them all to retire.
	for _, antichain := range antichains {
		for _, step := range antichain {
			ex.deployment.Ctx().StatusDiag.Infof(diag.RawMessage(step.URN(), "completing deletion from previous update"))
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
		ex.reportExecResult("failed", preview)
		return result.Bail()
	} else if canceled {
		ex.reportExecResult("canceled", preview)
		return result.Bail()
	}
	return nil
}

// import imports a list of resources into a stack.
func (ex *deploymentExecutor) importResources(callerCtx context.Context, opts Options, preview bool) result.Result {
	if len(ex.deployment.imports) == 0 {
		return nil
	}

	// Create an executor for this import.
	ctx, cancel := context.WithCancel(callerCtx)
	stepExec := newStepExecutor(ctx, cancel, ex.deployment, opts, preview, true)

	importer := &importer{
		deployment: ex.deployment,
		executor:   stepExec,
		preview:    preview,
	}
	res := importer.importResources(ctx)
	stepExec.SignalCompletion()
	stepExec.WaitForCompletion()

	// NOTE: we use the presence of an error in the caller context in order to distinguish caller-initiated
	// cancellation from internally-initiated cancellation.
	canceled := callerCtx.Err() != nil

	if res != nil || stepExec.Errored() {
		if res != nil && res.Error() != nil {
			ex.reportExecResult(fmt.Sprintf("failed: %s", res.Error()), preview)
		} else {
			ex.reportExecResult("failed", preview)
		}
		return result.Bail()
	} else if canceled {
		ex.reportExecResult("canceled", preview)
		return result.Bail()
	}
	return nil
}

// refresh refreshes the state of the base checkpoint file for the current deployment in memory.
func (ex *deploymentExecutor) refresh(callerCtx context.Context, opts Options, preview bool) result.Result {
	prev := ex.deployment.prev
	if prev == nil || len(prev.Resources) == 0 {
		return nil
	}

	// Make sure if there were any targets specified, that they all refer to existing resources.
	targetMapOpt := createTargetMap(opts.RefreshTargets)
	if res := ex.checkTargets(opts.RefreshTargets, OpRefresh); res != nil {
		return res
	}

	// If the user did not provide any --target's, create a refresh step for each resource in the
	// old snapshot.  If they did provider --target's then only create refresh steps for those
	// specific targets.
	steps := []Step{}
	resourceToStep := map[*resource.State]Step{}
	for _, res := range prev.Resources {
		if targetMapOpt == nil || targetMapOpt[res.URN] {
			step := NewRefreshStep(ex.deployment, res, nil)
			steps = append(steps, step)
			resourceToStep[res] = step
		}
	}

	// Fire up a worker pool and issue each refresh in turn.
	ctx, cancel := context.WithCancel(callerCtx)
	stepExec := newStepExecutor(ctx, cancel, ex.deployment, opts, preview, true)
	stepExec.ExecuteParallel(steps)
	stepExec.SignalCompletion()
	stepExec.WaitForCompletion()

	ex.rebuildBaseState(resourceToStep, true /*refresh*/)

	// NOTE: we use the presence of an error in the caller context in order to distinguish caller-initiated
	// cancellation from internally-initiated cancellation.
	canceled := callerCtx.Err() != nil

	if stepExec.Errored() {
		ex.reportExecResult("failed", preview)
		return result.Bail()
	} else if canceled {
		ex.reportExecResult("canceled", preview)
		return result.Bail()
	}
	return nil
}

func (ex *deploymentExecutor) rebuildBaseState(resourceToStep map[*resource.State]Step, refresh bool) {
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

	ex.deployment.prev.Resources = resources
	ex.deployment.olds, ex.deployment.depGraph = olds, graph.NewDependencyGraph(resources)
}
