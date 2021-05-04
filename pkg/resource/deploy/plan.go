package deploy

import (
	"fmt"
	"sort"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// A Plan is a mapping from URNs to ResourcePlans. The plan defines an expected set of resources and the expected
// inputs and operations for each. The inputs and operations are treated as constraints, and may allow for inputs or
// operations that do not exactly match those recorded in the plan. In the case of inputs, unknown values in the plan
// accept any value (including no value) as valid. For operations, a same step is allowed in place of an update or
// a replace step, and an update is allowed in place of a replace step. All resource options are required to match
// exactly.
type Plan map[resource.URN]*ResourcePlan

// A ResourcePlan represents the planned goal state and resource operations for a single resource. The operations are
// ordered.
type ResourcePlan struct {
	Goal    *resource.Goal
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

func (rp *ResourcePlan) checkGoal(programGoal *resource.Goal) error {
	contract.Assert(rp.Goal.Type == programGoal.Type)
	contract.Assert(rp.Goal.Name == programGoal.Name)

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
	if message, changed := rp.diffPropertyKeys(rp.Goal.AdditionalSecretOutputs, programGoal.AdditionalSecretOutputs); changed {
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

	// Check that the properties meet the constraints set in the plan.
	if diff, constrained := programGoal.Properties.ConstrainedTo(rp.Goal.Properties); !constrained {
		// TODO(pdg-plan): message!
		var paths []string
		for k := range diff.Adds {
			paths = append(paths, "+"+string(k))
		}
		for k := range diff.Deletes {
			paths = append(paths, "-"+string(k))
		}
		for k := range diff.Updates {
			paths = append(paths, "~"+string(k))
		}
		return fmt.Errorf("properties changed: %v", strings.Join(paths, ", "))
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
