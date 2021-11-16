package deploy

import (
	"fmt"
	"sort"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// A Plan is a mapping from URNs to ResourcePlans. The plan defines an expected set of resources and the expected
// inputs and operations for each. The inputs and operations are treated as constraints, and may allow for inputs or
// operations that do not exactly match those recorded in the plan. In the case of inputs, unknown values in the plan
// accept any value (including no value) as valid. For operations, a same step is allowed in place of an update or
// a replace step, and an update is allowed in place of a replace step. All resource options are required to match
// exactly.
type Plan map[resource.URN]*ResourcePlan

// Goal is a desired state for a resource object.  Normally it represents a subset of the resource's state expressed by
// a program, however if Output is true, it represents a more complete, post-deployment view of the state.
type GoalPlan struct {
	Type                    tokens.Type                             // the type of resource.
	Name                    tokens.QName                            // the name for the resource's URN.
	Custom                  bool                                    // true if this resource is custom, managed by a plugin.
	Adds                    resource.PropertyMap                    // the resource's properties we expect to add.
	Deletes                 []resource.PropertyKey                  // the resource's properties we expect to delete.
	Updates                 resource.PropertyMap                    // the resource's properties we expect to update.
	Parent                  resource.URN                            // an optional parent URN for this resource.
	Protect                 bool                                    // true to protect this resource from deletion.
	Dependencies            []resource.URN                          // dependencies of this resource object.
	Provider                string                                  // the provider to use for this resource.
	PropertyDependencies    map[resource.PropertyKey][]resource.URN // the set of dependencies that affect each property.
	DeleteBeforeReplace     *bool                                   // true if this resource should be deleted prior to replacement.
	IgnoreChanges           []string                                // a list of property names to ignore during changes.
	AdditionalSecretOutputs []resource.PropertyKey                  // outputs that should always be treated as secrets.
	Aliases                 []resource.URN                          // additional URNs that should be aliased to this resource.
	ID                      resource.ID                             // the expected ID of the resource, if any.
	CustomTimeouts          resource.CustomTimeouts                 // an optional config object for resource options
}

func NewGoalPlan(oldOutputs resource.PropertyMap, goal *resource.Goal) *GoalPlan {
	if goal == nil {
		return nil
	}

	return &GoalPlan{
		Type:                    goal.Type,
		Name:                    goal.Name,
		Custom:                  goal.Custom,
		Adds:                    nil,
		Deletes:                 nil,
		Updates:                 nil,
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
	Ops     []StepOp
	Outputs resource.PropertyMap
}

func (rp *ResourcePlan) diffURNs(a, b []resource.URN) (message string, changed bool) {
	stringsA := make([]string, len(a))
	for i, urn := range a {
		stringsA[i] = string(urn)
	}
	stringsB := make([]string, len(a))
	for i, urn := range b {
		stringsB[i] = string(urn)
	}
	return rp.diffStrings(stringsA, stringsB)
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
	return rp.diffStrings(stringsA, stringsB)
}

func (rp *ResourcePlan) diffStrings(a, b []string) (message string, changed bool) {
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

	sort.Strings(adds)
	sort.Strings(deletes)

	if len(adds) == 0 && len(deletes) == 0 {
		return "", false
	}

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

func (rp *ResourcePlan) diffPropertyDependencies(a, b map[resource.PropertyKey][]resource.URN) error {
	return nil
}

func (rp *ResourcePlan) checkGoal(
	oldOutputs resource.PropertyMap,
	newInputs resource.PropertyMap,
	programGoal *resource.Goal) error {

	contract.Assert(programGoal != nil)
	contract.Assert(newInputs != nil)
	// rp.Goal may be nil, but if it isn't Type and Name should match
	contract.Assert(rp.Goal == nil || rp.Goal.Type == programGoal.Type)
	contract.Assert(rp.Goal == nil || rp.Goal.Name == programGoal.Name)

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
	if message, changed := rp.diffStrings(rp.Goal.IgnoreChanges, programGoal.IgnoreChanges); changed {
		return fmt.Errorf("ignoreChanges changed: %v", message)
	}

	// Check that the additionalSecretOutputs sets are identical.
	if message, changed := rp.diffPropertyKeys(
		rp.Goal.AdditionalSecretOutputs, programGoal.AdditionalSecretOutputs); changed {
		return fmt.Errorf("additionalSecretOutputs changed: %v", message)
	}

	// Check that the alias sets are identical.
	if message, changed := rp.diffURNs(rp.Goal.Aliases, programGoal.Aliases); changed {
		return fmt.Errorf("aliases changed: %v", message)
	}

	// Check that the dependencies match.
	if message, changed := rp.diffURNs(rp.Goal.Dependencies, programGoal.Dependencies); changed {
		return fmt.Errorf("dependencies changed: %v", message)
	}

	// Check that the property diffs meet the constraints set in the plan.
	changes := []string{}
	if diff, hasDiff := oldOutputs.DiffIncludeUnknowns(newInputs); hasDiff {
		// Check that any adds are in the goal for adds
		for k := range diff.Adds {
			if expected, has := rp.Goal.Adds[k]; has {
				actual := diff.Adds[k]
				if !expected.IsComputed() && !expected.DeepEquals(actual) {
					// diff wants to add this with value X but constraint wants to add with value Y
					changes = append(changes, "+"+string(k))
				}
			} else {
				// diff wants to add this, but not listed as an add in the constraints
				changes = append(changes, "+"+string(k))
			}
		}

		// Check that any removes are in the goal for removes
		for k := range diff.Deletes {
			found := false
			for i := range rp.Goal.Deletes {
				if rp.Goal.Deletes[i] == k {
					found = true
					break
				}
			}

			if !found {
				// diff wants to delete this, but not listed as a delete in the constraints
				changes = append(changes, "-"+string(k))
			}
		}

		// Check that any changes are in the goal for changes
		for k := range diff.Updates {
			if expected, has := rp.Goal.Updates[k]; has {
				actual := diff.Adds[k]
				if !expected.IsComputed() && !expected.DeepEquals(actual) {
					// diff wants to change this with value X but constraint wants to change with value Y
					changes = append(changes, "~"+string(k))
				}
			} else {
				// diff wants to update this, but not listed as an update in the constraints
				changes = append(changes, "~"+string(k))
			}
		}
	}

	if len(changes) > 0 {
		return fmt.Errorf("properties changed: %v", strings.Join(changes, ", "))
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
