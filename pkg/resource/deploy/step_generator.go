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
	"path/filepath"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/apitype"

	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/deploy/providers"
	"github.com/pulumi/pulumi/pkg/resource/graph"
	"github.com/pulumi/pulumi/pkg/resource/plugin"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/util/logging"
	"github.com/pulumi/pulumi/pkg/util/result"
)

// stepGenerator is responsible for turning resource events into steps that
// can be fed to the plan executor. It does this by consulting the plan
// and calculating the appropriate step action based on the requested goal
// state and the existing state of the world.
type stepGenerator struct {
	plan *Plan   // the plan to which this step generator belongs
	opts Options // options for this step generator

	// signals that one or more PolicyViolationEvents have been reported to the user, and the plan
	// should terminate in error. This primarily allows `preview` to aggregate many policy violation
	// events and report them all at once.
	hasPolicyViolations bool

	// The mutex used to control acces to shared state, including the resource lock condition variable.
	mutex sync.RWMutex
	// The set of resources that are locked for planning. This is used to ensure that multiple changes to the same
	// resource are not being planned in parallel. This is possible if a resource is part of two DBR trees or is
	// being registered concurrently with a DBR.
	resourceLocks map[resource.URN]bool
	// The condition variable used to control access to resourceLocks. Must use the mutex above for locking.
	resourceCond sync.Cond

	urns           map[resource.URN]bool            // set of URNs discovered for this plan
	reads          map[resource.URN]bool            // set of URNs read for this plan
	deletes        map[resource.URN]bool            // set of URNs deleted in this plan
	replaces       map[resource.URN]bool            // set of URNs replaced in this plan
	updates        map[resource.URN]bool            // set of URNs updated in this plan
	creates        map[resource.URN]bool            // set of URNs created in this plan
	sames          map[resource.URN]bool            // set of URNs that were not changed in this plan
	pendingDeletes map[*resource.State]bool         // set of resources (not URNs!) that are pending deletion
	providers      map[resource.URN]*resource.State // URN map of providers that we have seen so far.

	// a map from URN to a list of property keys that caused the replacement of a dependent resource during a
	// delete-before-replace.
	dependentReplaceKeys map[resource.URN][]resource.PropertyKey
	// a map from old names (aliased URNs) to the new URN that aliased to them.
	aliased map[resource.URN]resource.URN
}

func (sg *stepGenerator) rlocked(fn func()) {
	sg.mutex.RLock()
	defer sg.mutex.RUnlock()

	fn()
}

func (sg *stepGenerator) locked(fn func()) {
	sg.mutex.Lock()
	defer sg.mutex.Unlock()

	fn()
}

// Locks a single resource by URN. The mutex must not be held by the caller prior to calling this function.
func (sg *stepGenerator) lockResource(urn resource.URN) {
	sg.resourceCond.L.Lock()
	defer sg.resourceCond.L.Unlock()

	for sg.resourceLocks[urn] {
		sg.resourceCond.Wait()
	}
	sg.resourceLocks[urn] = true
}

// Unlocks a single resource by URN. The mutex must not be held by the caller prior to calling this function.
func (sg *stepGenerator) unlockResource(urn resource.URN) {
	sg.resourceCond.L.Lock()
	defer func() {
		sg.resourceCond.L.Unlock()
		sg.resourceCond.Signal()
	}()

	delete(sg.resourceLocks, urn)
}

// Attempts to lock a set of resources; returns true if all resource locks were acquired. The mutex *must* be held by
// the caller prior to calling this function.
func (sg *stepGenerator) tryLockResourcesLocked(resources []*resource.State) bool {
	for _, r := range resources {
		if sg.resourceLocks[r.URN] {
			return false
		}
	}
	for _, r := range resources {
		sg.resourceLocks[r.URN] = true
	}
	return true
}

// Unlocks the list of resources. The mutex must not be held by the caller prior to calling this function.
func (sg *stepGenerator) unlockResources(resources []*resource.State) {
	sg.resourceCond.L.Lock()
	defer func() {
		sg.resourceCond.L.Unlock()
		sg.resourceCond.Signal()
	}()

	for _, r := range resources {
		delete(sg.resourceLocks, r.URN)
	}
}

// Unlocks the resources listed in the dependent replaces. The mutex must not be held by the caller prior to calling
// this function.
func (sg *stepGenerator) unlockDependentReplaces(toReplace []dependentReplace) {
	sg.resourceCond.L.Lock()
	defer func() {
		sg.resourceCond.L.Unlock()
		sg.resourceCond.Signal()
	}()

	for _, r := range toReplace {
		delete(sg.resourceLocks, r.res.URN)
	}
}

// GenerateReadSteps is responsible for producing one or more steps required to service
// a ReadResourceEvent coming from the language host.
func (sg *stepGenerator) GenerateReadSteps(event ReadResourceEvent) ([]Step, result.Result) {
	urn := sg.plan.generateURN(event.Parent(), event.Type(), event.Name())
	sg.lockResource(urn)
	defer sg.unlockResource(urn)

	newState := resource.NewState(event.Type(),
		urn,
		true,  /*custom*/
		false, /*delete*/
		event.ID(),
		event.Properties(),
		make(resource.PropertyMap), /* outputs */
		event.Parent(),
		false, /*protect*/
		true,  /*external*/
		event.Dependencies(),
		nil, /* initErrors */
		event.Provider(),
		nil,   /* propertyDependencies */
		false, /* deleteBeforeCreate */
		event.AdditionalSecretOutputs(),
		nil, /* aliases */
		nil, /* customTimeouts */
	)
	old, hasOld := sg.plan.Olds()[urn]

	// If the snapshot has an old resource for this URN and it's not external, we're going
	// to have to delete the old resource and conceptually replace it with the resource we
	// are about to read.
	//
	// We accomplish this through the "read-replacement" step, which atomically reads a resource
	// and marks the resource it is replacing as pending deletion.
	//
	// In the event that the new "read" resource's ID matches the existing resource,
	// we do not need to delete the resource - we know exactly what resource we are going
	// to get from the read.
	//
	// This operation is tentatively called "relinquish" - it semantically represents the
	// release of a resource from the management of Pulumi.
	if hasOld && !old.External && old.ID != event.ID() {
		logging.V(7).Infof(
			"stepGenerator.GenerateReadSteps(...): replacing existing resource %s, ids don't match", urn)
		sg.locked(func() {
			sg.replaces[urn] = true
		})
		return []Step{
			NewReadReplacementStep(sg.plan, event, old, newState),
			NewReplaceStep(sg.plan, old, newState, nil, nil, nil, true),
		}, nil
	}

	if bool(logging.V(7)) && hasOld && old.ID == event.ID() {
		logging.V(7).Infof("stepGenerator.GenerateReadSteps(...): recognized relinquish of resource %s", urn)
	}

	sg.locked(func() {
		sg.reads[urn] = true
	})
	return []Step{
		NewReadStep(sg.plan, event, old, newState),
	}, nil
}

// GenerateSteps produces one or more steps required to achieve the goal state specified by the
// incoming RegisterResourceEvent.
//
// If the given resource is a custom resource, the step generator will invoke Diff and Check on the
// provider associated with that resource. If those fail, an error is returned.
func (sg *stepGenerator) GenerateSteps(
	updateTargetsOpt map[resource.URN]bool, event RegisterResourceEvent) ([]Step, result.Result) {

	var invalid bool // will be set to true if this object fails validation.

	goal := event.Goal()

	// Generate an URN for this new resource, then check for an old resource so that we can figure out if this is a
	// create, delete, etc., and/or to diff.  We look up first by URN and then by any provided aliases.  If it is found
	// using an alias, record that alias so that we do not delete the aliased resource later.
	var oldInputs resource.PropertyMap
	var oldOutputs resource.PropertyMap
	var old *resource.State
	var hasOld bool

	urn := sg.plan.generateURN(goal.Parent, goal.Type, goal.Name)
	sg.lockResource(urn)
	defer sg.unlockResource(urn)

	sg.locked(func() {
		if sg.urns[urn] {
			invalid = true
			// TODO[pulumi/pulumi-framework#19]: improve this error message!
			sg.plan.Diag().Errorf(diag.GetDuplicateResourceURNError(urn), urn)
		}
		sg.urns[urn] = true

		for _, urnOrAlias := range append([]resource.URN{urn}, goal.Aliases...) {
			old, hasOld = sg.plan.Olds()[urnOrAlias]
			if hasOld {
				oldInputs = old.Inputs
				oldOutputs = old.Outputs
				if urnOrAlias != urn {
					if previousAliasURN, alreadyAliased := sg.aliased[urnOrAlias]; alreadyAliased {
						invalid = true
						sg.plan.Diag().Errorf(diag.GetDuplicateResourceAliasError(urn), urnOrAlias, urn, previousAliasURN)
					}
					sg.aliased[urnOrAlias] = urn
				}
				break
			}
		}
	})

	// Create the desired inputs from the goal state
	inputs := goal.Properties
	if hasOld {
		// Set inputs back to their old values (if any) for any "ignored" properties
		processedInputs, res := processIgnoreChanges(inputs, oldInputs, goal.IgnoreChanges)
		if res != nil {
			return nil, res
		}
		inputs = processedInputs
	}

	// Produce a new state object that we'll build up as operations are performed.  Ultimately, this is what will
	// get serialized into the checkpoint file.
	new := resource.NewState(goal.Type, urn, goal.Custom, false, "", inputs, nil, goal.Parent, goal.Protect, false,
		goal.Dependencies, goal.InitErrors, goal.Provider, goal.PropertyDependencies, false,
		goal.AdditionalSecretOutputs, goal.Aliases, &goal.CustomTimeouts)

	// Is this thing a provider resource? If so, stash it - we might need it later when calculating replacement
	// of resources that use this provider.
	if providers.IsProviderType(goal.Type) {
		sg.locked(func() {
			sg.providers[urn] = new
		})
	}

	// Fetch the provider for this resource.
	prov, res := sg.loadResourceProvider(urn, goal.Custom, goal.Provider, goal.Type)
	if res != nil {
		return nil, res
	}

	// We only allow unknown property values to be exposed to the provider if we are performing an update preview.
	allowUnknowns := sg.plan.preview

	// We may be re-creating this resource if it got deleted earlier in the execution of this plan.
	var recreating bool
	sg.rlocked(func() {
		_, recreating = sg.deletes[urn]
	})

	// We may be creating this resource if it previously existed in the snapshot as an External resource
	wasExternal := hasOld && old.External

	// If the goal contains an ID, this may be an import. An import occurs if there is no old resource or if the old
	// resource's ID does not match the ID in the goal state.
	isImport := goal.Custom && goal.ID != "" && (!hasOld || old.External || old.ID != goal.ID)
	if isImport {
		// Write the ID of the resource to import into the new state and return an ImportStep or an
		// ImportReplacementStep
		new.ID = goal.ID
		if isReplace := hasOld && !recreating; isReplace {
			return []Step{
				NewImportReplacementStep(sg.plan, event, old, new, goal.IgnoreChanges),
				NewReplaceStep(sg.plan, old, new, nil, nil, nil, true),
			}, nil
		}
		return []Step{NewImportStep(sg.plan, event, new, goal.IgnoreChanges)}, nil
	}

	// Ensure the provider is okay with this resource and fetch the inputs to pass to subsequent methods.
	var err error
	if prov != nil {
		var failures []plugin.CheckFailure

		// If we are re-creating this resource because it was deleted earlier, the old inputs are now
		// invalid (they got deleted) so don't consider them. Similarly, if the old resource was External,
		// don't consider those inputs since Pulumi does not own them.
		if recreating || wasExternal {
			inputs, failures, err = prov.Check(urn, nil, goal.Properties, allowUnknowns)
		} else {
			inputs, failures, err = prov.Check(urn, oldInputs, inputs, allowUnknowns)
		}

		if err != nil {
			return nil, result.FromError(err)
		} else if issueCheckErrors(sg.plan, new, urn, failures) {
			invalid = true
		}
		new.Inputs = inputs
	}

	// Load all policy packs into the plugin host.
	for _, path := range sg.plan.localPolicyPackPaths {
		abs, err := filepath.Abs(path)
		if err != nil {
			return nil, result.FromError(err)
		}

		var analyzer plugin.Analyzer
		analyzer, err = sg.plan.ctx.Host.PolicyAnalyzer(tokens.QName(abs), path)
		if err != nil {
			return nil, result.FromError(err)
		} else if analyzer == nil {
			return nil, result.Errorf("analyzer could not be loaded from path %q", path)
		}
	}

	// Get the final list of policy packs.
	analyzers := sg.plan.ctx.Host.ListAnalyzers()

	for _, analyzer := range analyzers {
		var diagnostics []plugin.AnalyzeDiagnostic
		diagnostics, err = analyzer.Analyze(new.Type, inputs)
		if err != nil {
			return nil, result.FromError(err)
		}
		for _, d := range diagnostics {
			// TODO(hausdorff): Batch up failures and report them all at once during preview. This
			// will cause them to fail eagerly.
			if d.EnforcementLevel == apitype.Mandatory {
				if !sg.plan.preview {
					invalid = true
				}
				sg.locked(func() {
					sg.hasPolicyViolations = true
				})
			}
			sg.opts.Events.OnPolicyViolation(new.URN, d)
		}
	}

	// If the resource isn't valid, don't proceed any further.
	if invalid {
		return nil, result.Bail()
	}

	// There are four cases we need to consider when figuring out what to do with this resource.
	//
	// Case 1: recreating
	//  In this case, we have seen a resource with this URN before and we have already issued a
	//  delete step for it. This happens when the engine has to delete a resource before it has
	//  enough information about whether that resource still exists. A concrete example is
	//  when a resource depends on a resource that is delete-before-replace: the engine must first
	//  delete the dependent resource before depending the DBR resource, but the engine can't know
	//  yet whether the dependent resource is being replaced or deleted.
	//
	//  In this case, we are seeing the resource again after deleting it, so it must be a replacement.
	//
	//  Logically, recreating implies hasOld, since in order to delete something it must have
	//  already existed.
	contract.Assert(!recreating || hasOld)
	if recreating {
		logging.V(7).Infof("Planner decided to re-create replaced resource '%v' deleted due to dependent DBR", urn)

		// Unmark this resource as deleted, we now know it's being replaced instead.
		var keys []resource.PropertyKey
		sg.locked(func() {
			delete(sg.deletes, urn)
			sg.replaces[urn] = true
			keys = sg.dependentReplaceKeys[urn]
		})
		return []Step{
			NewReplaceStep(sg.plan, old, new, nil, nil, nil, false),
			NewCreateReplacementStep(sg.plan, event, old, new, keys, nil, nil, false),
		}, nil
	}

	// Case 2: wasExternal
	//  In this case, the resource we are operating upon exists in the old snapshot, but it
	//  was "external" - Pulumi does not own its lifecycle. Conceptually, this operation is
	//  akin to "taking ownership" of a resource that we did not previously control.
	//
	//  Since we are not allowed to manipulate the existing resource, we must create a resource
	//  to take its place. Since this is technically a replacement operation, we pend deletion of
	//  read until the end of the plan.
	if wasExternal {
		logging.V(7).Infof("Planner recognized '%s' as old external resource, creating instead", urn)
		sg.locked(func() {
			sg.creates[urn] = true
		})
		return []Step{
			NewCreateReplacementStep(sg.plan, event, old, new, nil, nil, nil, true),
			NewReplaceStep(sg.plan, old, new, nil, nil, nil, true),
		}, nil
	}

	// Case 3: hasOld
	//  In this case, the resource we are operating upon now exists in the old snapshot.
	//  It must be an update or a replace. Which operation we do depends on the the specific change made to the
	//  resource's properties:
	//
	//  - if the user has requested that only specific resources be updated, and this resource is
	//    not in that set, do no 'Diff' and just treat the resource as 'same' (i.e. unchanged).
	//
	//  - If the resource's provider reference changed, the resource must be replaced. This behavior is founded upon
	//    the assumption that providers are recreated iff their configuration changed in such a way that they are no
	//    longer able to manage existing resources.
	//
	//  - Otherwise, we invoke the resource's provider's `Diff` method. If this method indicates that the resource must
	//    be replaced, we do so. If it does not, we update the resource in place.
	if hasOld {
		contract.Assert(old != nil)

		// If the user requested only specific resources to update, and this resource was not in
		// that set, then do nothin but create a SameStep for it.
		if updateTargetsOpt != nil && !updateTargetsOpt[urn] {
			logging.V(7).Infof(
				"Planner decided not to update '%v' due to not being in target group (same) (inputs=%v)", urn, new.Inputs)
		} else {
			updateSteps, res := sg.generateStepsFromDiff(
				event, urn, old, new, oldInputs, oldOutputs, inputs, prov, goal)

			if res != nil {
				return nil, res
			}

			if len(updateSteps) > 0 {
				// 'Diff' produced update steps.  We're done at this point.
				return updateSteps, nil
			}

			// Diff didn't produce any steps for this resource.  Fall through and indicate that it
			// is same/unchanged.
			logging.V(7).Infof("Planner decided not to update '%v' after diff (same) (inputs=%v)", urn, new.Inputs)
		}

		// No need to update anything, the properties didn't change.
		sg.locked(func() {
			sg.sames[urn] = true
		})
		return []Step{NewSameStep(sg.plan, event, old, new)}, nil
	}

	if updateTargetsOpt != nil && !updateTargetsOpt[urn] {
		d := diag.GetResourceIsBeingCreatedButWasNotSpecifiedInTargetList(urn)

		if sg.plan.preview {
			// During preview we only warn here but let the planning proceed.  This allows the user
			// to hear about *all* the potential resources they'd need to add a -target arg for.
			// This prevents the annoying scenario where you get notified about a problem, fix it,
			// rerun and then get notified about the very next problem.
			sg.plan.Diag().Warningf(d, urn)
		} else {
			// Targets were specified, but didn't include this resource to create.  Give a particular
			// error in that case and stop immediately.
			sg.plan.Diag().Errorf(d, urn)
			return nil, result.Bail()
		}
	}

	// Case 4: Not Case 1, 2, or 3
	//  If a resource isn't being recreated and it's not being updated or replaced,
	//  it's just being created.
	sg.locked(func() {
		sg.creates[urn] = true
	})
	logging.V(7).Infof("Planner decided to create '%v' (inputs=%v)", urn, new.Inputs)
	return []Step{NewCreateStep(sg.plan, event, new)}, nil
}

func (sg *stepGenerator) generateStepsFromDiff(
	event RegisterResourceEvent, urn resource.URN, old, new *resource.State,
	oldInputs, oldOutputs, inputs resource.PropertyMap,
	prov plugin.Provider, goal *resource.Goal) ([]Step, result.Result) {

	// We only allow unknown property values to be exposed to the provider if we are performing an update preview.
	allowUnknowns := sg.plan.preview

	diff, err := sg.diff(urn, old, new, oldInputs, oldOutputs, inputs, prov, allowUnknowns, goal.IgnoreChanges)
	// If the plugin indicated that the diff is unavailable, assume that the resource will be updated and
	// report the message contained in the error.
	if _, ok := err.(plugin.DiffUnavailableError); ok {
		diff = plugin.DiffResult{Changes: plugin.DiffSome}
		sg.plan.ctx.Diag.Warningf(diag.RawMessage(urn, err.Error()))
	} else if err != nil {
		return nil, result.FromError(err)
	}

	// Ensure that we received a sensible response.
	if diff.Changes != plugin.DiffNone && diff.Changes != plugin.DiffSome {
		return nil, result.Errorf(
			"unrecognized diff state for %s: %d", urn, diff.Changes)
	}

	// If there were changes, check for a replacement vs. an in-place update.
	if diff.Changes == plugin.DiffSome {
		if diff.Replace() {
			// If the goal state specified an ID, issue an error: the replacement will change the ID, and is
			// therefore incompatible with the goal state.
			if goal.ID != "" {
				const message = "previously-imported resources that still specify an ID may not be replaced; " +
					"please remove the `import` declaration from your program"
				if sg.plan.preview {
					sg.plan.ctx.Diag.Warningf(diag.StreamMessage(urn, message, 0))
				} else {
					return nil, result.Errorf(message)
				}
			}

			sg.locked(func() {
				sg.replaces[urn] = true
			})

			// If we are going to perform a replacement, we need to recompute the default values.  The above logic
			// had assumed that we were going to carry them over from the old resource, which is no longer true.
			if prov != nil {
				var failures []plugin.CheckFailure
				inputs, failures, err = prov.Check(urn, nil, goal.Properties, allowUnknowns)
				if err != nil {
					return nil, result.FromError(err)
				} else if issueCheckErrors(sg.plan, new, urn, failures) {
					return nil, result.Bail()
				}
				new.Inputs = inputs
			}

			if logging.V(7) {
				logging.V(7).Infof("Planner decided to replace '%v' (oldprops=%v inputs=%v)",
					urn, oldInputs, new.Inputs)
			}

			// We have two approaches to performing replacements:
			//
			//     * CreateBeforeDelete: the default mode first creates a new instance of the resource, then
			//       updates all dependent resources to point to the new one, and finally after all of that,
			//       deletes the old resource.  This ensures minimal downtime.
			//
			//     * DeleteBeforeCreate: this mode can be used for resources that cannot be tolerate having
			//       side-by-side old and new instances alive at once.  This first deletes the resource and
			//       then creates the new one.  This may result in downtime, so is less preferred.  Note that
			//       until pulumi/pulumi#624 is resolved, we cannot safely perform this operation on resources
			//       that have dependent resources (we try to delete the resource while they refer to it).
			//
			// The provider is responsible for requesting which of these two modes to use. The user can override
			// the provider's decision by setting the `deleteBeforeReplace` field of `ResourceOptions` to either
			// `true` or `false`.
			deleteBeforeReplace := diff.DeleteBeforeReplace
			if goal.DeleteBeforeReplace != nil {
				deleteBeforeReplace = *goal.DeleteBeforeReplace
			}
			if deleteBeforeReplace {
				logging.V(7).Infof("Planner decided to delete-before-replacement for resource '%v'", urn)
				contract.Assert(sg.plan.depGraph != nil)

				// DeleteBeforeCreate implies that we must immediately delete the resource. For correctness,
				// we must also eagerly delete all resources that depend directly or indirectly on the resource
				// being replaced and would be replaced by a change to the relevant dependency.
				//
				// To do this, we'll utilize the dependency information contained in the snapshot if it is
				// trustworthy, which is interpreted by the DependencyGraph type.
				var steps []Step
				if sg.opts.TrustDependencies {
					toReplace, res := sg.calculateDependentReplacements(old)
					if res != nil {
						return nil, res
					}
					defer sg.unlockDependentReplaces(toReplace)

					// Deletions must occur in reverse dependency order, and `deps` is returned in dependency
					// order, so we iterate in reverse.
					sg.locked(func() {
						for i := len(toReplace) - 1; i >= 0; i-- {
							dependentResource := toReplace[i].res

							// If we already deleted this resource due to some other DBR, don't do it again.
							if sg.deletes[dependentResource.URN] {
								continue
							}

							sg.dependentReplaceKeys[dependentResource.URN] = toReplace[i].keys

							logging.V(7).Infof("Planner decided to delete '%v' due to dependence on condemned resource '%v'",
								dependentResource.URN, urn)

							steps = append(steps, NewDeleteReplacementStep(sg.plan, dependentResource, true))
							// Mark the condemned resource as deleted. We won't know until later in the plan whether
							// or not we're going to be replacing this resource.
							sg.deletes[dependentResource.URN] = true
						}
					})
				}

				return append(steps,
					NewDeleteReplacementStep(sg.plan, old, true),
					NewReplaceStep(sg.plan, old, new, diff.ReplaceKeys, diff.ChangedKeys, diff.DetailedDiff, false),
					NewCreateReplacementStep(
						sg.plan, event, old, new, diff.ReplaceKeys, diff.ChangedKeys, diff.DetailedDiff, false),
				), nil
			}

			return []Step{
				NewCreateReplacementStep(
					sg.plan, event, old, new, diff.ReplaceKeys, diff.ChangedKeys, diff.DetailedDiff, true),
				NewReplaceStep(sg.plan, old, new, diff.ReplaceKeys, diff.ChangedKeys, diff.DetailedDiff, true),
				// note that the delete step is generated "later" on, after all creates/updates finish.
			}, nil
		}

		// If we fell through, it's an update.
		sg.locked(func() {
			sg.updates[urn] = true
		})
		if logging.V(7) {
			logging.V(7).Infof("Planner decided to update '%v' (oldprops=%v inputs=%v", urn, oldInputs, new.Inputs)
		}
		return []Step{
			NewUpdateStep(sg.plan, event, old, new, diff.StableKeys, diff.ChangedKeys, diff.DetailedDiff,
				goal.IgnoreChanges),
		}, nil
	}

	// If resource was unchanged, but there were initialization errors, generate an empty update
	// step to attempt to "continue" awaiting initialization.
	if len(old.InitErrors) > 0 {
		sg.locked(func() {
			sg.updates[urn] = true
		})
		return []Step{NewUpdateStep(sg.plan, event, old, new, diff.StableKeys, nil, nil, nil)}, nil
	}

	return nil, nil
}

func (sg *stepGenerator) GenerateDeletes(targetsOpt map[resource.URN]bool) ([]Step, result.Result) {
	// To compute the deletion list, we must walk the list of old resources *backwards*.  This is because the list is
	// stored in dependency order, and earlier elements are possibly leaf nodes for later elements.  We must not delete
	// dependencies prior to their dependent nodes.
	var dels []Step
	if prev := sg.plan.prev; prev != nil {
		for i := len(prev.Resources) - 1; i >= 0; i-- {
			// If this resource is explicitly marked for deletion or wasn't seen at all, delete it.
			res := prev.Resources[i]
			if res.Delete {
				// The below assert is commented-out because it's believed to be wrong.
				//
				// The original justification for this assert is that the author (swgillespie) believed that
				// it was impossible for a single URN to be deleted multiple times in the same program.
				// This has empirically been proven to be false - it is possible using today engine to construct
				// a series of actions that puts arbitrarily many pending delete resources with the same URN in
				// the snapshot.
				//
				// It is not clear whether or not this is OK. I (swgillespie), the author of this comment, have
				// seen no evidence that it is *not* OK. However, concerns were raised about what this means for
				// structural resources, and so until that question is answered, I am leaving this comment and
				// assert in the code.
				//
				// Regardless, it is better to admit strange behavior in corner cases than it is to crash the CLI
				// whenever we see multiple deletes for the same URN.
				// contract.Assert(!sg.deletes[res.URN])
				if sg.pendingDeletes[res] {
					logging.V(7).Infof(
						"Planner ignoring pending-delete resource (%v, %v) that was already deleted", res.URN, res.ID)
					continue
				}

				if sg.deletes[res.URN] {
					logging.V(7).Infof(
						"Planner is deleting pending-delete urn '%v' that has already been deleted", res.URN)
				}

				logging.V(7).Infof("Planner decided to delete '%v' due to replacement", res.URN)
				sg.deletes[res.URN] = true
				dels = append(dels, NewDeleteReplacementStep(sg.plan, res, false))
			} else if _, aliased := sg.aliased[res.URN]; !sg.sames[res.URN] && !sg.updates[res.URN] && !sg.replaces[res.URN] &&
				!sg.reads[res.URN] && !aliased {
				// NOTE: we deliberately do not check sg.deletes here, as it is possible for us to issue multiple
				// delete steps for the same URN if the old checkpoint contained pending deletes.
				logging.V(7).Infof("Planner decided to delete '%v'", res.URN)
				sg.deletes[res.URN] = true
				if !res.PendingReplacement {
					dels = append(dels, NewDeleteStep(sg.plan, res))
				} else {
					dels = append(dels, NewRemovePendingReplaceStep(sg.plan, res))
				}
			}
		}
	}

	// If -target was provided to either `pulumi update` or `pulumi destroy` then only delete
	// resources that were specified.
	allowedResourcesToDelete, res := sg.determineAllowedResourcesToDeleteFromTargets(targetsOpt)
	if res != nil {
		return nil, res
	}

	if allowedResourcesToDelete != nil {
		filtered := []Step{}
		for _, step := range dels {
			if _, has := allowedResourcesToDelete[step.URN()]; has {
				filtered = append(filtered, step)
			}
		}

		dels = filtered
	}

	return dels, nil
}

func (sg *stepGenerator) determineAllowedResourcesToDeleteFromTargets(
	targetsOpt map[resource.URN]bool) (map[resource.URN]bool, result.Result) {

	if targetsOpt == nil {
		// no specific targets, so we won't filter down anything
		return nil, nil
	}

	logging.V(7).Infof("Planner was asked to only delete/update '%v'", targetsOpt)
	resourcesToDelete := make(map[resource.URN]bool)

	// Now actually use all the requested targets to figure out the exact set to delete.
	for target := range targetsOpt {
		current := sg.plan.olds[target]
		resourcesToDelete[target] = true

		// the item the user is asking to destroy may cause downstream replacements.  Clean those up
		// as well. Use the standard delete-before-replace computation to determine the minimal
		// set of downstream resources that are affected.
		deps, res := sg.calculateDependentReplacements(current)
		if res != nil {
			return nil, res
		}

		for _, dep := range deps {
			logging.V(7).Infof("GenerateDeletes(...): Adding dependent: %v", dep.res.URN)
			resourcesToDelete[dep.res.URN] = true
		}

		sg.unlockDependentReplaces(deps)
	}

	// Also see if any resources have a resource we're deleting as a parent. If so, we'll block
	// the delete.  It's a little painful.  But can be worked around by explicitly deleting
	// children before parents.  Note: in almost all cases, people will want to delete children,
	// so this restriction should not be too onerous.
	for _, res := range sg.plan.prev.Resources {
		if res.Parent != "" {
			if _, has := resourcesToDelete[res.URN]; has {
				// already deleting this sibling
				continue
			}

			if _, has := resourcesToDelete[res.Parent]; has {
				sg.plan.Diag().Errorf(diag.GetCannotDeleteParentResourceWithoutAlsoDeletingChildError(res.Parent),
					res.Parent, res.URN)
				return nil, result.Bail()
			}
		}
	}

	if logging.V(7) {
		keys := []resource.URN{}
		for k := range resourcesToDelete {
			keys = append(keys, k)
		}

		logging.V(7).Infof("Planner will delete all of '%v'", keys)
	}

	return resourcesToDelete, nil
}

// GeneratePendingDeletes generates delete steps for all resources that are pending deletion. This function should be
// called at the start of a plan in order to find all resources that are pending deletion from the prevous plan.
func (sg *stepGenerator) GeneratePendingDeletes() []Step {
	var dels []Step
	if prev := sg.plan.prev; prev != nil {
		logging.V(7).Infof("stepGenerator.GeneratePendingDeletes(): scanning previous snapshot for pending deletes")
		for i := len(prev.Resources) - 1; i >= 0; i-- {
			res := prev.Resources[i]
			if res.Delete {
				logging.V(7).Infof(
					"stepGenerator.GeneratePendingDeletes(): resource (%v, %v) is pending deletion", res.URN, res.ID)
				sg.pendingDeletes[res] = true
				dels = append(dels, NewDeleteStep(sg.plan, res))
			}
		}
	}
	return dels
}

// scheduleDeletes takes a list of steps that will delete resources and "schedules" them by producing a list of list of
// steps, where each list can be executed in parallel but a previous list must be executed to completion before advacing
// to the next list.
//
// In lieu of tracking per-step dependencies and orienting the step executor around these dependencies, this function
// provides a conservative approximation of what deletions can safely occur in parallel. The insight here is that the
// resource dependency graph is a partially-ordered set and all partially-ordered sets can be easily decomposed into
// antichains - subsets of the set that are all not comparable to one another. (In this definition, "not comparable"
// means "do not depend on one another").
//
// The algorithm for decomposing a poset into antichains is:
//  1. While there exist elements in the poset,
//    1a. There must exist at least one "maximal" element of the poset. Let E_max be those elements.
//    2a. Remove all elements E_max from the poset. E_max is an antichain.
//    3a. Goto 1.
//
// Translated to our dependency graph:
//  1. While the set of condemned resources is not empty:
//    1a. Remove all resources with no outgoing edges from the graph and add them to the current antichain.
//    2a. Goto 1.
//
// The resulting list of antichains is a list of list of steps that can be safely executed in parallel. Since we must
// process deletes in reverse (so we don't delete resources upon which other resources depend), we reverse the list and
// hand it back to the plan executor for safe execution.
func (sg *stepGenerator) ScheduleDeletes(deleteSteps []Step) []antichain {
	var antichains []antichain                // the list of parallelizable steps we intend to return.
	dg := sg.plan.depGraph                    // the current plan's dependency graph.
	condemned := make(graph.ResourceSet)      // the set of condemned resources.
	stepMap := make(map[*resource.State]Step) // a map from resource states to the steps that delete them.

	// If we don't trust the dependency graph we've been given, we must be conservative and delete everything serially.
	if !sg.opts.TrustDependencies {
		logging.V(7).Infof("Planner does not trust dependency graph, scheduling deletions serially")
		for _, step := range deleteSteps {
			antichains = append(antichains, antichain{step})
		}

		return antichains
	}

	logging.V(7).Infof("Planner trusts dependency graph, scheduling deletions in parallel")

	// For every step we've been given, record it as condemned and save the step that will be used to delete it. We'll
	// iteratively place these steps into antichains as we remove elements from the condemned set.
	for _, step := range deleteSteps {
		condemned[step.Res()] = true
		stepMap[step.Res()] = step
	}

	for len(condemned) > 0 {
		var steps antichain
		logging.V(7).Infof("Planner beginning schedule of new deletion antichain")
		for res := range condemned {
			// Does res have any outgoing edges to resources that haven't already been removed from the graph?
			condemnedDependencies := dg.DependenciesOf(res).Intersect(condemned)
			if len(condemnedDependencies) == 0 {
				// If not, it's safe to delete res at this stage.
				logging.V(7).Infof("Planner scheduling deletion of '%v'", res.URN)
				steps = append(steps, stepMap[res])
			}

			// If one of this resource's dependencies or this resource's parent hasn't been removed from the graph yet,
			// it can't be deleted this round.
		}

		// For all reosurces that are to be deleted in this round, remove them from the graph.
		for _, step := range steps {
			delete(condemned, step.Res())
		}

		antichains = append(antichains, steps)
	}

	// Up until this point, all logic has been "backwards" - we're scheduling resources for deletion when all of their
	// dependencies finish deletion, but that's exactly the opposite of what we need to do. We can only delete a
	// resource when all *resources that depend on it* complete deletion. Our solution is still correct, though, it's
	// just backwards.
	//
	// All we have to do here is reverse the list and then our solution is correct.
	for i := len(antichains)/2 - 1; i >= 0; i-- {
		opp := len(antichains) - 1 - i
		antichains[i], antichains[opp] = antichains[opp], antichains[i]
	}

	return antichains
}

// providerChanged diffs the Provider field of old and new resources, returning true if the rest of the step generator
// should consider there to be a diff between these two resources.
func (sg *stepGenerator) providerChanged(urn resource.URN, old, new *resource.State) (bool, error) {
	// If a resource's Provider field has changed, we may need to show a diff and we may not. This is subtle. See
	// pulumi/pulumi#2753 for more details.
	//
	// Recent versions of Pulumi allow for language hosts to pass a plugin version to the engine. The purpose of this is
	// to ensure that the plugin that the engine uses for a particular resource is *exactly equal* to the version of the
	// SDK that the language host used to produce the resource registration. This is critical for correct versioning
	// semantics; it is generally an error for a language SDK to produce a registration that is serviced by a
	// differently versioned plugin, since the two version in complete lockstep and there is no guarantee that the two
	// will work correctly together when not the same version.

	if old.Provider == new.Provider {
		return false, nil
	}

	logging.V(stepExecutorLogLevel).Infof("sg.diffProvider(%s, ...): observed provider diff", urn)
	logging.V(stepExecutorLogLevel).Infof("sg.diffProvider(%s, ...): %s => %s", urn, old.Provider, new.Provider)

	oldRef, err := providers.ParseReference(old.Provider)
	if err != nil {
		return false, err
	}
	newRef, err := providers.ParseReference(new.Provider)
	if err != nil {
		return false, err
	}

	// If one or both of these providers are not default providers, we will need to accept the diff and replace
	// everything. This might not be strictly necessary, but it is conservatively correct.
	if !providers.IsDefaultProvider(oldRef.URN()) || !providers.IsDefaultProvider(newRef.URN()) {
		logging.V(stepExecutorLogLevel).Infof(
			"sg.diffProvider(%s, ...): reporting provider diff due to change in default provider status", urn)
		logging.V(stepExecutorLogLevel).Infof(
			"sg.diffProvider(%s, ...): old provider %q is default: %v",
			urn, oldRef.URN(), providers.IsDefaultProvider(oldRef.URN()))
		logging.V(stepExecutorLogLevel).Infof(
			"sg.diffProvider(%s, ...): new provider %q is default: %v",
			urn, newRef.URN(), providers.IsDefaultProvider(newRef.URN()))
		return true, err
	}

	// If both of these providers are default providers, use the *new provider* to diff the config and determine if
	// this provider requires replacement.
	//
	// Note that, if we have many resources managed by the same provider that is getting replaced in this manner,
	// this will call DiffConfig repeatedly with the same arguments for every resource. If this becomes a
	// performance problem, this result can be cached.
	newProv, ok := sg.plan.providers.GetProvider(newRef)
	if !ok {
		return false, errors.Errorf("failed to resolve provider reference: %q", oldRef.String())
	}

	oldRes, ok := sg.plan.olds[oldRef.URN()]
	contract.Assertf(ok, "old state didn't have provider, despite resource using it?")

	var newRes *resource.State
	sg.rlocked(func() {
		newRes, ok = sg.providers[newRef.URN()]
	})
	contract.Assertf(ok, "new plan didn't have provider, despite resource using it?")

	diff, err := newProv.DiffConfig(newRef.URN(), oldRes.Inputs, newRes.Inputs, true, nil)
	if err != nil {
		return false, err
	}

	// If there is a replacement diff, we must also replace this resource.
	if diff.Replace() {
		logging.V(stepExecutorLogLevel).Infof(
			"sg.diffProvider(%s, ...): new provider's DiffConfig reported replacement", urn)
		return true, nil
	}

	// Otherwise, it's safe to allow this new provider to replace our old one.
	logging.V(stepExecutorLogLevel).Infof(
		"sg.diffProvider(%s, ...): both providers are default, proceeding with resource diff", urn)
	return false, nil
}

// diff returns a DiffResult for the given resource.
func (sg *stepGenerator) diff(urn resource.URN, old, new *resource.State, oldInputs, oldOutputs,
	newInputs resource.PropertyMap, prov plugin.Provider, allowUnknowns bool,
	ignoreChanges []string) (plugin.DiffResult, error) {

	// Before diffing the resource, diff the provider field. If the provider field changes, we may or may
	// not need to replace the resource.
	providerChanged, err := sg.providerChanged(urn, old, new)
	if err != nil {
		return plugin.DiffResult{}, err
	} else if providerChanged {
		return plugin.DiffResult{Changes: plugin.DiffSome, ReplaceKeys: []resource.PropertyKey{"provider"}}, nil
	}

	// Apply legacy diffing behavior if requested. In this mode, if the provider-calculated inputs for a resource did
	// not change, then the resource is considered to have no diff between its desired and actual state.
	if sg.opts.UseLegacyDiff && oldInputs.DeepEquals(newInputs) {
		return plugin.DiffResult{Changes: plugin.DiffNone}, nil
	}

	// If there is no provider for this resource (which should only happen for component resources), simply return a
	// "diffs exist" result.
	if prov == nil {
		if oldInputs.DeepEquals(newInputs) {
			return plugin.DiffResult{Changes: plugin.DiffNone}, nil
		}
		return plugin.DiffResult{Changes: plugin.DiffSome}, nil
	}

	return diffResource(urn, old.ID, oldInputs, oldOutputs, newInputs, prov, allowUnknowns, ignoreChanges)
}

// diffResource invokes the Diff function for the given custom resource's provider and returns the result.
func diffResource(urn resource.URN, id resource.ID, oldInputs, oldOutputs,
	newInputs resource.PropertyMap, prov plugin.Provider, allowUnknowns bool,
	ignoreChanges []string) (plugin.DiffResult, error) {

	contract.Require(prov != nil, "prov != nil")

	// Grab the diff from the provider. At this point we know that there were changes to the Pulumi inputs, so if the
	// provider returns an "unknown" diff result, pretend it returned "diffs exist".
	diff, err := prov.Diff(urn, id, oldOutputs, newInputs, allowUnknowns, ignoreChanges)
	if err != nil {
		return diff, err
	}
	if diff.Changes == plugin.DiffUnknown {
		if oldInputs.DeepEquals(newInputs) {
			diff.Changes = plugin.DiffNone
		} else {
			diff.Changes = plugin.DiffSome
		}
	}
	return diff, nil
}

// issueCheckErrors prints any check errors to the diagnostics sink.
func issueCheckErrors(plan *Plan, new *resource.State, urn resource.URN, failures []plugin.CheckFailure) bool {
	if len(failures) == 0 {
		return false
	}
	inputs := new.Inputs
	for _, failure := range failures {
		if failure.Property != "" {
			plan.Diag().Errorf(diag.GetResourcePropertyInvalidValueError(urn),
				new.Type, urn.Name(), failure.Property, inputs[failure.Property], failure.Reason)
		} else {
			plan.Diag().Errorf(
				diag.GetResourceInvalidError(urn), new.Type, urn.Name(), failure.Reason)
		}
	}
	return true
}

// processIgnoreChanges sets the value for each ignoreChanges property in inputs to the value from oldInputs.  This has
// the effect of ensuring that no changes will be made for the corresponding property.
func processIgnoreChanges(inputs, oldInputs resource.PropertyMap,
	ignoreChanges []string) (resource.PropertyMap, result.Result) {

	ignoredInputs := resource.NewObjectProperty(inputs.Copy())
	var invalidPaths []string
	for _, ignoreChange := range ignoreChanges {
		path, err := resource.ParsePropertyPath(ignoreChange)
		if err != nil {
			continue
		}

		oldValue, hasOld := path.Get(resource.NewObjectProperty(oldInputs))
		_, hasNew := path.Get(resource.NewObjectProperty(inputs))

		var ok bool
		switch {
		case hasOld && hasNew:
			ok = path.Set(ignoredInputs, oldValue)
			contract.Assert(ok)
		case hasOld && !hasNew:
			ok = path.Set(ignoredInputs, oldValue)
		case !hasOld && hasNew:
			ok = path.Delete(ignoredInputs)
		default:
			ok = true
		}
		if !ok {
			invalidPaths = append(invalidPaths, ignoreChange)
		}
	}
	if len(invalidPaths) != 0 {
		return nil, result.Errorf("cannot ignore changes to the following properties because one or more elements of "+
			"the path are missing: %q", strings.Join(invalidPaths, ", "))
	}
	return ignoredInputs.ObjectValue(), nil
}

func (sg *stepGenerator) loadResourceProvider(
	urn resource.URN, custom bool, provider string, typ tokens.Type) (plugin.Provider, result.Result) {

	// If this is not a custom resource, then it has no provider by definition.
	if !custom {
		return nil, nil
	}

	// If this resource is a provider resource, use the plan's provider registry for its CRUD operations.
	// Otherwise, resolve the the resource's provider reference.
	if providers.IsProviderType(typ) {
		return sg.plan.providers, nil
	}

	contract.Assert(provider != "")
	ref, refErr := providers.ParseReference(provider)
	if refErr != nil {
		sg.plan.Diag().Errorf(diag.GetBadProviderError(urn), provider, urn, refErr)
		return nil, result.Bail()
	}
	p, ok := sg.plan.GetProvider(ref)
	if !ok {
		sg.plan.Diag().Errorf(diag.GetUnknownProviderError(urn), provider, urn, refErr)
		return nil, result.Bail()
	}
	return p, nil
}

type dependentReplace struct {
	res  *resource.State
	keys []resource.PropertyKey
}

func (sg *stepGenerator) calculateDependentReplacements(root *resource.State) ([]dependentReplace, result.Result) {
	// We need to compute the set of resources that may be replaced by a change to the resource
	// under consideration. We do this by taking the complete set of transitive dependents on the
	// resource under consideration and removing any resources that would not be replaced by changes
	// to their dependencies. We determine whether or not a resource may be replaced by substituting
	// unknowns for input properties that may change due to deletion of the resources their value
	// depends on and calling the resource provider's `Diff` method.
	//
	// This is perhaps clearer when described by example. Consider the following dependency graph:
	//
	//       A
	//     __|__
	//     B   C
	//     |  _|_
	//     D  E F
	//
	// In this graph, all of B, C, D, E, and F transitively depend on A. It may be the case,
	// however, that changes to the specific properties of any of those resources R that would occur
	// if a resource on the path to A were deleted and recreated may not cause R to be replaced. For
	// example, the edge from B to A may be a simple `dependsOn` edge such that a change to B does
	// not actually influence any of B's input properties.  More commonly, the edge from B to A may
	// be due to a property from A being used as the input to a property of B that does not require
	// B to be replaced upon a change. In these cases, neither B nor D would need to be deleted
	// before A could be deleted.
	var toReplace []dependentReplace
	replaceSet := map[resource.URN]bool{root.URN: true}

	requiresReplacement := func(r *resource.State) (bool, []resource.PropertyKey, result.Result) {
		// Neither component nor external resources require replacement.
		if !r.Custom || r.External {
			return false, nil, nil
		}

		// If the resource's provider is in the replace set, we mustreplace this resource.
		if r.Provider != "" {
			ref, err := providers.ParseReference(r.Provider)
			if err != nil {
				return false, nil, result.FromError(err)
			}
			if replaceSet[ref.URN()] {
				return true, nil, nil
			}
		}

		// Scan the properties of this resource in order to determine whether or not any of them depend on a resource
		// that requires replacement and build a set of input properties for the provider diff.
		hasDependencyInReplaceSet, inputsForDiff := false, resource.PropertyMap{}
		for pk, pv := range r.Inputs {
			for _, propertyDep := range r.PropertyDependencies[pk] {
				if replaceSet[propertyDep] {
					hasDependencyInReplaceSet = true
					pv = resource.MakeComputed(resource.NewStringProperty("<unknown>"))
				}
			}
			inputsForDiff[pk] = pv
		}

		// If none of this resource's properties depend on a resource in the replace set, then none of the properties
		// may change and this resource does not need to be replaced.
		if !hasDependencyInReplaceSet {
			return false, nil, nil
		}

		// Otherwise, fetch the resource's provider. Since we have filtered out component resources, this resource must
		// have a provider.
		prov, res := sg.loadResourceProvider(r.URN, r.Custom, r.Provider, r.Type)
		if res != nil {
			return false, nil, res
		}
		contract.Assert(prov != nil)

		// Call the provider's `Diff` method and return.
		diff, err := prov.Diff(r.URN, r.ID, r.Outputs, inputsForDiff, true, nil)
		if err != nil {
			return false, nil, result.FromError(err)
		}
		return diff.Replace(), diff.ReplaceKeys, nil
	}

	// Walk the root resource's dependents in order and build up the set of resources that require replacement.
	//
	// NOTE: the dependency graph we use for this calculation is based on the dependency graph from the last snapshot.
	// If there are resources in this graph that used to depend on the root but have been re-registered such that they
	// no longer depend on the root, we may make incorrect decisions. To avoid that, we rely on the observation that
	// dependents can only have been _removed_ from the base dependency graph: for a dependent to have been added,
	// it would have had to have been registered prior to the root, which is not a valid operation. This means that
	// any resources that depend on the root must not yet have been registered, which in turn implies that resources
	// that have already been registered or are in the process of being registered must not depend on the root. Thus,
	// we ignore these resources if they are encountered while walking the old dependency graph to determine the set
	// of dependents.
	//
	// In addition to calculating the set of resources we may need to delete before deleting the root, we need to lock
	// all of these resources in order to prevent concurrent resource registrations or concurrent DBR calculations from
	// planning different operations. We will unlock any resources we do not plan on deleting prior to returning from
	// this function. It is up to the caller to unlock the set of dependent replaces this function returns.
	dependents := func() []*resource.State {
		sg.resourceCond.L.Lock()
		defer sg.resourceCond.L.Unlock()

		for {
			// Calculate the set of dependents and attempt to lock them. If we succeed at locking all of the
			// dependents, return. Otherwise, wait on the condition variable and loop.
			dependents := sg.plan.depGraph.DependingOn(root, sg.urns)
			if sg.tryLockResourcesLocked(dependents) {
				return dependents
			}
			sg.resourceCond.Wait()
		}
	}()

	for i, d := range dependents {
		replace, keys, res := requiresReplacement(d)
		if res != nil {
			sg.unlockDependentReplaces(toReplace)
			sg.unlockResources(dependents[i:])
			return nil, res
		}
		if replace {
			toReplace, replaceSet[d.URN] = append(toReplace, dependentReplace{res: d, keys: keys}), true
		} else {
			sg.unlockResource(d.URN)
		}
	}

	// Return the list of resources to replace.
	return toReplace, nil
}

// newStepGenerator creates a new step generator that operates on the given plan.
func newStepGenerator(plan *Plan, opts Options) *stepGenerator {
	sg := &stepGenerator{
		plan:                 plan,
		opts:                 opts,
		resourceLocks:        make(map[resource.URN]bool),
		urns:                 make(map[resource.URN]bool),
		reads:                make(map[resource.URN]bool),
		creates:              make(map[resource.URN]bool),
		sames:                make(map[resource.URN]bool),
		replaces:             make(map[resource.URN]bool),
		updates:              make(map[resource.URN]bool),
		deletes:              make(map[resource.URN]bool),
		pendingDeletes:       make(map[*resource.State]bool),
		providers:            make(map[resource.URN]*resource.State),
		dependentReplaceKeys: make(map[resource.URN][]resource.PropertyKey),
		aliased:              make(map[resource.URN]resource.URN),
	}
	sg.resourceCond.L = &sg.mutex
	return sg
}
