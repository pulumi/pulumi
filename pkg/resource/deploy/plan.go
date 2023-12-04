package deploy

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mitchellh/copystructure"

	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/pkg/v3/version"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// A Plan is a mapping from URNs to ResourcePlans. The plan defines an expected set of resources and the expected
// inputs and operations for each. The inputs and operations are treated as constraints, and may allow for inputs or
// operations that do not exactly match those recorded in the plan. In the case of inputs, unknown values in the plan
// accept any value (including no value) as valid. For operations, a same step is allowed in place of an update or
// a replace step, and an update is allowed in place of a replace step. All resource options are required to match
// exactly.
type Plan struct {
	ResourcePlans map[resource.URN]*ResourcePlan
	Manifest      Manifest
	// The configuration in use during the plan.
	Config config.Map
}

func NewPlan(config config.Map) Plan {
	manifest := Manifest{
		Time:    time.Now(),
		Version: version.Version,
		// Plugins: sm.plugins, - Explicitly dropped, since we don't use the plugin list in the manifest anymore.
	}
	manifest.Magic = manifest.NewMagic()

	return Plan{
		ResourcePlans: make(map[resource.URN]*ResourcePlan),
		Manifest:      manifest,
		Config:        config,
	}
}

// Clone makes a deep copy of the given plan and returns a pointer to the clone.
func (plan *Plan) Clone() *Plan {
	return copystructure.Must(copystructure.Copy(plan)).(*Plan)
}

// PlanDiff holds the results of diffing two object property maps.
type PlanDiff struct {
	Adds    resource.PropertyMap   // the resource's properties we expect to add.
	Deletes []resource.PropertyKey // the resource's properties we expect to delete.
	Updates resource.PropertyMap   // the resource's properties we expect to update.
}

// Returns true if the Deletes array contains the given key
func (planDiff *PlanDiff) ContainsDelete(key resource.PropertyKey) bool {
	found := false
	for i := range planDiff.Deletes {
		if planDiff.Deletes[i] == key {
			found = true
			break
		}
	}
	return found
}

func (planDiff *PlanDiff) MakeError(
	key resource.PropertyKey,
	actualOperation string,
	actualValue *resource.PropertyValue,
) string {
	// diff wants to do 'actualOperation' (one of '+', '~', '-', '=') but plan differs. This function looks up what
	// key wanted to do to print a more useful error message

	// See if the plan was an add, remove, update, or nothing to give a better error message
	var expectedOperation string
	var expectedValue *resource.PropertyValue
	if expected, has := planDiff.Adds[key]; has {
		expectedValue = &expected
		expectedOperation = "+"
	} else if expected, has = planDiff.Updates[key]; has {
		expectedValue = &expected
		expectedOperation = "~"
	} else if planDiff.ContainsDelete(key) {
		expectedOperation = "-"
	} else {
		expectedOperation = "="
	}
	diff := ""
	if actualValue != nil && expectedValue != nil {
		diff = "[" + expectedValue.String() + "!=" + actualValue.String() + "]"
	} else if actualValue != nil {
		diff = "[" + actualValue.String() + "]"
	} else if expectedValue != nil {
		diff = "[" + expectedValue.String() + "]"
	}
	return expectedOperation + actualOperation + string(key) + diff
}

// Goal is a desired state for a resource object.  Normally it represents a subset of the resource's state expressed by
// a program, however if Output is true, it represents a more complete, post-deployment view of the state.
type GoalPlan struct {
	// the type of resource.
	Type tokens.Type
	// the name for the resource's URN.
	Name string
	// true if this resource is custom, managed by a plugin.
	Custom bool
	// the resource's checked input properties we expect to change.
	InputDiff PlanDiff
	// the resource's output properties we expect to change (only set for RegisterResourceOutputs)
	OutputDiff PlanDiff
	// an optional parent URN for this resource.
	Parent resource.URN
	// true to protect this resource from deletion.
	Protect bool
	// dependencies of this resource object.
	Dependencies []resource.URN
	// the provider to use for this resource.
	Provider string
	// the set of dependencies that affect each property.
	PropertyDependencies map[resource.PropertyKey][]resource.URN
	// true if this resource should be deleted prior to replacement.
	DeleteBeforeReplace *bool
	// a list of property names to ignore during changes.
	IgnoreChanges []string
	// outputs that should always be treated as secrets.
	AdditionalSecretOutputs []resource.PropertyKey
	// Structured Alias objects to be assigned to this resource
	Aliases []resource.Alias
	// the expected ID of the resource, if any.
	ID resource.ID
	// an optional config object for resource options
	CustomTimeouts resource.CustomTimeouts
}

func NewPlanDiff(inputDiff *resource.ObjectDiff) PlanDiff {
	var adds resource.PropertyMap
	var deletes []resource.PropertyKey
	var updates resource.PropertyMap

	var diff PlanDiff
	if inputDiff != nil {
		adds = inputDiff.Adds
		updates = make(resource.PropertyMap)
		for k := range inputDiff.Updates {
			updates[k] = inputDiff.Updates[k].New
		}
		deletes = make([]resource.PropertyKey, len(inputDiff.Deletes))
		i := 0
		for k := range inputDiff.Deletes {
			deletes[i] = k
			i = i + 1
		}

		diff = PlanDiff{Adds: adds, Deletes: deletes, Updates: updates}
	}

	return diff
}

func NewGoalPlan(inputDiff *resource.ObjectDiff, goal *resource.Goal) *GoalPlan {
	if goal == nil {
		return nil
	}

	diff := NewPlanDiff(inputDiff)

	return &GoalPlan{
		Type:                    goal.Type,
		Name:                    goal.Name,
		Custom:                  goal.Custom,
		InputDiff:               diff,
		OutputDiff:              PlanDiff{},
		Parent:                  goal.Parent,
		Protect:                 goal.Protect,
		Dependencies:            goal.Dependencies,
		Provider:                goal.Provider,
		PropertyDependencies:    goal.PropertyDependencies,
		DeleteBeforeReplace:     goal.DeleteBeforeReplace,
		IgnoreChanges:           goal.IgnoreChanges,
		AdditionalSecretOutputs: goal.AdditionalSecretOutputs,
		Aliases:                 goal.Aliases,
		ID:                      goal.ID,
		CustomTimeouts:          goal.CustomTimeouts,
	}
}

// A ResourcePlan represents the planned goal state and resource operations for a single resource. The operations are
// ordered.
type ResourcePlan struct {
	Goal    *GoalPlan
	Ops     []display.StepOp
	Outputs resource.PropertyMap
	// The random byte seed used for resource goal.
	Seed []byte
}

func (rp *ResourcePlan) diffURNs(a, b []resource.URN) (message string, changed bool) {
	stringsA := make([]string, len(a))
	for i, urn := range a {
		stringsA[i] = string(urn)
	}
	stringsB := make([]string, len(b))
	for i, urn := range b {
		stringsB[i] = string(urn)
	}
	return rp.diffStringSets(stringsA, stringsB)
}

func (rp *ResourcePlan) diffPropertyKeys(a, b []resource.PropertyKey) (message string, changed bool) {
	stringsA := make([]string, len(a))
	for i, key := range a {
		stringsA[i] = string(key)
	}
	stringsB := make([]string, len(a))
	for i, key := range b {
		stringsB[i] = string(key)
	}
	return rp.diffStringSets(stringsA, stringsB)
}

func (rp *ResourcePlan) diffStringSets(a, b []string) (message string, changed bool) {
	setA := map[string]struct{}{}
	for _, s := range a {
		setA[s] = struct{}{}
	}

	setB := map[string]struct{}{}
	for _, s := range b {
		setB[s] = struct{}{}
	}

	var adds, deletes []string
	for s := range setA {
		if _, has := setB[s]; !has {
			deletes = append(deletes, s)
		}
	}
	for s := range setB {
		if _, has := setA[s]; !has {
			adds = append(adds, s)
		}
	}

	if len(adds) == 0 && len(deletes) == 0 {
		return "", false
	}

	sort.Strings(adds)
	sort.Strings(deletes)

	if len(adds) != 0 {
		message = fmt.Sprintf("added %v", strings.Join(adds, ", "))
	}
	if len(deletes) != 0 {
		if len(adds) != 0 {
			message += "; "
		}
		message += fmt.Sprintf("deleted %v", strings.Join(deletes, ", "))
	}
	return message, true
}

func (rp *ResourcePlan) diffAliases(a, b []resource.Alias) (message string, changed bool) {
	setA := map[resource.Alias]struct{}{}
	for _, s := range a {
		setA[s] = struct{}{}
	}

	setB := map[resource.Alias]struct{}{}
	for _, s := range b {
		setB[s] = struct{}{}
	}

	var adds, deletes []string
	for s := range setA {
		if _, has := setB[s]; !has {
			deletes = append(deletes, fmt.Sprintf("%v", s))
		}
	}
	for s := range setB {
		if _, has := setA[s]; !has {
			adds = append(adds, fmt.Sprintf("%v", s))
		}
	}

	if len(adds) == 0 && len(deletes) == 0 {
		return "", false
	}

	sort.Strings(adds)
	sort.Strings(deletes)

	if len(adds) != 0 {
		message = fmt.Sprintf("added %v", strings.Join(adds, ", "))
	}
	if len(deletes) != 0 {
		if len(adds) != 0 {
			message += "; "
		}
		message += fmt.Sprintf("deleted %v", strings.Join(deletes, ", "))
	}
	return message, true
}

// This is similar to ResourcePlan.checkGoal but for the case we're we don't have a goal saved.
// This simple checks that we're not changing anything.
func checkMissingPlan(
	oldState *resource.State,
	newInputs resource.PropertyMap,
	programGoal *resource.Goal,
) error {
	// We new up a fake ResourcePlan that matches the old state and then simply call checkGoal on it.
	goal := &GoalPlan{
		Type:                    oldState.Type,
		Name:                    oldState.URN.Name(),
		Custom:                  oldState.Custom,
		InputDiff:               PlanDiff{},
		OutputDiff:              PlanDiff{},
		Parent:                  oldState.Parent,
		Protect:                 oldState.Protect,
		Dependencies:            oldState.Dependencies,
		Provider:                oldState.Provider,
		PropertyDependencies:    oldState.PropertyDependencies,
		DeleteBeforeReplace:     nil,
		IgnoreChanges:           nil,
		AdditionalSecretOutputs: oldState.AdditionalSecretOutputs,
		Aliases:                 oldState.GetAliases(),
		ID:                      "",
		CustomTimeouts:          oldState.CustomTimeouts,
	}

	rp := ResourcePlan{Goal: goal}
	return rp.checkGoal(oldState.Inputs, newInputs, programGoal)
}

func checkDiff(olds, news resource.PropertyMap, planDiff PlanDiff) error {
	changes := []string{}
	var diff *resource.ObjectDiff
	if diff = olds.DiffIncludeUnknowns(news); diff != nil {
		// Check that any adds are in the goal for adds
		for k := range diff.Adds {
			actual := diff.Adds[k]
			if expected, has := planDiff.Adds[k]; has {
				if !expected.DeepEqualsIncludeUnknowns(actual) {
					// diff wants to add this with value X but constraint wants to add with value Y
					changes = append(changes, planDiff.MakeError(k, "+", &actual))
				}
			} else {
				// diff wants to add this, but not listed as an add in the constraints
				changes = append(changes, planDiff.MakeError(k, "+", &actual))
			}
		}

		// Check that any removes are in the goal for removes
		for k := range diff.Deletes {
			if !planDiff.ContainsDelete(k) {
				// diff wants to delete this, but not listed as a delete in the constraints

				// Check if this was recorded as an Update with <computed>
				if expected, has := planDiff.Updates[k]; has {
					if expected.IsComputed() {
						// This was planned as an Update to <computed> it probably resolved to undefined and so became
						// a delete, this is not a plan violation
					} else {
						// diff wants to delete this, plan wants to update it
						changes = append(changes, planDiff.MakeError(k, "-", nil))
					}
				} else {
					// diff wants to delete this, but not listed as a delete in the constraints
					changes = append(changes, planDiff.MakeError(k, "-", nil))
				}
			}
		}

		// Check that any changes are in the goal for changes or adds
		// "or adds" is because if our constraint says to add K=V and someone has already
		// added K=W we don't consider it a constraint violation to update K to V.
		// This is similar to how if we have a Create resource constraint we don't consider it
		// a violation to just update it instead of creating it.
		for k := range diff.Updates {
			actual := diff.Updates[k].New
			if expected, has := planDiff.Updates[k]; has {
				if !expected.DeepEqualsIncludeUnknowns(actual) {
					// diff wants to change this with value X but constraint wants to change with value Y
					changes = append(changes, planDiff.MakeError(k, "~", &actual))
				}
			} else if expected, has := planDiff.Adds[k]; has {
				if !expected.DeepEqualsIncludeUnknowns(actual) {
					// diff wants to change this with value X but constraint wants to add with value Y
					changes = append(changes, planDiff.MakeError(k, "~", &actual))
				}
			} else {
				// diff wants to update this, but not listed as an update in the constraints
				changes = append(changes, planDiff.MakeError(k, "~", &actual))
			}
		}
	} else {
		// No diff, just new up an empty ObjectDiff for checks below
		diff = &resource.ObjectDiff{}
	}

	// Symmetric check, check that the constraints didn't expect things to happen that aren't in the new inputs

	for k := range planDiff.Adds {
		// We expected an add, make sure the value is in the new inputs.
		// That means it's either an add, update, or a same, both are ok for an add constraint.
		expected := planDiff.Adds[k]

		// If this is in diff.Adds or diff.Updates we'll of already checked it
		_, inAdds := diff.Adds[k]
		_, inUpdates := diff.Updates[k]

		if !inAdds && !inUpdates {
			// It wasn't in the diff as an add or update so check we have a same
			if actual, has := news[k]; has {
				if !expected.DeepEqualsIncludeUnknowns(actual) {
					// diff wants to same this with value X but constraint wants to add with value Y
					changes = append(changes, planDiff.MakeError(k, "=", &actual))
				}
			} else {
				// Not a same, update or an add but constraint wants to add it

				// Check if this was <computed> origionally because that could of resolved to undefined
				// and thus it's ok to be missing, else this is a real missing property
				if !expected.IsComputed() {
					changes = append(changes, planDiff.MakeError(k, "-", nil))
				}
			}
		}
	}

	for k := range planDiff.Updates {
		// We expected an update, make sure the value is in the new inputs as an update (not an add)
		expected := planDiff.Updates[k]

		// If this is in diff.Updates we'll of already checked it
		_, inUpdates := diff.Updates[k]

		if !inUpdates {
			// Check if this was in adds, it's not ok to have an update constraint but actually do an add
			if actual, has := diff.Adds[k]; has {
				// Constraint wants to update it, but diff wants to add it
				changes = append(changes, planDiff.MakeError(k, "+", &actual))
			} else if actual, has := news[k]; has {
				// It wasn't in the diff as an add so check we have a same
				if !expected.DeepEqualsIncludeUnknowns(actual) {
					// diff wants to same this with value X but constraint wants to update with value Y
					changes = append(changes, planDiff.MakeError(k, "=", &actual))
				}
			} else {
				// Not a same or an update but constraint wants to update it

				// Check if this was <computed> origionally because that could of resolved to undefined
				// and thus it's ok to be missing, else this is a real missing property
				if !expected.IsComputed() {
					changes = append(changes, planDiff.MakeError(k, "-", nil))
				}
			}
		}
	}

	for i := range planDiff.Deletes {
		// We expected a delete, make sure its not present
		k := planDiff.Deletes[i]

		// If this is in diff.Deletes we'll of already checked it
		_, inDeletes := diff.Deletes[k]
		if !inDeletes {
			// See if this is an add, update, or same
			if actual, has := diff.Adds[k]; has {
				// Constraint wants to delete this but diff wants to add it
				changes = append(changes, planDiff.MakeError(k, "+", &actual))
			} else if actual, has := diff.Updates[k]; has {
				// Constraint wants to delete this but diff wants to update it
				changes = append(changes, planDiff.MakeError(k, "~", &actual.New))
			} else if actual, has := diff.Sames[k]; has {
				// Constraint wants to delete this but diff wants to leave it same
				changes = append(changes, planDiff.MakeError(k, "=", &actual))
			}
		}
	}

	if len(changes) > 0 {
		// Sort changes, mostly so it's easy to write tests against determinstic strings
		sort.Strings(changes)
		return fmt.Errorf("properties changed: %v", strings.Join(changes, ", "))
	}

	return nil
}

func (rp *ResourcePlan) checkOutputs(
	oldOutputs resource.PropertyMap,
	newOutputs resource.PropertyMap,
) error {
	contract.Assertf(rp.Goal != nil, "resource plan goal must be set")

	// Check that the property diffs meet the constraints set in the plan.
	return checkDiff(oldOutputs, newOutputs, rp.Goal.OutputDiff)
}

func (rp *ResourcePlan) checkGoal(
	oldInputs resource.PropertyMap,
	newInputs resource.PropertyMap,
	programGoal *resource.Goal,
) error {
	contract.Requiref(programGoal != nil, "programGoal", "must not be nil")
	// rp.Goal may be nil, but if it isn't Type and Name should match
	if rp.Goal != nil {
		contract.Assertf(rp.Goal.Type == programGoal.Type,
			"resource plan goal type (%v) does not match program goal type (%v)", rp.Goal.Type, programGoal.Type)
		contract.Assertf(rp.Goal.Name == programGoal.Name,
			"resource plan goal name (%v) does not match program goal name (%v)", rp.Goal.Type, programGoal.Name)
	}

	if rp.Goal == nil {
		// If the plan goal is nil it expected a delete
		return fmt.Errorf("resource unexpectedly not deleted")
	}

	// Check that either both resources are custom resources or both are component resources.
	if programGoal.Custom != rp.Goal.Custom {
		// TODO(pdg-plan): wording?
		expected := "custom"
		if !rp.Goal.Custom {
			expected = "component"
		}
		return fmt.Errorf("resource kind changed (expected %v)", expected)
	}

	// Check that the provider is identical.
	if rp.Goal.Provider != programGoal.Provider {
		// Provider references are a combination of URN and ID, the latter of which may be unknown. Check for that
		// case here.
		expected, err := providers.ParseReference(rp.Goal.Provider)
		if err != nil {
			return fmt.Errorf("failed to parse provider reference %v: %w", rp.Goal.Provider, err)
		}
		actual, err := providers.ParseReference(programGoal.Provider)
		if err != nil {
			return fmt.Errorf("failed to parse provider reference %v: %w", programGoal.Provider, err)
		}
		if expected.URN() != actual.URN() || expected.ID() != providers.UnknownID {
			return fmt.Errorf("provider changed (expected %v)", rp.Goal.Provider)
		}
	}

	// Check that the parent is identical.
	if programGoal.Parent != rp.Goal.Parent {
		return fmt.Errorf("parent changed (expected %v)", rp.Goal.Parent)
	}

	// Check that the protect bit is identical.
	if programGoal.Protect != rp.Goal.Protect {
		return fmt.Errorf("protect changed (expected %v)", rp.Goal.Protect)
	}

	// Check that the DBR bit is identical.
	switch {
	case rp.Goal.DeleteBeforeReplace == nil && programGoal.DeleteBeforeReplace == nil:
		// OK
	case rp.Goal.DeleteBeforeReplace != nil && programGoal.DeleteBeforeReplace != nil:
		if *rp.Goal.DeleteBeforeReplace != *programGoal.DeleteBeforeReplace {
			return fmt.Errorf("deleteBeforeReplace changed (expected %v)", *rp.Goal.DeleteBeforeReplace)
		}
	default:
		expected := "no value"
		if rp.Goal.DeleteBeforeReplace != nil {
			expected = fmt.Sprintf("%v", *rp.Goal.DeleteBeforeReplace)
		}
		return fmt.Errorf("deleteBeforeReplace changed (expected %v)", expected)
	}

	// Check that the import ID is identical.
	if rp.Goal.ID != programGoal.ID {
		return fmt.Errorf("importID changed (expected %v)", rp.Goal.ID)
	}

	// Check that the timeouts are identical.
	switch {
	case rp.Goal.CustomTimeouts.Create != programGoal.CustomTimeouts.Create:
		return fmt.Errorf("create timeout changed (expected %v)", rp.Goal.CustomTimeouts.Create)
	case rp.Goal.CustomTimeouts.Update != programGoal.CustomTimeouts.Update:
		return fmt.Errorf("update timeout changed (expected %v)", rp.Goal.CustomTimeouts.Update)
	case rp.Goal.CustomTimeouts.Delete != programGoal.CustomTimeouts.Delete:
		return fmt.Errorf("delete timeout changed (expected %v)", rp.Goal.CustomTimeouts.Delete)
	}

	// Check that the ignoreChanges sets are identical.
	if message, changed := rp.diffStringSets(rp.Goal.IgnoreChanges, programGoal.IgnoreChanges); changed {
		return fmt.Errorf("ignoreChanges changed: %v", message)
	}

	// Check that the additionalSecretOutputs sets are identical.
	if message, changed := rp.diffPropertyKeys(
		rp.Goal.AdditionalSecretOutputs, programGoal.AdditionalSecretOutputs); changed {
		return fmt.Errorf("additionalSecretOutputs changed: %v", message)
	}

	// Check that the dependencies match.
	if message, changed := rp.diffURNs(rp.Goal.Dependencies, programGoal.Dependencies); changed {
		return fmt.Errorf("dependencies changed: %v", message)
	}

	// Check that the actual alias sets are identical.
	if message, changed := rp.diffAliases(rp.Goal.Aliases, programGoal.Aliases); changed {
		return fmt.Errorf("aliases changed: %v", message)
	}

	// Check that the property diffs meet the constraints set in the plan
	if err := checkDiff(oldInputs, newInputs, rp.Goal.InputDiff); err != nil {
		return err
	}

	// Check that the property dependencies match. Note that because it is legal for a property that is unknown in the
	// plan to be unset in the program, we allow the omission of a property from the program's dependency set.
	for k, urns := range rp.Goal.PropertyDependencies {
		if programDeps, ok := programGoal.PropertyDependencies[k]; ok {
			if message, changed := rp.diffURNs(urns, programDeps); changed {
				return fmt.Errorf("dependencies for %v changed: %v", k, message)
			}
		}
	}

	return nil
}
