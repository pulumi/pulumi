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
	"errors"
	"fmt"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/pkg/v3/resource/graph"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
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
func (ex *deploymentExecutor) Execute(callerCtx context.Context, opts Options, preview bool) (*Plan, error) {
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
		if err := ex.refresh(callerCtx, opts, preview); err != nil {
			return nil, err
		}
		if opts.RefreshOnly {
			return nil, nil
		}
	} else if ex.deployment.prev != nil && len(ex.deployment.prev.PendingOperations) > 0 && !preview {
		// Print a warning for users that there are pending operations.
		// Explain that these operations can be cleared using pulumi refresh (except for CREATE operations)
		// since these require user intevention:
		ex.printPendingOperationsWarning()
	}

	if err := ex.checkTargets(opts.ReplaceTargets); err != nil {
		return nil, err
	}

	// Begin iterating the source.
	src, err := ex.deployment.source.Iterate(callerCtx, opts, ex.deployment)
	if err != nil {
		return nil, err
	}

	// Set up a step generator for this deployment.
	ex.stepGen = newStepGenerator(ex.deployment, opts, opts.Targets, opts.ReplaceTargets)

	// Derive a cancellable context for this deployment. We will only cancel this context if some piece of the
	// deployment's execution fails.
	ctx, cancel := context.WithCancel(callerCtx)

	// Set up a step generator and executor for this deployment.
	ex.stepExec = newStepExecutor(ctx, cancel, ex.deployment, opts, preview, false)

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

	// The main loop. We'll continuously select for incoming events and the cancellation signal. There are
	// a three ways we can exit this loop:
	//  1. The SourceIterator sends us a `nil` event. This means that we're done processing source events and
	//     we should begin processing deletes.
	//  2. The SourceIterator sends us an error. This means some error occurred in the source program and we
	//     should bail.
	//  3. The stepExecCancel cancel context gets canceled. This means some error occurred in the step executor
	//     and we need to bail. This can also happen if the user hits Ctrl-C.
	canceled, err := func() (bool, error) {
		logging.V(4).Infof("deploymentExecutor.Execute(...): waiting for incoming events")
		for {
			select {
			case event := <-incomingEvents:
				logging.V(4).Infof("deploymentExecutor.Execute(...): incoming event (nil? %v, %v)", event.Event == nil,
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
					// Check targets before performDeletes mutates the initial Snapshot.
					targetErr := ex.checkTargets(opts.Targets)

					err := ex.performDeletes(ctx, opts.Targets)
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

				if err := ex.handleSingleEvent(event.Event); err != nil {
					if !result.IsBail(err) {
						logging.V(4).Infof("deploymentExecutor.Execute(...): error handling event: %v", err)
						ex.reportError(ex.deployment.generateEventURN(event.Event), err)
					}
					cancel()
					return false, result.BailError(err)
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
	stepExecutorError := ex.stepExec.Errored()
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
		ex.reportExecResult("failed", preview)
		if err != nil {
			return nil, result.BailError(err)
		}
		if stepExecutorError != nil {
			return nil, result.BailErrorf("step executor errored: %w", stepExecutorError)
		}
		return nil, result.BailErrorf("step generator errored")
	} else if canceled {
		ex.reportExecResult("canceled", preview)
		return nil, result.BailErrorf("canceled")
	}

	return ex.deployment.newPlans.plan(), err
}

func (ex *deploymentExecutor) performDeletes(
	ctx context.Context, targetsOpt UrnTargets,
) error {
	defer func() {
		// We're done here - signal completion so that the step executor knows to terminate.
		ex.stepExec.SignalCompletion()
	}()

	prev := ex.deployment.prev
	if prev == nil || len(prev.Resources) == 0 {
		return nil
	}

	logging.V(7).Infof("performDeletes(...): beginning")

	// GenerateDeletes mutates state we need to lock the step executor while we do this.
	ex.stepExec.Lock()

	// At this point we have generated the set of resources above that we would normally want to
	// delete.  However, if the user provided -target's we will only actually delete the specific
	// resources that are in the set explicitly asked for.
	deleteSteps, err := ex.stepGen.GenerateDeletes(targetsOpt)
	// Regardless of if this error'd or not the step executor needs unlocking
	ex.stepExec.Unlock()
	if err != nil {
		logging.V(7).Infof("performDeletes(...): generating deletes produced error result")
		return err
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
	if targetsOpt.IsConstrained() {
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
func (ex *deploymentExecutor) handleSingleEvent(event SourceEvent) error {
	contract.Requiref(event != nil, "event", "must not be nil")

	var steps []Step
	var err error
	switch e := event.(type) {
	case RegisterResourceEvent:
		logging.V(4).Infof("deploymentExecutor.handleSingleEvent(...): received RegisterResourceEvent")
		steps, err = ex.stepGen.GenerateSteps(e)
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

	ex.stepExec.ExecuteSerial(steps)
	return nil
}

// import imports a list of resources into a stack.
func (ex *deploymentExecutor) importResources(
	callerCtx context.Context,
	opts Options,
	preview bool,
) (*Plan, error) {
	if len(ex.deployment.imports) == 0 {
		return nil, nil
	}

	// Create an executor for this import.
	ctx, cancel := context.WithCancel(callerCtx)
	stepExec := newStepExecutor(ctx, cancel, ex.deployment, opts, preview, true)

	importer := &importer{
		deployment: ex.deployment,
		executor:   stepExec,
		preview:    preview,
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
			ex.reportExecResult(fmt.Sprintf("failed: %s", err), preview)
		} else {
			ex.reportExecResult("failed", preview)
		}
		if err != nil {
			return nil, result.BailError(err)
		}
		return nil, result.BailErrorf("step executor errored: %w", stepExecutorError)
	} else if canceled {
		ex.reportExecResult("canceled", preview)
		return nil, result.BailErrorf("canceled")
	}
	return ex.deployment.newPlans.plan(), nil
}

// refresh refreshes the state of the base checkpoint file for the current deployment in memory.
func (ex *deploymentExecutor) refresh(callerCtx context.Context, opts Options, preview bool) error {
	prev := ex.deployment.prev
	if prev == nil || len(prev.Resources) == 0 {
		return nil
	}

	// Make sure if there were any targets specified, that they all refer to existing resources.
	if err := ex.checkTargets(opts.Targets); err != nil {
		return err
	}

	// If the user did not provide any --target's, create a refresh step for each resource in the
	// old snapshot.  If they did provider --target's then only create refresh steps for those
	// specific targets.
	steps := []Step{}
	resourceToStep := map[*resource.State]Step{}
	for _, res := range prev.Resources {
		if opts.Targets.Contains(res.URN) {
			// For each resource we're going to refresh we need to ensure we have a provider for it
			err := ex.deployment.EnsureProvider(res.Provider)
			if err != nil {
				return fmt.Errorf("could not load provider for resource %v: %w", res.URN, err)
			}

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

	stepExecutorError := stepExec.Errored()
	if stepExecutorError != nil {
		ex.reportExecResult("failed", preview)
		return result.BailErrorf("step executor errored: %w", stepExecutorError)
	} else if canceled {
		ex.reportExecResult("canceled", preview)
		return result.BailErrorf("canceled")
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
				contract.Assertf(old.Custom, "expected custom resource")
				contract.Assertf(!providers.IsProviderType(old.Type), "expected non-provider resource")
			}
			continue
		}

		// Remove any deleted resources from this resource's dependency list.
		if len(new.Dependencies) != 0 {
			deps := slice.Prealloc[resource.URN](len(new.Dependencies))
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

	undangleParentResources(olds, resources)

	ex.deployment.prev.Resources = resources
	ex.deployment.olds, ex.deployment.depGraph = olds, graph.NewDependencyGraph(resources)
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
