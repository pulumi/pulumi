// Copyright 2016-2021, Pulumi Corporation.
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
	cryptorand "crypto/rand"
	"fmt"
	"strings"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/pkg/v3/resource/graph"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
)

// stepGenerator is responsible for turning resource events into steps that can be fed to the deployment executor.
// It does this by consulting the deployment and calculating the appropriate step action based on the requested goal
// state and the existing state of the world.
type stepGenerator struct {
	deployment *Deployment // the deployment to which this step generator belongs
	opts       Options     // options for this step generator

	// signals that one or more errors have been reported to the user, and the deployment should terminate
	// in error. This primarily allows `preview` to aggregate many policy violation events and
	// report them all at once.
	sawError bool

	urns     map[resource.URN]bool // set of URNs discovered for this deployment
	reads    map[resource.URN]bool // set of URNs read for this deployment
	deletes  map[resource.URN]bool // set of URNs deleted in this deployment
	replaces map[resource.URN]bool // set of URNs replaced in this deployment
	updates  map[resource.URN]bool // set of URNs updated in this deployment
	creates  map[resource.URN]bool // set of URNs created in this deployment
	sames    map[resource.URN]bool // set of URNs that were not changed in this deployment

	// set of URNs that would have been created, but were filtered out because the user didn't
	// specify them with --target
	skippedCreates map[resource.URN]bool

	pendingDeletes map[*resource.State]bool         // set of resources (not URNs!) that are pending deletion
	providers      map[resource.URN]*resource.State // URN map of providers that we have seen so far.

	// a map from URN to a list of property keys that caused the replacement of a dependent resource during a
	// delete-before-replace.
	dependentReplaceKeys map[resource.URN][]resource.PropertyKey

	// a map from old names (aliased URNs) to the new URN that aliased to them.
	aliased map[resource.URN]resource.URN
	// a map from current URN of the resource to the old URN that it was aliased from.
	aliases map[resource.URN]resource.URN

	// targetsActual is the set of targets explicitly targeted by the engine, this can be different from opts.targets if
	// --target-dependents is true. This does _not_ include resources that have been implicitly targeted, like providers.
	targetsActual UrnTargets
}

// isTargetedForUpdate returns if `res` is targeted for update. The function accommodates
// `--target-dependents`.
func (sg *stepGenerator) isTargetedForUpdate(res *resource.State) bool {
	if sg.opts.Targets.Contains(res.URN) {
		return true
	} else if !sg.opts.TargetDependents {
		return false
	}

	if ref := res.Provider; ref != "" {
		proivderRef, err := providers.ParseReference(ref)
		contract.AssertNoErrorf(err, "failed to parse provider reference: %v", ref)
		providerURN := proivderRef.URN()
		if sg.targetsActual.Contains(providerURN) {
			return true
		}
	}

	if res.Parent != "" {
		if sg.targetsActual.Contains(res.Parent) {
			return true
		}
	}
	for _, dep := range res.Dependencies {
		if dep != "" && sg.targetsActual.Contains(dep) {
			return true
		}
	}
	return false
}

func (sg *stepGenerator) isTargetedReplace(urn resource.URN) bool {
	return sg.opts.ReplaceTargets.IsConstrained() && sg.opts.ReplaceTargets.Contains(urn)
}

func (sg *stepGenerator) Errored() bool {
	return sg.sawError
}

// checkParent checks that the parent given is valid for the given resource type, and returns a default parent
// if there is one.
func (sg *stepGenerator) checkParent(parent resource.URN, resourceType tokens.Type) (resource.URN, error) {
	// Some goal settings are based on the parent settings so make sure our parent is correct.
	if resourceType == resource.RootStackType {
		// The RootStack must not have a parent set
		if parent != "" {
			return "", fmt.Errorf("root stack resource can not have a parent (tried to set it to %v)", parent)
		}
	} else {
		// For other resources they may or may not have a parent.
		//
		// TODO(fraser): I think every resource but the RootStack should have a parent, however currently a
		// number of our tests do not create a RootStack resource, feels odd that it's possible for the engine
		// to run without a RootStack resource. I feel this ought to be fixed by making the engine always
		// create the RootStack before running the user program, however that leaves some questions of what to
		// do if we ever support changing any of the settings (such as the provider map) on the RootStack
		// resource. For now we set it to the root stack if we can find it, but we don't error on blank parents

		// If it is set check the parent exists.
		if parent != "" {
			// The parent for this resource hasn't been registered yet. That's an error and we can't continue.
			if _, hasParent := sg.urns[parent]; !hasParent {
				return "", fmt.Errorf("could not find parent resource %v", parent)
			}
		} else { //nolint:staticcheck // https://github.com/pulumi/pulumi/issues/10950
			// Else try and set it to the root stack

			// TODO: It looks like this currently has some issues with state ordering (see
			// https://github.com/pulumi/pulumi/issues/10950). Best I can guess is the stack resource is
			// hitting the step generator and so saving it's URN to sg.urns and issuing a Create step but not
			// actually getting to writing it's state to the snapshot. Then in parallel with this something
			// else is causing a pulumi:providers:pulumi default provider to be created, this picks up the
			// stack URN from sg.urns and so sets it's parent automatically, but then races the step executor
			// to write itself to state before the stack resource manages to. Long term we want to ensure
			// there's always a stack resource present, and so that all resources (except the stack) have a
			// parent (this will save us some work in each SDK), but for now lets just turn this support off.

			//for urn := range sg.urns {
			//	if urn.Type() == resource.RootStackType {
			//		return urn, nil
			//	}
			//}
		}
	}

	return parent, nil
}

// bailDiag prints the given diagnostic to the error stream and then returns a bail error with the same message.
func (sg *stepGenerator) bailDaig(diag *diag.Diag, args ...interface{}) error {
	sg.deployment.Diag().Errorf(diag, args...)
	return result.BailErrorf(diag.Message, args...)
}

// generateURN generates a URN for a new resource and confirms we haven't seen it before in this deployment.
func (sg *stepGenerator) generateURN(
	parent resource.URN, ty tokens.Type, name string,
) (resource.URN, error) {
	// Generate a URN for this new resource, confirm we haven't seen it before in this deployment.
	urn := sg.deployment.generateURN(parent, ty, name)
	if sg.urns[urn] {
		// TODO[pulumi/pulumi-framework#19]: improve this error message!
		return "", sg.bailDaig(diag.GetDuplicateResourceURNError(urn), urn)
	}
	sg.urns[urn] = true
	return urn, nil
}

// GenerateReadSteps is responsible for producing one or more steps required to service
// a ReadResourceEvent coming from the language host.
func (sg *stepGenerator) GenerateReadSteps(event ReadResourceEvent) ([]Step, error) {
	// Some event settings are based on the parent settings so make sure our parent is correct.
	parent, err := sg.checkParent(event.Parent(), event.Type())
	if err != nil {
		return nil, err
	}

	urn, err := sg.generateURN(parent, event.Type(), event.Name())
	if err != nil {
		return nil, err
	}

	newState := resource.NewState(event.Type(),
		urn,
		true,  /*custom*/
		false, /*delete*/
		event.ID(),
		event.Properties(),
		make(resource.PropertyMap), /* outputs */
		parent,
		false, /*protect*/
		true,  /*external*/
		event.Dependencies(),
		nil, /* initErrors */
		event.Provider(),
		nil,   /* propertyDependencies */
		false, /* deleteBeforeCreate */
		event.AdditionalSecretOutputs(),
		nil,   /* aliases */
		nil,   /* customTimeouts */
		"",    /* importID */
		false, /* retainOnDelete */
		"",    /* deletedWith */
		nil,   /* created */
		nil,   /* modified */
		event.SourcePosition(),
	)
	old, hasOld := sg.deployment.Olds()[urn]

	if newState.ID == "" {
		return nil, fmt.Errorf("Expected an ID for %v", urn)
	}

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
		sg.replaces[urn] = true
		return []Step{
			NewReadReplacementStep(sg.deployment, event, old, newState),
			NewReplaceStep(sg.deployment, old, newState, nil, nil, nil, true),
		}, nil
	}

	if bool(logging.V(7)) && hasOld && old.ID == event.ID() {
		logging.V(7).Infof("stepGenerator.GenerateReadSteps(...): recognized relinquish of resource %s", urn)
	}

	sg.reads[urn] = true
	return []Step{
		NewReadStep(sg.deployment, event, old, newState),
	}, nil
}

// GenerateSteps produces one or more steps required to achieve the goal state specified by the
// incoming RegisterResourceEvent.
//
// If the given resource is a custom resource, the step generator will invoke Diff and Check on the
// provider associated with that resource. If those fail, an error is returned.
func (sg *stepGenerator) GenerateSteps(event RegisterResourceEvent) ([]Step, error) {
	steps, err := sg.generateSteps(event)
	if err != nil {
		contract.Assertf(len(steps) == 0, "expected no steps if there is an error")
		return nil, err
	}

	// Check each proposed step against the relevant resource plan, if any
	for _, s := range steps {
		logging.V(5).Infof("Checking step %s for %s", s.Op(), s.URN())

		if sg.deployment.plan != nil {
			if resourcePlan, ok := sg.deployment.plan.ResourcePlans[s.URN()]; ok {
				if len(resourcePlan.Ops) == 0 {
					return nil, fmt.Errorf("%v is not allowed by the plan: no more steps were expected for this resource", s.Op())
				}
				constraint := resourcePlan.Ops[0]
				// We remove the Op from the list before doing the constraint check.
				// This is because we look at Ops at the end to see if any expected operations didn't attempt to happen.
				// This op has been attempted, it just might fail its constraint.
				resourcePlan.Ops = resourcePlan.Ops[1:]
				if !ConstrainedTo(s.Op(), constraint) {
					return nil, fmt.Errorf("%v is not allowed by the plan: this resource is constrained to %v", s.Op(), constraint)
				}
			} else {
				if !ConstrainedTo(s.Op(), OpSame) {
					return nil, fmt.Errorf("%v is not allowed by the plan: no steps were expected for this resource", s.Op())
				}
			}
		}

		// If we're generating plans add the operation to the plan being generated
		if sg.opts.GeneratePlan {
			// Resource plan might be aliased
			urn, isAliased := sg.aliased[s.URN()]
			if !isAliased {
				urn = s.URN()
			}
			if resourcePlan, ok := sg.deployment.newPlans.get(urn); ok {
				// If the resource is in the plan, add the operation to the plan.
				resourcePlan.Ops = append(resourcePlan.Ops, s.Op())
			} else if !ConstrainedTo(s.Op(), OpSame) {
				return nil, fmt.Errorf("Expected a new resource plan for %v", urn)
			}
		}
	}

	// TODO(dixler): `--replace a` currently is treated as a targeted update, but this is not correct.
	//               Removing `|| sg.replaceTargetsOpt.IsConstrained()` would result in a behavior change
	//               that would require some thinking to fully understand the repercussions.
	if !(sg.opts.Targets.IsConstrained() || sg.opts.ReplaceTargets.IsConstrained()) {
		return steps, nil
	}

	// We got a set of steps to perform during a targeted update. If any of the steps are not same steps and depend on
	// creates we skipped because they were not in the --target list, issue an error that that the create was necessary
	// and that the user must target the resource to create.
	for _, step := range steps {
		if step.Op() == OpSame || step.New() == nil {
			continue
		}

		for _, urn := range step.New().Dependencies {
			if sg.skippedCreates[urn] {
				// Targets were specified, but didn't include this resource to create.  And a
				// resource we are producing a step for does depend on this created resource.
				// Give a particular error in that case to let them know.  Also mark that we're
				// in an error state so that we eventually will error out of the entire
				// application run.
				d := diag.GetResourceWillBeCreatedButWasNotSpecifiedInTargetList(step.URN())

				sg.deployment.Diag().Errorf(d, step.URN(), urn)
				sg.sawError = true

				if !sg.deployment.preview {
					// In preview we keep going so that the user will hear about all the problems and can then
					// fix up their command once (as opposed to adding a target, rerunning, adding a target,
					// rerunning, etc. etc.).
					//
					// Doing a normal run.  We should not proceed here at all.  We don't want to create
					// something the user didn't ask for.
					return nil, result.BailErrorf("untargeted create")
				}

				// Remove the resource from the list of skipped creates so that we do not issue duplicate diagnostics.
				delete(sg.skippedCreates, urn)
			}
		}
	}

	return steps, nil
}

func (sg *stepGenerator) collapseAliasToUrn(goal *resource.Goal, alias resource.Alias) resource.URN {
	if alias.URN != "" {
		return alias.URN
	}

	n := alias.Name
	if n == "" {
		n = goal.Name
	}
	t := alias.Type
	if t == "" {
		t = string(goal.Type)
	}

	parent := alias.Parent
	if parent == "" {
		parent = goal.Parent
	} else {
		// If the parent used an alias then use it's old URN here, as that will be this resource old URN as well.
		if parentAlias, has := sg.aliases[parent]; has {
			parent = parentAlias
		}
	}
	parentIsRootStack := parent != "" && parent.Type() == resource.RootStackType
	if alias.NoParent || parentIsRootStack {
		parent = ""
	}

	project := alias.Project
	if project == "" {
		project = sg.deployment.source.Project().String()
	}
	stack := alias.Stack
	if stack == "" {
		stack = sg.deployment.Target().Name.String()
	}

	return resource.CreateURN(n, t, parent, project, stack)
}

// inheritedChildAlias computes the alias that should be applied to a child based on an alias applied to it's
// parent. This may involve changing the name of the resource in cases where the resource has a named derived
// from the name of the parent, and the parent name changed.
func (sg *stepGenerator) inheritedChildAlias(
	childType tokens.Type,
	childName, parentName string,
	parentAlias resource.URN,
) resource.URN {
	// If the child name has the parent name as a prefix, then we make the assumption that
	// it was constructed from the convention of using '{name}-details' as the name of the
	// child resource.  To ensure this is aliased correctly, we must then also replace the
	// parent aliases name in the prefix of the child resource name.
	//
	// For example:
	// * name: "newapp-function"
	// * options.parent.__name: "newapp"
	// * parentAlias: "urn:pulumi:stackname::projectname::awsx:ec2:Vpc::app"
	// * parentAliasName: "app"
	// * aliasName: "app-function"
	// * childAlias: "urn:pulumi:stackname::projectname::aws:s3/bucket:Bucket::app-function"

	aliasName := childName
	if strings.HasPrefix(childName, parentName) {
		aliasName = parentAlias.Name() + strings.TrimPrefix(childName, parentName)
	}
	return resource.NewURN(
		sg.deployment.Target().Name.Q(),
		sg.deployment.source.Project(),
		parentAlias.QualifiedType(),
		childType,
		aliasName)
}

func (sg *stepGenerator) generateAliases(goal *resource.Goal) []resource.URN {
	var result []resource.URN
	aliases := make(map[resource.URN]struct{}, 0)

	addAlias := func(alias resource.URN) {
		if _, has := aliases[alias]; !has {
			aliases[alias] = struct{}{}
			result = append(result, alias)
		}
	}

	for _, alias := range goal.Aliases {
		urn := sg.collapseAliasToUrn(goal, alias)
		addAlias(urn)
	}
	// Now multiply out any aliases our parent had.
	if goal.Parent != "" {
		if parentAlias, has := sg.aliases[goal.Parent]; has {
			addAlias(sg.inheritedChildAlias(goal.Type, goal.Name, goal.Parent.Name(), parentAlias))
			for _, alias := range goal.Aliases {
				childAlias := sg.collapseAliasToUrn(goal, alias)
				aliasedChildType := childAlias.Type()
				aliasedChildName := childAlias.Name()
				inheritedAlias := sg.inheritedChildAlias(aliasedChildType, aliasedChildName, goal.Parent.Name(), parentAlias)
				addAlias(inheritedAlias)
			}
		}
	}

	return result
}

func (sg *stepGenerator) generateSteps(event RegisterResourceEvent) ([]Step, error) {
	var invalid bool // will be set to true if this object fails validation.

	goal := event.Goal()

	// Some goal settings are based on the parent settings so make sure our parent is correct.
	parent, err := sg.checkParent(goal.Parent, goal.Type)
	if err != nil {
		return nil, err
	}
	goal.Parent = parent

	urn, err := sg.generateURN(goal.Parent, goal.Type, goal.Name)
	if err != nil {
		return nil, err
	}

	// Generate the aliases for this resource.
	aliases := sg.generateAliases(goal)

	if previousAliasURN, alreadyAliased := sg.aliased[urn]; alreadyAliased {
		// This resource is claiming to be X but we've already seen another resource claim that via aliases
		invalid = true
		sg.deployment.Diag().Errorf(diag.GetDuplicateResourceAliasedError(urn), urn, previousAliasURN)
	}

	// Check for an old resource so that we can figure out if this is a create, delete, etc., and/or
	// to diff.  We look up first by URN and then by any provided aliases.  If it is found using an
	// alias, record that alias so that we do not delete the aliased resource later.
	var oldInputs resource.PropertyMap
	var oldOutputs resource.PropertyMap
	var old *resource.State
	var hasOld bool
	var alias []resource.Alias
	var createdAt, modifiedAt *time.Time
	// Important: Check the URN first, then aliases. Otherwise we may pick the wrong resource which
	// could lead to a corrupt snapshot.
	for _, urnOrAlias := range append([]resource.URN{urn}, aliases...) {
		old, hasOld = sg.deployment.Olds()[urnOrAlias]
		if hasOld {
			oldInputs = old.Inputs
			oldOutputs = old.Outputs
			createdAt = old.Created
			modifiedAt = old.Modified
			if urnOrAlias != urn {
				if _, alreadySeen := sg.urns[urnOrAlias]; alreadySeen {
					// This resource is claiming to X but we've already seen that urn created
					invalid = true
					sg.deployment.Diag().Errorf(diag.GetDuplicateResourceAliasError(urn), urnOrAlias, urn, urn)
				}
				if previousAliasURN, alreadyAliased := sg.aliased[urnOrAlias]; alreadyAliased {
					// This resource is claiming to be X but we've already seen another resource claim that
					invalid = true
					sg.deployment.Diag().Errorf(diag.GetDuplicateResourceAliasError(urn), urnOrAlias, urn, previousAliasURN)
				}
				sg.aliased[urnOrAlias] = urn

				// register the alias with the provider registry
				sg.deployment.providers.RegisterAlias(urn, urnOrAlias)

				// NOTE: we save the URN of the existing resource so that the snapshotter can replace references to the
				// existing resource with the URN of the newly-registered resource. We do not need to save any of the
				// resource's other possible aliases.
				alias = []resource.Alias{{URN: urnOrAlias}}
				// Save the alias actually being used so we can look it up later if anything has this as a parent
				sg.aliases[urn] = urnOrAlias
			}
			break
		}
	}

	// Create the desired inputs from the goal state
	inputs := goal.Properties
	if hasOld {
		// Set inputs back to their old values (if any) for any "ignored" properties
		processedInputs, err := processIgnoreChanges(inputs, oldInputs, goal.IgnoreChanges)
		if err != nil {
			return nil, err
		}
		inputs = processedInputs
	}

	aliasUrns := make([]resource.URN, len(alias))
	for i, a := range alias {
		aliasUrns[i] = a.URN
	}

	// Produce a new state object that we'll build up as operations are performed.  Ultimately, this is what will
	// get serialized into the checkpoint file.
	new := resource.NewState(goal.Type, urn, goal.Custom, false, "", inputs, nil, goal.Parent, goal.Protect, false,
		goal.Dependencies, goal.InitErrors, goal.Provider, goal.PropertyDependencies, false,
		goal.AdditionalSecretOutputs, aliasUrns, &goal.CustomTimeouts, "", goal.RetainOnDelete, goal.DeletedWith,
		createdAt, modifiedAt, goal.SourcePosition)

	// Mark the URN/resource as having been seen. So we can run analyzers on all resources seen, as well as
	// lookup providers for calculating replacement of resources that use the provider.
	sg.deployment.goals.set(urn, goal)
	if providers.IsProviderType(goal.Type) {
		sg.providers[urn] = new
	}

	// Fetch the provider for this resource.
	prov, err := sg.loadResourceProvider(urn, goal.Custom, goal.Provider, goal.Type)
	if err != nil {
		return nil, err
	}

	// We only allow unknown property values to be exposed to the provider if we are performing an update preview.
	allowUnknowns := sg.deployment.preview

	// We may be re-creating this resource if it got deleted earlier in the execution of this deployment.
	_, recreating := sg.deletes[urn]

	// We may be creating this resource if it previously existed in the snapshot as an External resource
	wasExternal := hasOld && old.External

	// If we have a plan for this resource we need to feed the saved seed to Check to remove non-determinism
	var randomSeed []byte
	if sg.deployment.plan != nil {
		if resourcePlan, ok := sg.deployment.plan.ResourcePlans[urn]; ok {
			randomSeed = resourcePlan.Seed
		}
	}
	// If the above didn't set the seed, generate a new random one. If we're running with plans but this
	// resource was missing a seed then if the seed is used later checks will fail.
	if randomSeed == nil {
		randomSeed = make([]byte, 32)
		n, err := cryptorand.Read(randomSeed)
		contract.AssertNoErrorf(err, "failed to generate random seed")
		contract.Assertf(n == len(randomSeed),
			"generated fewer (%d) than expected (%d) random bytes", n, len(randomSeed))
	}

	// If the goal contains an ID, this may be an import. An import occurs if there is no old resource or if the old
	// resource's ID does not match the ID in the goal state.
	var oldImportID resource.ID
	if hasOld {
		oldImportID = old.ID
		// If the old resource has an ImportID, look at that rather than the ID, since some resources use a different
		// format of identifier for the import input than the ID property.
		if old.ImportID != "" {
			oldImportID = old.ImportID
		}
	}
	isImport := goal.Custom && goal.ID != "" && (!hasOld || old.External || oldImportID != goal.ID)
	if isImport {
		// TODO(seqnum) Not sure how sequence numbers should interact with imports

		// Write the ID of the resource to import into the new state and return an ImportStep or an
		// ImportReplacementStep
		new.ID = goal.ID
		new.ImportID = goal.ID

		// If we're generating plans create a plan, Imports have no diff, just a goal state
		if sg.opts.GeneratePlan {
			newResourcePlan := &ResourcePlan{
				Seed: randomSeed,
				Goal: NewGoalPlan(nil, goal),
			}
			sg.deployment.newPlans.set(urn, newResourcePlan)
		}

		if isReplace := hasOld && !recreating; isReplace {
			return []Step{
				NewImportReplacementStep(sg.deployment, event, old, new, goal.IgnoreChanges, randomSeed),
				NewReplaceStep(sg.deployment, old, new, nil, nil, nil, true),
			}, nil
		}
		return []Step{NewImportStep(sg.deployment, event, new, goal.IgnoreChanges, randomSeed)}, nil
	}

	isImplicitlyTargetedResource := providers.IsProviderType(urn.Type()) || urn.Type() == resource.RootStackType

	// Internally managed resources are under Pulumi's control and changes or creations should be invisible to
	// the user, we also implicitly target providers (both default and explicit, see
	// https://github.com/pulumi/pulumi/issues/13557 and https://github.com/pulumi/pulumi/issues/13591 for
	// context on why).

	// Resources are targeted by default
	isTargeted := true
	if sg.opts.Targets.IsConstrained() && !isImplicitlyTargetedResource {
		isTargeted = sg.isTargetedForUpdate(new)
	}

	// Ensure the provider is okay with this resource and fetch the inputs to pass to subsequent methods.
	if prov != nil {
		var failures []plugin.CheckFailure

		checkInputs := prov.Check
		if !isTargeted {
			// If not targeted, stub out the provider check and use the old inputs directly.
			checkInputs = func(urn resource.URN, olds, news resource.PropertyMap,
				allowUnknowns bool, randomSeed []byte,
			) (resource.PropertyMap, []plugin.CheckFailure, error) {
				return oldInputs, nil, nil
			}
		}

		// If we are re-creating this resource because it was deleted earlier, the old inputs are now
		// invalid (they got deleted) so don't consider them. Similarly, if the old resource was External,
		// don't consider those inputs since Pulumi does not own them. Finally, if the resource has been
		// targeted for replacement, ignore its old state.
		if recreating || wasExternal || sg.isTargetedReplace(urn) || !hasOld {
			inputs, failures, err = checkInputs(urn, nil, goal.Properties, allowUnknowns, randomSeed)
		} else {
			inputs, failures, err = checkInputs(urn, oldInputs, inputs, allowUnknowns, randomSeed)
		}

		if err != nil {
			return nil, err
		} else if issueCheckErrors(sg.deployment, new, urn, failures) {
			invalid = true
		}
		new.Inputs = inputs
	}

	// If the resource is valid and we're generating plans then generate a plan
	if !invalid && sg.opts.GeneratePlan {
		if recreating || wasExternal || sg.isTargetedReplace(urn) || !hasOld {
			oldInputs = nil
		}
		inputDiff := oldInputs.Diff(inputs)

		// Generate the output goal plan, if we're recreating this it should already exist
		if recreating {
			plan, ok := sg.deployment.newPlans.get(urn)
			if !ok {
				return nil, fmt.Errorf("no plan for resource %v", urn)
			}
			// The plan will have had it's Ops already partially filled in for the delete operation, but we
			// now have the information needed to fill in Seed and Goal.
			plan.Seed = randomSeed
			plan.Goal = NewGoalPlan(inputDiff, goal)
		} else {
			newResourcePlan := &ResourcePlan{
				Seed: randomSeed,
				Goal: NewGoalPlan(inputDiff, goal),
			}
			sg.deployment.newPlans.set(urn, newResourcePlan)
		}
	}

	// If there is a plan for this resource, validate that the program goal conforms to the plan.
	// If theres no plan for this resource check that nothing has been changed.
	// We don't check plans if the resource is invalid, it's going to fail anyway.
	if !invalid && sg.deployment.plan != nil {
		resourcePlan, ok := sg.deployment.plan.ResourcePlans[urn]
		if !ok {
			if old == nil {
				// We could error here, but we'll trigger an error later on anyway that Create isn't valid here
			} else if err := checkMissingPlan(old, inputs, goal); err != nil {
				return nil, fmt.Errorf("resource %s violates plan: %w", urn, err)
			}
		} else {
			if err := resourcePlan.checkGoal(oldInputs, inputs, goal); err != nil {
				return nil, fmt.Errorf("resource %s violates plan: %w", urn, err)
			}
		}
	}

	// Send the resource off to any Analyzers before being operated on. We do two passes: first we perform
	// remediatoins, and *then* we do analysis, since we want analyzers to run on the final resource states.
	analyzers := sg.deployment.ctx.Host.ListAnalyzers()
	for _, remediate := range []bool{true, false} {
		for _, analyzer := range analyzers {
			r := plugin.AnalyzerResource{
				URN:        new.URN,
				Type:       new.Type,
				Name:       new.URN.Name(),
				Properties: inputs,
				Options: plugin.AnalyzerResourceOptions{
					Protect:                 new.Protect,
					IgnoreChanges:           goal.IgnoreChanges,
					DeleteBeforeReplace:     goal.DeleteBeforeReplace,
					AdditionalSecretOutputs: new.AdditionalSecretOutputs,
					Aliases:                 new.GetAliases(),
					CustomTimeouts:          new.CustomTimeouts,
				},
			}
			providerResource := sg.getProviderResource(new.URN, new.Provider)
			if providerResource != nil {
				r.Provider = &plugin.AnalyzerProviderResource{
					URN:        providerResource.URN,
					Type:       providerResource.Type,
					Name:       providerResource.URN.Name(),
					Properties: providerResource.Inputs,
				}
			}

			if remediate {
				// During the first pass, perform remediations. This ensures subsequent analyzers run
				// against the transformed properties, ensuring nothing circumvents the analysis checks.
				tresults, err := analyzer.Remediate(r)
				if err != nil {
					return nil, fmt.Errorf("failed to run remediation: %w", err)
				} else if len(tresults) > 0 {
					for _, tresult := range tresults {
						if tresult.Diagnostic != "" {
							// If there is a diagnostic, we have a warning to display.
							sg.opts.Events.OnPolicyViolation(new.URN, plugin.AnalyzeDiagnostic{
								PolicyName:        tresult.PolicyName,
								PolicyPackName:    tresult.PolicyPackName,
								PolicyPackVersion: tresult.PolicyPackVersion,
								Description:       tresult.Description,
								Message:           tresult.Diagnostic,
								EnforcementLevel:  apitype.Advisory,
								URN:               new.URN,
							})
						} else if tresult.Properties != nil {
							// Emit a nice message so users know what was remediated.
							sg.opts.Events.OnPolicyRemediation(new.URN, tresult, inputs, tresult.Properties)
							// Use the transformed inputs rather than the old ones from this point onwards.
							inputs = tresult.Properties
							new.Inputs = tresult.Properties
						}
					}
				}
			} else {
				// During the second pass, perform analysis. This happens after remediations so that
				// analyzers see properties as they were after the transformations have occurred.
				diagnostics, err := analyzer.Analyze(r)
				if err != nil {
					return nil, fmt.Errorf("failed to run policy: %w", err)
				}
				for _, d := range diagnostics {
					if d.EnforcementLevel == apitype.Mandatory {
						if !sg.deployment.preview {
							invalid = true
						}
						sg.sawError = true
					}
					// For now, we always use the URN we have here rather than a URN specified with the diagnostic.
					sg.opts.Events.OnPolicyViolation(new.URN, d)
				}
			}
		}
	}

	// If the resource isn't valid, don't proceed any further.
	if invalid {
		return nil, result.BailErrorf("resource %s is invalid", urn)
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
	contract.Assertf(!recreating || hasOld, "cannot recreate a resource that doesn't exist")
	if recreating {
		logging.V(7).Infof("Planner decided to re-create replaced resource '%v' deleted due to dependent DBR", urn)

		// Unmark this resource as deleted, we now know it's being replaced instead.
		delete(sg.deletes, urn)
		sg.replaces[urn] = true
		keys := sg.dependentReplaceKeys[urn]
		return []Step{
			NewReplaceStep(sg.deployment, old, new, nil, nil, nil, false),
			NewCreateReplacementStep(sg.deployment, event, old, new, keys, nil, nil, false),
		}, nil
	}

	// Case 2: wasExternal
	//  In this case, the resource we are operating upon exists in the old snapshot, but it
	//  was "external" - Pulumi does not own its lifecycle. Conceptually, this operation is
	//  akin to "taking ownership" of a resource that we did not previously control.
	//
	//  Since we are not allowed to manipulate the existing resource, we must create a resource
	//  to take its place. Since this is technically a replacement operation, we pend deletion of
	//  read until the end of the deployment.
	if wasExternal {
		logging.V(7).Infof("Planner recognized '%s' as old external resource, creating instead", urn)
		sg.creates[urn] = true
		if err != nil {
			return nil, err
		}

		return []Step{
			NewCreateReplacementStep(sg.deployment, event, old, new, nil, nil, nil, true),
			NewReplaceStep(sg.deployment, old, new, nil, nil, nil, true),
		}, nil
	}

	// This looks odd that we have to recheck isTargetedForUpdate but it's to cover implicitly targeted
	// resources like providers (where isTargeted is always true), but which might have been _explicitly_
	// targeted due to being in the --targets list or being explicitly pulled in by --target-dependents.
	if isTargeted && sg.isTargetedForUpdate(new) {
		// Transitive dependencies are not initially targeted, ensure that they are in the Targets so that the
		// step_generator identifies that the URN is targeted if applicable
		sg.targetsActual.addLiteral(urn)
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
		contract.Assertf(old != nil, "must have old resource if hasOld is true")

		// If the user requested only specific resources to update, and this resource was not in
		// that set, then do nothing but create a SameStep for it.
		if !isTargeted {
			logging.V(7).Infof(
				"Planner decided not to update '%v' due to not being in target group (same) (inputs=%v)", urn, new.Inputs)
		} else {
			updateSteps, err := sg.generateStepsFromDiff(
				event, urn, old, new, oldInputs, oldOutputs, inputs, prov, goal, randomSeed)
			if err != nil {
				return nil, err
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
		sg.sames[urn] = true
		return []Step{NewSameStep(sg.deployment, event, old, new)}, nil
	}

	// Case 4: Not Case 1, 2, or 3
	//  If a resource isn't being recreated and it's not being updated or replaced,
	//  it's just being created.

	// We're in the create stage now.  In a normal run just issue a 'create step'. If, however, the
	// user is doing a run with `--target`s, then we need to operate specially here.
	//
	// 1. If the user did include this resource urn in the --target list, then we can proceed
	// normally and issue a create step for this.
	//
	// 2. However, if they did not include the resource in the --target list, then we want to flat
	// out ignore it (just like we ignore updates to resource not in the --target list).  This has
	// interesting implications though. Specifically, what to do if a prop from this resource is
	// then actually needed by a property we *are* doing a targeted create/update for.
	//
	// In that case, we want to error to force the user to be explicit about wanting this resource
	// to be created. However, we can't issue the error until later on when the resource is
	// referenced. So, to support this we create a special "same" step here for this resource. That
	// "same" step has a bit on it letting us know that it is for this case. If we then later see a
	// resource that depends on this resource, we will issue an error letting the user know.
	//
	// We will also not record this non-created resource into the checkpoint as it doesn't actually
	// exist.

	if !isTargeted {
		sg.sames[urn] = true
		sg.skippedCreates[urn] = true
		return []Step{NewSkippedCreateStep(sg.deployment, event, new)}, nil
	}

	sg.creates[urn] = true
	logging.V(7).Infof("Planner decided to create '%v' (inputs=%v)", urn, new.Inputs)
	return []Step{NewCreateStep(sg.deployment, event, new)}, nil
}

func (sg *stepGenerator) generateStepsFromDiff(
	event RegisterResourceEvent, urn resource.URN, old, new *resource.State,
	oldInputs, oldOutputs, inputs resource.PropertyMap,
	prov plugin.Provider, goal *resource.Goal, randomSeed []byte,
) ([]Step, error) {
	// We only allow unknown property values to be exposed to the provider if we are performing an update preview.
	allowUnknowns := sg.deployment.preview

	diff, err := sg.diff(urn, old, new, oldInputs, oldOutputs, inputs, prov, allowUnknowns, goal.IgnoreChanges)
	// If the plugin indicated that the diff is unavailable, assume that the resource will be updated and
	// report the message contained in the error.
	if _, ok := err.(plugin.DiffUnavailableError); ok {
		diff = plugin.DiffResult{Changes: plugin.DiffSome}
		sg.deployment.ctx.Diag.Warningf(diag.RawMessage(urn, err.Error()))
	} else if err != nil {
		return nil, err
	}

	// Ensure that we received a sensible response.
	if diff.Changes != plugin.DiffNone && diff.Changes != plugin.DiffSome {
		return nil, fmt.Errorf(
			"unrecognized diff state for %s: %d", urn, diff.Changes)
	}

	hasInitErrors := len(old.InitErrors) > 0

	// Update the diff to apply any replaceOnChanges annotations and to include initErrors in the diff.
	diff, err = applyReplaceOnChanges(diff, goal.ReplaceOnChanges, hasInitErrors)
	if err != nil {
		return nil, err
	}

	// If there were changes check for a replacement vs. an in-place update.
	if diff.Changes == plugin.DiffSome {
		if diff.Replace() {
			// If this resource is protected we can't replace it because that entails a delete
			// Note that we do allow unprotecting and replacing to happen in a single update
			// cycle, we don't look at old.Protect here.
			if new.Protect && old.Protect {
				message := fmt.Sprintf("unable to replace resource %q\n"+
					"as it is currently marked for protection. To unprotect the resource, "+
					"remove the `protect` flag from the resource in your Pulumi "+
					"program and run `pulumi up`", urn)
				sg.deployment.ctx.Diag.Errorf(diag.StreamMessage(urn, message, 0))
				sg.sawError = true
				return nil, result.BailErrorf(message)
			}

			// If the goal state specified an ID, issue an error: the replacement will change the ID, and is
			// therefore incompatible with the goal state.
			if goal.ID != "" {
				const message = "previously-imported resources that still specify an ID may not be replaced; " +
					"please remove the `import` declaration from your program"
				if sg.deployment.preview {
					sg.deployment.ctx.Diag.Warningf(diag.StreamMessage(urn, message, 0))
				} else {
					return nil, fmt.Errorf(message)
				}
			}

			sg.replaces[urn] = true

			// If we are going to perform a replacement, we need to recompute the default values.  The above logic
			// had assumed that we were going to carry them over from the old resource, which is no longer true.
			//
			// Note that if we're performing a targeted replace, we already have the correct inputs.
			if prov != nil && !sg.isTargetedReplace(urn) {
				var failures []plugin.CheckFailure
				inputs, failures, err = prov.Check(urn, nil, goal.Properties, allowUnknowns, randomSeed)
				if err != nil {
					return nil, err
				} else if issueCheckErrors(sg.deployment, new, urn, failures) {
					return nil, result.BailErrorf("resource %v has check errors: %v", urn, failures)
				}
				new.Inputs = inputs
			}

			if logging.V(7) {
				logging.V(7).Infof("Planner decided to replace '%v' (oldprops=%v inputs=%v replaceKeys=%v)",
					urn, oldInputs, new.Inputs, diff.ReplaceKeys)
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
				contract.Assertf(sg.deployment.depGraph != nil,
					"dependency graph must be available for delete-before-replace")

				// DeleteBeforeCreate implies that we must immediately delete the resource. For correctness,
				// we must also eagerly delete all resources that depend directly or indirectly on the resource
				// being replaced and would be replaced by a change to the relevant dependency.
				//
				// To do this, we'll utilize the dependency information contained in the snapshot if it is
				// trustworthy, which is interpreted by the DependencyGraph type.
				var steps []Step
				if sg.opts.TrustDependencies {
					toReplace, err := sg.calculateDependentReplacements(old)
					if err != nil {
						return nil, err
					}

					// Deletions must occur in reverse dependency order, and `deps` is returned in dependency
					// order, so we iterate in reverse.
					for i := len(toReplace) - 1; i >= 0; i-- {
						dependentResource := toReplace[i].res

						// If we already deleted this resource due to some other DBR, don't do it again.
						if sg.pendingDeletes[dependentResource] {
							continue
						}

						// If we're generating plans create a plan for this delete
						if sg.opts.GeneratePlan {
							if _, ok := sg.deployment.newPlans.get(dependentResource.URN); !ok {
								// We haven't see this resource before, create a new
								// resource plan for it with no goal (because it's going to be a delete)
								resourcePlan := &ResourcePlan{}
								sg.deployment.newPlans.set(dependentResource.URN, resourcePlan)
							}
						}

						sg.dependentReplaceKeys[dependentResource.URN] = toReplace[i].keys

						logging.V(7).Infof("Planner decided to delete '%v' due to dependence on condemned resource '%v'",
							dependentResource.URN, urn)

						// This resource might already be pending-delete
						if dependentResource.Delete {
							steps = append(steps, NewDeleteStep(sg.deployment, sg.deletes, dependentResource))
						} else {
							steps = append(steps, NewDeleteReplacementStep(sg.deployment, sg.deletes, dependentResource, true))
						}
						// Mark the condemned resource as deleted. We won't know until later in the deployment whether
						// or not we're going to be replacing this resource.
						sg.deletes[dependentResource.URN] = true
						sg.pendingDeletes[dependentResource] = true
					}
				}

				// We're going to delete the old resource before creating the new one. We need to make sure
				// that the old provider is loaded.
				err := sg.deployment.EnsureProvider(old.Provider)
				if err != nil {
					return nil, fmt.Errorf("could not load provider for resource %v: %w", old.URN, err)
				}

				return append(steps,
					NewDeleteReplacementStep(sg.deployment, sg.deletes, old, true),
					NewReplaceStep(sg.deployment, old, new, diff.ReplaceKeys, diff.ChangedKeys, diff.DetailedDiff, false),
					NewCreateReplacementStep(
						sg.deployment, event, old, new, diff.ReplaceKeys, diff.ChangedKeys, diff.DetailedDiff, false),
				), nil
			}

			return []Step{
				NewCreateReplacementStep(
					sg.deployment, event, old, new, diff.ReplaceKeys, diff.ChangedKeys, diff.DetailedDiff, true),
				NewReplaceStep(sg.deployment, old, new, diff.ReplaceKeys, diff.ChangedKeys, diff.DetailedDiff, true),
				// note that the delete step is generated "later" on, after all creates/updates finish.
			}, nil
		}

		// If we fell through, it's an update.
		sg.updates[urn] = true
		if logging.V(7) {
			logging.V(7).Infof("Planner decided to update '%v' (oldprops=%v inputs=%v)", urn, oldInputs, new.Inputs)
		}
		return []Step{
			NewUpdateStep(sg.deployment, event, old, new, diff.StableKeys, diff.ChangedKeys, diff.DetailedDiff,
				goal.IgnoreChanges),
		}, nil
	}

	// If resource was unchanged, but there were initialization errors, generate an empty update
	// step to attempt to "continue" awaiting initialization.
	if hasInitErrors {
		sg.updates[urn] = true
		return []Step{NewUpdateStep(sg.deployment, event, old, new, diff.StableKeys, nil, nil, nil)}, nil
	}

	// Else there are no changes needed
	return nil, nil
}

func (sg *stepGenerator) GenerateDeletes(targetsOpt UrnTargets) ([]Step, error) {
	// To compute the deletion list, we must walk the list of old resources *backwards*.  This is because the list is
	// stored in dependency order, and earlier elements are possibly leaf nodes for later elements.  We must not delete
	// dependencies prior to their dependent nodes.
	var dels []Step
	if prev := sg.deployment.prev; prev != nil {
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
				dels = append(dels, NewDeleteReplacementStep(sg.deployment, sg.deletes, res, false))
			} else if _, aliased := sg.aliased[res.URN]; !sg.sames[res.URN] && !sg.updates[res.URN] && !sg.replaces[res.URN] &&
				!sg.reads[res.URN] && !aliased {
				// NOTE: we deliberately do not check sg.deletes here, as it is possible for us to issue multiple
				// delete steps for the same URN if the old checkpoint contained pending deletes.
				logging.V(7).Infof("Planner decided to delete '%v'", res.URN)
				sg.deletes[res.URN] = true
				if !res.PendingReplacement {
					dels = append(dels, NewDeleteStep(sg.deployment, sg.deletes, res))
				} else {
					dels = append(dels, NewRemovePendingReplaceStep(sg.deployment, res))
				}
			}

			// We just added a Delete step, so we need to ensure the provider for this resource is available.
			if sg.deletes[res.URN] {
				err := sg.deployment.EnsureProvider(res.Provider)
				if err != nil {
					return nil, fmt.Errorf("could not load provider for resource %v: %w", res.URN, err)
				}
			}
		}
	}

	// Check each proposed delete against the relevant resource plan
	for _, s := range dels {
		if sg.deployment.plan != nil {
			if resourcePlan, ok := sg.deployment.plan.ResourcePlans[s.URN()]; ok {
				if len(resourcePlan.Ops) == 0 {
					return nil, fmt.Errorf("%v is not allowed by the plan: no more steps were expected for this resource", s.Op())
				}

				constraint := resourcePlan.Ops[0]
				// We remove the Op from the list before doing the constraint check.
				// This is because we look at Ops at the end to see if any expected operations didn't attempt to happen.
				// This op has been attempted, it just might fail its constraint.
				resourcePlan.Ops = resourcePlan.Ops[1:]

				if !ConstrainedTo(s.Op(), constraint) {
					return nil, fmt.Errorf("%v is not allowed by the plan: this resource is constrained to %v", s.Op(), constraint)
				}
			} else {
				if !ConstrainedTo(s.Op(), OpSame) {
					return nil, fmt.Errorf("%v is not allowed by the plan: no steps were expected for this resource", s.Op())
				}
			}
		}

		// If we're generating plans add a delete op to the plan for this resource
		if sg.opts.GeneratePlan {
			resourcePlan, ok := sg.deployment.newPlans.get(s.URN())
			if !ok {
				// TODO(pdg-plan): using the program inputs means that non-determinism could sneak in as part of default
				// application. However, it is necessary in the face of computed inputs.
				resourcePlan = &ResourcePlan{}
				sg.deployment.newPlans.set(s.URN(), resourcePlan)
			}
			resourcePlan.Ops = append(resourcePlan.Ops, s.Op())
		}
	}

	// If -target was provided to either `pulumi update` or `pulumi destroy` then only delete
	// resources that were specified.
	allowedResourcesToDelete, err := sg.determineAllowedResourcesToDeleteFromTargets(targetsOpt)
	if err != nil {
		return nil, err
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

	deletingUnspecifiedTarget := false
	for _, step := range dels {
		urn := step.URN()
		if !targetsOpt.Contains(urn) && !sg.opts.TargetDependents {
			d := diag.GetResourceWillBeDestroyedButWasNotSpecifiedInTargetList(urn)

			// Targets were specified, but didn't include this resource to create.  Report all the
			// problematic targets so the user doesn't have to keep adding them one at a time and
			// re-running the operation.
			//
			// Mark that step generation entered an error state so that the entire app run fails.
			sg.deployment.Diag().Errorf(d, urn)
			sg.sawError = true

			deletingUnspecifiedTarget = true
		}
	}

	if deletingUnspecifiedTarget && !sg.deployment.preview {
		// In preview we keep going so that the user will hear about all the problems and can then
		// fix up their command once (as opposed to adding a target, rerunning, adding a target,
		// rerunning, etc. etc.).
		//
		// Doing a normal run.  We should not proceed here at all.  We don't want to delete
		// something the user didn't ask for.
		return nil, result.BailErrorf("delete untargeted resource")
	}

	return dels, nil
}

// getTargetDependents returns the (transitive) set of dependents on the target resources.
// This includes both implicit and explicit dependents in the DAG itself, as well as children.
func (sg *stepGenerator) getTargetDependents(targetsOpt UrnTargets) map[resource.URN]bool {
	// Seed the list with the initial set of targets.
	var frontier []*resource.State
	for _, res := range sg.deployment.prev.Resources {
		if targetsOpt.Contains(res.URN) {
			frontier = append(frontier, res)
		}
	}

	// Produce a dependency graph of resources.
	dg := graph.NewDependencyGraph(sg.deployment.prev.Resources)

	// Now accumulate a list of targets that are implicated because they depend upon the targets.
	targets := make(map[resource.URN]bool)
	for len(frontier) > 0 {
		// Pop the next to explore, mark it, and skip any we've already seen.
		next := frontier[0]
		frontier = frontier[1:]
		if _, has := targets[next.URN]; has {
			continue
		}
		targets[next.URN] = true

		// Compute the set of resources depending on this one, either implicitly, explicitly,
		// or because it is a child resource. Add them to the frontier to keep exploring.
		deps := dg.DependingOn(next, targets, true)
		frontier = append(frontier, deps...)
	}

	return targets
}

// determineAllowedResourcesToDeleteFromTargets computes the full (transitive) closure of resources
// that need to be deleted to permit the full list of targetsOpt resources to be deleted. This list
// will include the targetsOpt resources, but may contain more than just that, if there are dependent
// or child resources that require the targets to exist (and so are implicated in the deletion).
func (sg *stepGenerator) determineAllowedResourcesToDeleteFromTargets(
	targetsOpt UrnTargets,
) (map[resource.URN]bool, error) {
	if !targetsOpt.IsConstrained() {
		// no specific targets, so we won't filter down anything
		return nil, nil
	}

	// Produce a map of targets and their dependents, including explicit and implicit
	// DAG dependencies, as well as children (transitively).
	targets := sg.getTargetDependents(targetsOpt)
	logging.V(7).Infof("Planner was asked to only delete/update '%v'", targetsOpt)
	resourcesToDelete := make(map[resource.URN]bool)

	// Now actually use all the requested targets to figure out the exact set to delete.
	for target := range targets {
		current := sg.deployment.olds[target]
		if current == nil {
			// user specified a target that didn't exist.  they will have already gotten a warning
			// about this when we called checkTargets.  explicitly ignore this target since it won't
			// be something we could possibly be trying to delete, nor could have dependents we
			// might need to replace either.
			continue
		}

		resourcesToDelete[target] = true

		// the item the user is asking to destroy may cause downstream replacements.  Clean those up
		// as well. Use the standard delete-before-replace computation to determine the minimal
		// set of downstream resources that are affected.
		deps, err := sg.calculateDependentReplacements(current)
		if err != nil {
			return nil, err
		}

		for _, dep := range deps {
			logging.V(7).Infof("GenerateDeletes(...): Adding dependent: %v", dep.res.URN)
			resourcesToDelete[dep.res.URN] = true
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

// ScheduleDeletes takes a list of steps that will delete resources and "schedules" them by producing a list of list of
// steps, where each list can be executed in parallel but a previous list must be executed to completion before
// advancing to the next list.
//
// In lieu of tracking per-step dependencies and orienting the step executor around these dependencies, this function
// provides a conservative approximation of what deletions can safely occur in parallel. The insight here is that the
// resource dependency graph is a partially-ordered set and all partially-ordered sets can be easily decomposed into
// antichains - subsets of the set that are all not comparable to one another. (In this definition, "not comparable"
// means "do not depend on one another").
//
// The algorithm for decomposing a poset into antichains is:
//  1. While there exist elements in the poset,
//     1a. There must exist at least one "maximal" element of the poset. Let E_max be those elements.
//     2a. Remove all elements E_max from the poset. E_max is an antichain.
//     3a. Goto 1.
//
// Translated to our dependency graph:
//  1. While the set of condemned resources is not empty:
//     1a. Remove all resources with no outgoing edges from the graph and add them to the current antichain.
//     2a. Goto 1.
//
// The resulting list of antichains is a list of list of steps that can be safely executed in parallel. Since we must
// process deletes in reverse (so we don't delete resources upon which other resources depend), we reverse the list and
// hand it back to the deployment executor for safe execution.
func (sg *stepGenerator) ScheduleDeletes(deleteSteps []Step) []antichain {
	var antichains []antichain                // the list of parallelizable steps we intend to return.
	dg := sg.deployment.depGraph              // the current deployment's dependency graph.
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
	logging.V(stepExecutorLogLevel).Infof("sg.diffProvider(%s, ...): %v => %v", urn, old.Provider, new.Provider)

	// If we're changing from a component resource to a non-component resource, there is no old provider to
	// diff against and trigger a delete but we need to Create the new custom resource. If we're changing from
	// a custom resource to a component resource, we should always trigger a replace.
	if old.Provider == "" || new.Provider == "" {
		return true, nil
	}

	oldRef, err := providers.ParseReference(old.Provider)
	if err != nil {
		return false, err
	}
	newRef, err := providers.ParseReference(new.Provider)
	if err != nil {
		return false, err
	}

	if alias, ok := sg.aliased[oldRef.URN()]; ok && alias == newRef.URN() {
		logging.V(stepExecutorLogLevel).Infof(
			"sg.diffProvider(%s, ...): observed an aliased provider from %q to %q", urn, oldRef.URN(), newRef.URN())
		return false, nil
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
	newProv, ok := sg.deployment.providers.GetProvider(newRef)
	if !ok {
		return false, fmt.Errorf("failed to resolve provider reference: %q", oldRef.String())
	}

	oldRes, ok := sg.deployment.olds[oldRef.URN()]
	contract.Assertf(ok, "old state didn't have provider, despite resource using it?")
	newRes, ok := sg.providers[newRef.URN()]
	contract.Assertf(ok, "new deployment didn't have provider, despite resource using it?")

	diff, err := newProv.DiffConfig(newRef.URN(), oldRes.Inputs, oldRes.Outputs, newRes.Inputs, true, nil)
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
	ignoreChanges []string,
) (plugin.DiffResult, error) {
	// If this resource is marked for replacement, just return a "replace" diff that blames the id.
	if sg.isTargetedReplace(urn) {
		return plugin.DiffResult{Changes: plugin.DiffSome, ReplaceKeys: []resource.PropertyKey{"id"}}, nil
	}

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
	ignoreChanges []string,
) (plugin.DiffResult, error) {
	contract.Requiref(prov != nil, "prov", "must not be nil")

	// Grab the diff from the provider. At this point we know that there were changes to the Pulumi inputs, so if the
	// provider returns an "unknown" diff result, pretend it returned "diffs exist".
	diff, err := prov.Diff(urn, id, oldInputs, oldOutputs, newInputs, allowUnknowns, ignoreChanges)
	if err != nil {
		return diff, err
	}
	if diff.Changes == plugin.DiffUnknown {
		new, res := processIgnoreChanges(newInputs, oldInputs, ignoreChanges)
		if res != nil {
			return plugin.DiffResult{}, err
		}
		tmp := oldInputs.Diff(new)
		if tmp.AnyChanges() {
			diff.Changes = plugin.DiffSome
			diff.ChangedKeys = tmp.ChangedKeys()
			diff.DetailedDiff = plugin.NewDetailedDiffFromObjectDiff(tmp, true /* inputDiff */)
		} else {
			diff.Changes = plugin.DiffNone
		}
	}
	return diff, nil
}

// issueCheckErrors prints any check errors to the diagnostics error sink.
func issueCheckErrors(deployment *Deployment, new *resource.State, urn resource.URN,
	failures []plugin.CheckFailure,
) bool {
	return issueCheckFailures(deployment.Diag().Errorf, new, urn, failures)
}

// issueCheckErrors prints any check errors to the given printer function.
func issueCheckFailures(printf func(*diag.Diag, ...interface{}), new *resource.State, urn resource.URN,
	failures []plugin.CheckFailure,
) bool {
	if len(failures) == 0 {
		return false
	}
	inputs := new.Inputs
	for _, failure := range failures {
		if failure.Property != "" {
			printf(diag.GetResourcePropertyInvalidValueError(urn),
				new.Type, urn.Name(), failure.Property, inputs[failure.Property], failure.Reason)
		} else {
			printf(
				diag.GetResourceInvalidError(urn), new.Type, urn.Name(), failure.Reason)
		}
	}
	return true
}

// processIgnoreChanges sets the value for each ignoreChanges property in inputs to the value from oldInputs.  This has
// the effect of ensuring that no changes will be made for the corresponding property.
func processIgnoreChanges(inputs, oldInputs resource.PropertyMap,
	ignoreChanges []string,
) (resource.PropertyMap, error) {
	ignoredInputs := inputs.Copy()
	var invalidPaths []string
	for _, ignoreChange := range ignoreChanges {
		path, err := resource.ParsePropertyPath(ignoreChange)
		if err != nil {
			continue
		}

		ok := path.Reset(oldInputs, ignoredInputs)
		if !ok {
			invalidPaths = append(invalidPaths, ignoreChange)
		}
	}
	if len(invalidPaths) != 0 {
		return nil, fmt.Errorf("cannot ignore changes to the following properties because one or more elements of "+
			"the path are missing: %q", strings.Join(invalidPaths, ", "))
	}
	return ignoredInputs, nil
}

func (sg *stepGenerator) loadResourceProvider(
	urn resource.URN, custom bool, provider string, typ tokens.Type,
) (plugin.Provider, error) {
	// If this is not a custom resource, then it has no provider by definition.
	if !custom {
		return nil, nil
	}

	// If this resource is a provider resource, use the deployment's provider registry for its CRUD operations.
	// Otherwise, resolve the the resource's provider reference.
	if providers.IsProviderType(typ) {
		return sg.deployment.providers, nil
	}

	contract.Assertf(provider != "", "must have a provider for custom resource %v", urn)
	ref, refErr := providers.ParseReference(provider)
	if refErr != nil {
		return nil, sg.bailDaig(diag.GetBadProviderError(urn), provider, urn, refErr)
	}
	if providers.IsDenyDefaultsProvider(ref) {
		pkg := providers.GetDeniedDefaultProviderPkg(ref)
		return nil, sg.bailDaig(diag.GetDefaultProviderDenied(urn), pkg, urn)
	}
	p, ok := sg.deployment.GetProvider(ref)
	if !ok {
		return nil, sg.bailDaig(diag.GetUnknownProviderError(urn), provider, urn)
	}
	return p, nil
}

func (sg *stepGenerator) getProviderResource(urn resource.URN, provider string) *resource.State {
	if provider == "" {
		return nil
	}

	// All callers of this method are on paths that have previously validated that the provider
	// reference can be parsed correctly and has a provider resource in the map.
	ref, err := providers.ParseReference(provider)
	contract.AssertNoErrorf(err, "failed to parse provider reference")
	result := sg.providers[ref.URN()]
	contract.Assertf(result != nil, "provider missing from step generator providers map")
	return result
}

// initErrorSpecialKey is a special property key used to indicate that a diff is due to
// initialization errors existing in the old state instead of due to a specific property
// diff between old and new states.
const initErrorSpecialKey = "#initerror"

// applyReplaceOnChanges adjusts a DiffResult returned from a provider to apply the ReplaceOnChange
// settings in the desired state and init errors from the previous state.
func applyReplaceOnChanges(diff plugin.DiffResult,
	replaceOnChanges []string, hasInitErrors bool,
) (plugin.DiffResult, error) {
	// No further work is necessary for DiffNone unless init errors are present.
	if diff.Changes != plugin.DiffSome && !hasInitErrors {
		return diff, nil
	}

	replaceOnChangePaths := slice.Prealloc[resource.PropertyPath](len(replaceOnChanges))
	for _, p := range replaceOnChanges {
		path, err := resource.ParsePropertyPath(p)
		if err != nil {
			return diff, err
		}
		replaceOnChangePaths = append(replaceOnChangePaths, path)
	}

	// Calculate the new DetailedDiff
	var modifiedDiff map[string]plugin.PropertyDiff
	if diff.DetailedDiff != nil {
		modifiedDiff = map[string]plugin.PropertyDiff{}
		for p, v := range diff.DetailedDiff {
			diffPath, err := resource.ParsePropertyPath(p)
			if err != nil {
				return diff, err
			}
			changeToReplace := false
			for _, replaceOnChangePath := range replaceOnChangePaths {
				if replaceOnChangePath.Contains(diffPath) {
					changeToReplace = true
					break
				}
			}
			if changeToReplace {
				v = v.ToReplace()
			}
			modifiedDiff[p] = v
		}
	}

	// Calculate the new ReplaceKeys
	modifiedReplaceKeysMap := map[resource.PropertyKey]struct{}{}
	for _, k := range diff.ReplaceKeys {
		modifiedReplaceKeysMap[k] = struct{}{}
	}
	for _, k := range diff.ChangedKeys {
		for _, replaceOnChangePath := range replaceOnChangePaths {
			keyPath, err := resource.ParsePropertyPath(string(k))
			if err != nil {
				continue
			}
			if replaceOnChangePath.Contains(keyPath) {
				modifiedReplaceKeysMap[k] = struct{}{}
			}
		}
	}
	modifiedReplaceKeys := slice.Prealloc[resource.PropertyKey](len(modifiedReplaceKeysMap))
	for k := range modifiedReplaceKeysMap {
		modifiedReplaceKeys = append(modifiedReplaceKeys, k)
	}

	// Add init errors to modified diff results
	modifiedChanges := diff.Changes
	if hasInitErrors {
		for _, replaceOnChangePath := range replaceOnChangePaths {
			initErrPath, err := resource.ParsePropertyPath(initErrorSpecialKey)
			if err != nil {
				continue
			}
			if replaceOnChangePath.Contains(initErrPath) {
				modifiedReplaceKeys = append(modifiedReplaceKeys, initErrorSpecialKey)
				if modifiedDiff != nil {
					modifiedDiff[initErrorSpecialKey] = plugin.PropertyDiff{
						Kind:      plugin.DiffUpdateReplace,
						InputDiff: false,
					}
				}
				// If an init error is present on a path that causes replacement, then trigger a replacement.
				modifiedChanges = plugin.DiffSome
			}
		}
	}

	return plugin.DiffResult{
		DetailedDiff:        modifiedDiff,
		ReplaceKeys:         modifiedReplaceKeys,
		ChangedKeys:         diff.ChangedKeys,
		Changes:             modifiedChanges,
		DeleteBeforeReplace: diff.DeleteBeforeReplace,
		StableKeys:          diff.StableKeys,
	}, nil
}

type dependentReplace struct {
	res  *resource.State
	keys []resource.PropertyKey
}

func (sg *stepGenerator) calculateDependentReplacements(root *resource.State) ([]dependentReplace, error) {
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

	requiresReplacement := func(r *resource.State) (bool, []resource.PropertyKey, error) {
		// Neither component nor external resources require replacement.
		if !r.Custom || r.External {
			return false, nil, nil
		}

		// If the resource's provider is in the replace set, we must replace this resource.
		if r.Provider != "" {
			ref, err := providers.ParseReference(r.Provider)
			if err != nil {
				return false, nil, err
			}
			if replaceSet[ref.URN()] {
				// We need to use the old provider configuration to delete this resource so ensure it's loaded.
				err := sg.deployment.EnsureProvider(r.Provider)
				if err != nil {
					return false, nil, fmt.Errorf("could not load provider for resource %v: %w", r.URN, err)
				}

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

		// We're going to have to call diff on this resources provider so ensure that we have it created
		if !providers.IsProviderType(r.Type) {
			err := sg.deployment.EnsureProvider(r.Provider)
			if err != nil {
				return false, nil, fmt.Errorf("could not load provider for resource %v: %w", r.URN, err)
			}
		} else {
			// This is a provider itself so load it so that Diff below is possible
			err := sg.deployment.SameProvider(r)
			if err != nil {
				return false, nil, fmt.Errorf("create provider %v: %w", r.URN, err)
			}
		}

		// Otherwise, fetch the resource's provider. Since we have filtered out component resources, this resource must
		// have a provider.
		prov, err := sg.loadResourceProvider(r.URN, r.Custom, r.Provider, r.Type)
		if err != nil {
			return false, nil, err
		}
		contract.Assertf(prov != nil, "resource %v has no provider", r.URN)

		// Call the provider's `Diff` method and return.
		diff, err := prov.Diff(r.URN, r.ID, r.Inputs, r.Outputs, inputsForDiff, true, nil)
		if err != nil {
			return false, nil, err
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
	// that have already been registered must not depend on the root. Thus, we ignore these resources if they are
	// encountered while walking the old dependency graph to determine the set of dependents.
	impossibleDependents := sg.urns
	for _, d := range sg.deployment.depGraph.DependingOn(root, impossibleDependents, false) {
		replace, keys, err := requiresReplacement(d)
		if err != nil {
			return nil, err
		}
		if replace {
			toReplace, replaceSet[d.URN] = append(toReplace, dependentReplace{res: d, keys: keys}), true
		}
	}

	// Return the list of resources to replace.
	return toReplace, nil
}

func (sg *stepGenerator) AnalyzeResources() error {
	var resources []plugin.AnalyzerStackResource
	sg.deployment.news.mapRange(func(urn resource.URN, v *resource.State) bool {
		goal, ok := sg.deployment.goals.get(urn)
		contract.Assertf(ok, "failed to load goal for %s", urn)
		resource := plugin.AnalyzerStackResource{
			AnalyzerResource: plugin.AnalyzerResource{
				URN:  v.URN,
				Type: v.Type,
				Name: v.URN.Name(),
				// Unlike Analyze, AnalyzeStack is called on the final outputs of each resource,
				// to verify the final stack is in a compliant state.
				Properties: v.Outputs,
				Options: plugin.AnalyzerResourceOptions{
					Protect:                 v.Protect,
					IgnoreChanges:           goal.IgnoreChanges,
					DeleteBeforeReplace:     goal.DeleteBeforeReplace,
					AdditionalSecretOutputs: v.AdditionalSecretOutputs,
					Aliases:                 v.GetAliases(),
					CustomTimeouts:          v.CustomTimeouts,
				},
			},
			Parent:               v.Parent,
			Dependencies:         v.Dependencies,
			PropertyDependencies: v.PropertyDependencies,
		}
		providerResource := sg.getProviderResource(v.URN, v.Provider)
		if providerResource != nil {
			resource.Provider = &plugin.AnalyzerProviderResource{
				URN:        providerResource.URN,
				Type:       providerResource.Type,
				Name:       providerResource.URN.Name(),
				Properties: providerResource.Inputs,
			}
		}
		resources = append(resources, resource)
		return true
	})

	analyzers := sg.deployment.ctx.Host.ListAnalyzers()
	for _, analyzer := range analyzers {
		diagnostics, err := analyzer.AnalyzeStack(resources)
		if err != nil {
			return err
		}
		for _, d := range diagnostics {
			sg.sawError = sg.sawError || (d.EnforcementLevel == apitype.Mandatory)
			// If a URN was provided and it is a URN associated with a resource in the stack, use it.
			// Otherwise, if the URN is empty or is not associated with a resource in the stack, use
			// the default root stack URN.
			var urn resource.URN
			if d.URN != "" {
				if _, ok := sg.deployment.news.get(d.URN); ok {
					urn = d.URN
				}
			}
			if urn == "" {
				urn = resource.DefaultRootStackURN(sg.deployment.Target().Name.Q(), sg.deployment.source.Project())
			}
			sg.opts.Events.OnPolicyViolation(urn, d)
		}
	}

	return nil
}

// newStepGenerator creates a new step generator that operates on the given deployment.
func newStepGenerator(
	deployment *Deployment, opts Options, updateTargetsOpt, replaceTargetsOpt UrnTargets,
) *stepGenerator {
	return &stepGenerator{
		deployment:           deployment,
		opts:                 opts,
		urns:                 make(map[resource.URN]bool),
		reads:                make(map[resource.URN]bool),
		creates:              make(map[resource.URN]bool),
		sames:                make(map[resource.URN]bool),
		replaces:             make(map[resource.URN]bool),
		updates:              make(map[resource.URN]bool),
		deletes:              make(map[resource.URN]bool),
		skippedCreates:       make(map[resource.URN]bool),
		pendingDeletes:       make(map[*resource.State]bool),
		providers:            make(map[resource.URN]*resource.State),
		dependentReplaceKeys: make(map[resource.URN][]resource.PropertyKey),
		aliased:              make(map[resource.URN]resource.URN),
		aliases:              make(map[resource.URN]resource.URN),
		targetsActual:        opts.Targets.Clone(),
	}
}
