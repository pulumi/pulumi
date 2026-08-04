// Copyright 2026, Pulumi Corporation.
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
	"fmt"
	"maps"
	"reflect"

	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	sdkproviders "github.com/pulumi/pulumi/sdk/v3/go/common/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// StateMigrationTransaction is the prepared state change produced from a registration's migration callbacks. It is
// treated as immutable after preparation so snapshot persistence and the in-memory migration commit consume the same
// prepared values.
//
// The transaction records two related things:
//   - PriorSubtree, ResultSubtree, and SuccessorURNs describe the subtree replacement, while
//     PreparedPriorResources describes the resulting complete prior-resource list.
//   - RetainedResourceRewrites, retainedRewriteIndices, and currentResourceRewrites associate prepared values with
//     live resource objects whose pointer identities must be preserved by persistence or the migration commit.
//
// A "retained resource" is a resource outside PriorSubtree that remains in Deployment.prev.Resources after the
// migration. It keeps its live pointer, although references in its state may need to be rewritten. For example, suppose
// Deployment.prev.Resources is [P, A, B, C], where C depends on B. A migration replaces subtree [A, B] with [A, D] and
// maps B to D. C remains in the list, but its dependency must be rewritten from B to D, producing the prepared value
// C′.
//
// StateMigrationTransaction:
//
//	RootURN:                  A
//	PriorSubtree:             [A, B]
//	ResultSubtree:            [A, D]
//	SuccessorURNs:            {B: D}
//	PreparedPriorResources:   [P, A, D, C′]
//	RetainedResourceRewrites: {C: C′}
//	retainedRewriteIndices:   {C: 3}
//
// Persistence first records the transaction's prepared post-migration state, which contains C′ in this example. If
// persistence returns an error, the migration commit does not run and Deployment.prev.Resources remains unchanged.
// After persistence succeeds, the migration commit copies C′'s rewritten fields into the original C object, sets
// Deployment.prev.Resources to [P, A, D, C], rebuilds the deployment's indexes, and publishes a stateMigrationRewrite
// for state materialized later in the update. The persisted and in-memory resource values are equivalent, but the
// in-memory state preserves C's pointer. A preview skips persistence and performs the same in-memory commit.
type StateMigrationTransaction struct {
	// RootURN is the current URN derived for the registration that supplied the migration callbacks.
	RootURN resource.URN

	// PriorSubtree is the pre-migration resource subtree passed to the callbacks, in Deployment.prev.Resources order.
	// Its entries are live pointers from Deployment.prev.Resources.
	PriorSubtree []*pkgresource.State

	// ResultSubtree is the final, reference-normalized replacement subtree produced by the callback chain, in
	// callback-result order.
	ResultSubtree []*pkgresource.State

	// SuccessorURNs maps every URN present in PriorSubtree but absent from ResultSubtree directly to its final
	// successor URN in ResultSubtree.
	SuccessorURNs map[resource.URN]resource.URN

	// PreparedPriorResources is the complete, integrity-checked post-migration replacement for
	// Deployment.prev.Resources.
	PreparedPriorResources []*pkgresource.State

	// RetainedResourceRewrites maps each retained live pointer whose references changed to its prepared value in
	// PreparedPriorResources.
	RetainedResourceRewrites map[*pkgresource.State]*pkgresource.State

	// retainedRewriteIndices maps each retained live pointer to the index of its prepared value in
	// PreparedPriorResources.
	retainedRewriteIndices map[*pkgresource.State]int

	// currentResourceRewrites maps each changed live resource already present in Deployment.news or Deployment.reads
	// to its prepared value. The migration commit copies the rewritten fields into the live object so those maps retain
	// the same pointer.
	currentResourceRewrites map[*pkgresource.State]*pkgresource.State
}

// A StateMigrationTransaction prepares rewrites for states whose final values are already available, but some states
// acquire their final values only after the migration has committed. These include states returned by in-flight
// refresh or import continuations, Step.Apply, RegisterResourceOutputs, and provider-published view steps. Their work
// may have captured references to predecessor URNs before the migration, so the engine rewrites those references when
// the states re-enter the deployment.
//
// Registration and read states pass through the same boundary because their goals may also contain references
// captured before the migration. References that already use successor URNs are unchanged.
//
// stateMigrationRewrite is the part of a committed transaction retained for this later normalization. successorURNs
// rewrites logical references; successorIdentities supplies the Custom and ID information needed to rebuild typed
// resource references and provider references with the successor's physical identity.
type stateMigrationRewrite struct {
	rootURN             resource.URN
	successorURNs       map[resource.URN]resource.URN
	successorIdentities map[resource.URN]stateMigrationSuccessorIdentity
}

// stateMigrationSuccessorIdentity is the physical identity needed to rebuild resource and provider references whose
// URNs are rewritten to a migration successor.
type stateMigrationSuccessorIdentity struct {
	custom bool
	id     resource.ID
}

func newStateMigrationRewrite(
	rootURN resource.URN,
	successors map[resource.URN]resource.URN,
	successorStates []*pkgresource.State,
) *stateMigrationRewrite {
	successorURNs := make(map[resource.URN]resource.URN, len(successors))
	maps.Copy(successorURNs, successors)
	return &stateMigrationRewrite{
		rootURN:             rootURN,
		successorURNs:       successorURNs,
		successorIdentities: stateMigrationSuccessorIdentities(successorStates),
	}
}

// applyToResource rewrites references in state using the successor URNs and identities retained for one committed
// migration.
func (rewrite *stateMigrationRewrite) applyToResource(state *pkgresource.State) (*pkgresource.State, error) {
	rewritten, err := rewriteStateMigrationReferences(
		[]*pkgresource.State{state}, rewrite.successorURNs, rewrite.successorIdentities)
	if err != nil {
		return nil, err
	}
	return rewritten[0], nil
}

// RewriteResources rewrites references in states to the transaction's successor URNs. Snapshot managers use this
// while preparing states produced earlier in the update.
func (transaction *StateMigrationTransaction) RewriteResources(
	states []*pkgresource.State,
) ([]*pkgresource.State, error) {
	return rewriteStateMigrationReferences(
		states, transaction.SuccessorURNs, stateMigrationSuccessorIdentities(transaction.ResultSubtree))
}

// RewriteResourcesInPlace rewrites states while preserving their pointer and lock identities.
func (transaction *StateMigrationTransaction) RewriteResourcesInPlace(states []*pkgresource.State) error {
	rewritten, err := transaction.RewriteResources(states)
	if err != nil {
		return err
	}
	for i, state := range states {
		if rewritten[i] == state {
			continue
		}
		state.Lock.Lock()
		applyStateMigrationReferenceRewrite(state, rewritten[i])
		state.Lock.Unlock()
	}
	return nil
}

// applyStateMigrationReferenceRewrite copies precisely the fields changed by reference rewriting. The caller must
// synchronize access to state.
func applyStateMigrationReferenceRewrite(state, fixed *pkgresource.State) {
	state.Parent = fixed.Parent
	state.Dependencies = fixed.Dependencies
	state.PropertyDependencies = fixed.PropertyDependencies
	state.DeletedWith = fixed.DeletedWith
	state.ReplaceWith = fixed.ReplaceWith
	state.ViewOf = fixed.ViewOf
	state.Provider = fixed.Provider
	state.Inputs = fixed.Inputs
	state.Outputs = fixed.Outputs
	state.ReplacementTrigger = fixed.ReplacementTrigger
}

// validateStateMigrationAccounting checks that a single migration invocation accounted for every resource it was
// handed: each input resource must either remain in the returned state or name a returned successor. This prevents
// omission from becoming an implicit "forget" operation.
func validateStateMigrationAccounting(
	urn resource.URN, oldSet, newSet []apitype.ResourceV3,
	successors map[resource.URN]resource.URN,
) error {
	errorf := func(format string, args ...any) error {
		prefix := fmt.Sprintf("state migration for %s: ", urn)
		return fmt.Errorf(prefix+format, args...)
	}

	oldStates := make(map[resource.URN]apitype.ResourceV3, len(oldSet))
	for _, res := range oldSet {
		if !res.URN.IsValid() {
			return errorf("received a resource with invalid URN %q", res.URN)
		}
		if _, exists := oldStates[res.URN]; exists {
			return errorf("received duplicate resource %s", res.URN)
		}
		oldStates[res.URN] = res
	}
	newStates := make(map[resource.URN]apitype.ResourceV3, len(newSet))
	for _, res := range newSet {
		if !res.URN.IsValid() {
			return errorf("returned a resource with invalid URN %q", res.URN)
		}
		if len(res.Aliases) != 0 {
			return errorf("returned resource %s with aliases; state migration callbacks must express renames with "+
				"successor mappings instead", res.URN)
		}
		if _, exists := newStates[res.URN]; exists {
			return errorf("returned duplicate resource %s", res.URN)
		}
		if res.Type != res.URN.Type() {
			return errorf("returned resource %s with type %s, which does not match its URN type %s",
				res.URN, res.Type, res.URN.Type())
		}
		newStates[res.URN] = res
	}

	for source, target := range successors {
		if source == "" || target == "" {
			return errorf("returned an empty successor mapping %q -> %q", source, target)
		}
		if _, ok := oldStates[source]; !ok {
			return errorf("returned successor for resource %s, which is not part of the prior state", source)
		}
		if _, ok := newStates[source]; ok {
			return errorf("resource %s is both returned and assigned successor %s", source, target)
		}
		if _, ok := newStates[target]; !ok {
			return errorf("successor %s for resource %s is not present in the returned state", target, source)
		}
	}

	for oldURN, old := range oldStates {
		newState, retained := newStates[oldURN]
		if !retained {
			if _, ok := successors[oldURN]; !ok {
				return errorf("did not account for resource %s: it must either be returned in the new state or "+
					"have a successor", oldURN)
			}
			continue
		}

		if old.Custom != newState.Custom {
			return errorf("resource %s changed between custom and component state without changing URN", oldURN)
		}
		if old.Custom && old.ID != newState.ID {
			return errorf("resource %s changed ID from %q to %q without changing URN", oldURN, old.ID, newState.ID)
		}
	}
	return nil
}

// finalStateMigrationSuccessors composes mappings returned by chained callbacks and validates them against the
// original and final state sets. For example, if two callbacks replace A with B and then B with C, it returns:
//
//	originalToFinal: {A: C}
//	allToFinal:      {A: C, B: C}
//
// originalToFinal is safe to apply to resources outside the migrated subtree because it contains only resources that
// were present in the original subtree and removed from the final one. allToFinal is used to normalize the callback
// result, whose references may still mention an intermediate resource such as B. It must not be applied outside the
// migrated subtree because an intermediate URN may also identify an unrelated resource in the deployment.
func finalStateMigrationSuccessors(
	original, final []apitype.ResourceV3, all map[resource.URN]resource.URN,
) (map[resource.URN]resource.URN, map[resource.URN]resource.URN, error) {
	allToFinal := make(map[resource.URN]resource.URN, len(all))
	for source := range all {
		resolved, err := resolveStateMigrationSuccessor(source, all)
		if err != nil {
			return nil, nil, err
		}
		allToFinal[source] = resolved
	}

	finalURNs := make(map[resource.URN]bool, len(final))
	for _, state := range final {
		finalURNs[state.URN] = true
	}
	for source, target := range allToFinal {
		if finalURNs[source] {
			return nil, nil, fmt.Errorf("resource %s is present in the final state and also has successor %s",
				source, target)
		}
		if !finalURNs[target] {
			return nil, nil, fmt.Errorf("successor %s for resource %s is not present in the final state", target, source)
		}
	}

	originalToFinal := make(map[resource.URN]resource.URN)
	for _, state := range original {
		if finalURNs[state.URN] {
			continue
		}
		target, ok := allToFinal[state.URN]
		if !ok {
			return nil, nil, fmt.Errorf("did not account for resource %s: it must either be returned in the final state or "+
				"have a successor", state.URN)
		}
		originalToFinal[state.URN] = target
	}
	return originalToFinal, allToFinal, nil
}

func resolveStateMigrationSuccessor(
	urn resource.URN, successors map[resource.URN]resource.URN,
) (resource.URN, error) {
	seen := make(map[resource.URN]bool)
	for {
		if seen[urn] {
			return "", fmt.Errorf("successor mappings contain a cycle at %s", urn)
		}
		seen[urn] = true
		next, ok := successors[urn]
		if !ok {
			return urn, nil
		}
		urn = next
	}
}

func stateMigrationSuccessorIdentities(
	states []*pkgresource.State,
) map[resource.URN]stateMigrationSuccessorIdentity {
	identities := make(map[resource.URN]stateMigrationSuccessorIdentity, len(states))
	for _, state := range states {
		identities[state.URN] = stateMigrationSuccessorIdentity{custom: state.Custom, id: state.ID}
	}
	return identities
}

func rewriteStateMigrationSuccessor(
	urn resource.URN, successors map[resource.URN]resource.URN,
) resource.URN {
	if urn == "" {
		return ""
	}
	if successor, ok := successors[urn]; ok {
		return successor
	}
	return urn
}

// rewriteStateMigrationReferences rewrites every structural and property reference whose URN appears in successorURNs.
// It returns a copy of each changed state and preserves the original pointer for unchanged states. successorIdentities
// supplies the Custom and ID information needed to rebuild typed resource and provider references with the successor's
// physical identity. When multiple sources resolve to one successor, dependency lists are deduplicated.
func rewriteStateMigrationReferences(
	states []*pkgresource.State,
	successorURNs map[resource.URN]resource.URN,
	successorIdentities map[resource.URN]stateMigrationSuccessorIdentity,
) ([]*pkgresource.State, error) {
	if len(successorURNs) == 0 {
		return states, nil
	}

	fixURN := func(urn resource.URN) resource.URN {
		return rewriteStateMigrationSuccessor(urn, successorURNs)
	}
	rewriteURNs := func(urns []resource.URN) []resource.URN {
		if len(urns) == 0 {
			return urns
		}
		result := make([]resource.URN, 0, len(urns))
		seen := make(map[resource.URN]bool, len(urns))
		for _, urn := range urns {
			fixed := fixURN(urn)
			if !seen[fixed] {
				seen[fixed] = true
				result = append(result, fixed)
			}
		}
		return result
	}

	var rewritePropertyValue func(resource.PropertyValue) resource.PropertyValue
	rewritePropertyMap := func(properties resource.PropertyMap) resource.PropertyMap {
		if properties == nil {
			return nil
		}
		result := make(resource.PropertyMap, len(properties))
		for key, value := range properties {
			result[key] = rewritePropertyValue(value)
		}
		return result
	}
	rewritePropertyValue = func(value resource.PropertyValue) resource.PropertyValue {
		switch {
		case value.IsArray():
			array := value.ArrayValue()
			result := make([]resource.PropertyValue, len(array))
			for i, element := range array {
				result[i] = rewritePropertyValue(element)
			}
			return resource.NewProperty(result)
		case value.IsObject():
			return resource.NewProperty(rewritePropertyMap(value.ObjectValue()))
		case value.IsComputed():
			return resource.MakeComputed(rewritePropertyValue(value.Input().Element))
		case value.IsOutput():
			output := value.OutputValue()
			output.Element = rewritePropertyValue(output.Element)
			output.Dependencies = rewriteURNs(output.Dependencies)
			return resource.NewProperty(output)
		case value.IsSecret():
			return resource.MakeSecret(rewritePropertyValue(value.SecretValue().Element))
		case value.IsResourceReference():
			ref := value.ResourceReferenceValue()
			fixed := fixURN(ref.URN)
			if fixed != ref.URN {
				ref.URN = fixed
				ref.Name = fixed.Name()
				ref.Type = string(fixed.Type())
				ref.PackageVersion = ""
				if identity, ok := successorIdentities[fixed]; ok {
					if identity.custom {
						ref.ID = resource.NewProperty(string(identity.id))
					} else {
						ref.ID = resource.NewNullProperty()
					}
				}
			}
			return resource.NewProperty(ref)
		default:
			return value
		}
	}

	result := make([]*pkgresource.State, len(states))
	for i, state := range states {
		fixed := state.Copy()
		fixed.Parent = fixURN(fixed.Parent)
		fixed.Dependencies = rewriteURNs(fixed.Dependencies)
		if fixed.PropertyDependencies != nil {
			propertyDependencies := make(map[resource.PropertyKey][]resource.URN, len(fixed.PropertyDependencies))
			for key, dependencies := range fixed.PropertyDependencies {
				propertyDependencies[key] = rewriteURNs(dependencies)
			}
			fixed.PropertyDependencies = propertyDependencies
		}
		fixed.DeletedWith = fixURN(fixed.DeletedWith)
		fixed.ReplaceWith = rewriteURNs(fixed.ReplaceWith)
		fixed.ViewOf = fixURN(fixed.ViewOf)
		if fixed.Provider != "" {
			ref, err := sdkproviders.ParseReference(fixed.Provider)
			if err != nil {
				return nil, fmt.Errorf("parsing provider reference %q: %w", fixed.Provider, err)
			}
			originalProviderURN := ref.URN()
			providerURN := fixURN(originalProviderURN)
			providerID := ref.ID()
			if providerURN != originalProviderURN {
				if identity, ok := successorIdentities[providerURN]; ok {
					providerID = identity.id
				}
			}
			providerRef, err := sdkproviders.NewReference(providerURN, providerID)
			if err != nil {
				return nil, fmt.Errorf("rewriting provider reference %q: %w", fixed.Provider, err)
			}
			fixed.Provider = providerRef.String()
		}
		fixed.Inputs = rewritePropertyMap(fixed.Inputs)
		fixed.Outputs = rewritePropertyMap(fixed.Outputs)
		fixed.ReplacementTrigger = resource.FromResourcePropertyValue(
			rewritePropertyValue(resource.ToResourcePropertyValue(fixed.ReplacementTrigger)))
		if stateMigrationReferenceFieldsEqual(fixed, state) {
			result[i] = state
		} else {
			result[i] = fixed
		}
	}
	return result, nil
}

// stateMigrationReferenceFieldsEqual reports whether the fields that can contain rewritten URNs are equal.
func stateMigrationReferenceFieldsEqual(a, b *pkgresource.State) bool {
	return a.Parent == b.Parent &&
		reflect.DeepEqual(a.Dependencies, b.Dependencies) &&
		reflect.DeepEqual(a.PropertyDependencies, b.PropertyDependencies) &&
		a.DeletedWith == b.DeletedWith &&
		reflect.DeepEqual(a.ReplaceWith, b.ReplaceWith) &&
		a.ViewOf == b.ViewOf &&
		a.Provider == b.Provider &&
		a.Inputs.DeepEquals(b.Inputs) &&
		a.Outputs.DeepEquals(b.Outputs) &&
		a.ReplacementTrigger.Equals(b.ReplacementTrigger)
}
