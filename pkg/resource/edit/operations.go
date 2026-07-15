// Copyright 2016, Pulumi Corporation.
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

package edit

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/graph"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// OperationFunc is the type of functions that edit resources within a snapshot. The edits are made in-place to the
// given snapshot and pertain to the specific passed-in resource.
type OperationFunc func(*deploy.Snapshot, *resource.State) error

// DeleteResource deletes a given resource from the snapshot, if it is possible to do so.
//
// If targetDependents is true, dependents will also be deleted. Otherwise an error
// instance of `ResourceHasDependenciesError` will be returned.
//
// If non-nil, onProtected will be called on all protected resources planed for deletion.
//
// If a resource is marked protected after onProtected is called, an error instance of
// `ResourceHasDependenciesError` will be returned.
func DeleteResource(
	snapshot *deploy.Snapshot, condemnedRes *resource.State,
	onProtected func(*resource.State) error, targetDependents bool,
) error {
	contract.Requiref(snapshot != nil, "snapshot", "must not be nil")
	contract.Requiref(condemnedRes != nil, "condemnedRes", "must not be nil")

	handleProtected := func(res *resource.State) error {
		if !res.Protect {
			return nil
		}
		var err error
		if onProtected != nil {
			err = onProtected(res)
		}
		if err == nil && res.Protect {
			err = ResourceProtectedError{res}
		}
		return err
	}

	if err := handleProtected(condemnedRes); err != nil {
		return err
	}

	var numSameURN int
	for _, res := range snapshot.Resources {
		if res.URN != condemnedRes.URN {
			continue
		}
		numSameURN++
	}
	isUniqueURN := numSameURN <= 1

	deleteSet := make(map[resource.URN][]*resource.State)
	dg := graph.NewDependencyGraph(snapshot.Resources)

	deps := dg.OnlyDependsOn(condemnedRes)
	if len(deps) != 0 {
		if !targetDependents {
			return ResourceHasDependenciesError{Condemned: condemnedRes, Dependencies: deps}
		}
		for _, dep := range deps {
			if err := handleProtected(dep); err != nil {
				return err
			}
			deleteSet[dep.URN] = append(deleteSet[dep.URN], dep)
		}
	}

	// If there are no resources that depend on condemnedRes, iterate through the snapshot and keep everything that's
	// not condemnedRes.
	newSnapshot := slice.Prealloc[*resource.State](len(snapshot.Resources))
	var children []*resource.State
search:
	for _, res := range snapshot.Resources {
		if res == condemnedRes {
			// Skip condemned resource.
			continue
		}

		for _, v := range deleteSet[res.URN] {
			if v == res {
				continue search
			}
		}

		// While iterating, keep track of the set of resources that are parented to our
		// condemned resource. This acts as a check on DependingOn, preventing a bug from
		// introducing state corruption.
		if res.Parent == condemnedRes.URN {
			children = append(children, res)
		}

		newSnapshot = append(newSnapshot, res)
	}

	// If condemnedRes is unique and there exists a resource that is the child of condemnedRes,
	// we can't delete it.
	contract.Assertf(!isUniqueURN || len(children) == 0, "unexpected children in resource dependency list")

	// Otherwise, we're good to go. Writing the new resource list into the snapshot persists the mutations that we have
	// made above.
	snapshot.Resources = newSnapshot
	return nil
}

// LocateResource returns all resources in the given snapshot that have the given URN.
func LocateResource(snap *deploy.Snapshot, urn resource.URN) []*resource.State {
	// If there is no snapshot then return no resources
	if snap == nil {
		return nil
	}

	var resources []*resource.State
	for _, res := range snap.Resources {
		if res.URN == urn {
			resources = append(resources, res)
		}
	}

	return resources
}

// RenameStackOptions controls how RenameStack rewrites URNs.
type RenameStackOptions struct {
	// OldName filters rewrites to URNs that already reference this stack. Required.
	OldName tokens.StackName
	// OldProject filters rewrites to URNs that already reference this project. Empty matches any project;
	// callers should pass the stack's project, which legacy refs carry only inside their URNs.
	OldProject tokens.PackageName
	// Force degrades provider-reference rewrite failures to warnings.
	Force bool
	// WarningWriter receives Force warnings. Nil discards warnings.
	WarningWriter io.Writer
}

type stackRenamer struct {
	oldName    tokens.StackName
	oldProject tokens.PackageName
	newName    tokens.StackName
	newProject tokens.PackageName
}

func (r stackRenamer) renameURN(u resource.URN) resource.URN {
	if u == "" {
		return u
	}
	if !r.oldName.IsEmpty() && u.Stack() != r.oldName.Q() {
		return u
	}
	if r.oldProject != "" && u.Project() != r.oldProject {
		return u
	}

	project := u.Project()
	if r.newProject != "" {
		project = r.newProject
	}

	// The pulumi:pulumi:Stack resource's name component is of the form `<project>-<stack>`.
	if u.QualifiedType() == resource.RootStackType {
		return resource.NewURN(r.newName.Q(), project, "", u.QualifiedType(),
			string(tokens.QName(project)+"-"+r.newName.Q()))
	}

	return resource.NewURN(r.newName.Q(), project, "", u.QualifiedType(), u.Name())
}

func renameProviderReference(
	provider string,
	resURN resource.URN,
	renamer stackRenamer,
	opts RenameStackOptions,
) (string, error) {
	if provider == "" {
		return provider, nil
	}
	ref, err := providers.ParseReference(provider)
	if err != nil {
		if opts.Force {
			w := opts.WarningWriter
			if w == nil {
				w = io.Discard
			}
			fmt.Fprintf(w, "Warning: parsing provider reference for %q: %v\n", resURN, err)
			return provider, nil
		}
		return "", fmt.Errorf("parsing provider reference for %q: %w", resURN, err)
	}

	newURN := renamer.renameURN(ref.URN())
	// Preserve byte-for-byte provider refs when unchanged. NewReference normalizes empty IDs.
	if newURN == ref.URN() {
		return provider, nil
	}
	newRef, err := providers.NewReference(newURN, ref.ID())
	if err != nil {
		if opts.Force {
			w := opts.WarningWriter
			if w == nil {
				w = io.Discard
			}
			fmt.Fprintf(w, "Warning: rebuilding provider reference for %q: %v\n", resURN, err)
			return provider, nil
		}
		return "", fmt.Errorf("rebuilding provider reference for %q: %w", resURN, err)
	}
	return newRef.String(), nil
}

func renameSerializedPropertyMap(m map[string]any, renamer stackRenamer) {
	if m == nil {
		return
	}
	for k, v := range m {
		m[k] = renameSerializedPropertyValue(v, renamer)
	}
}

func renameSerializedPropertyValue(v any, renamer stackRenamer) any {
	switch v := v.(type) {
	case []any:
		for i := range v {
			v[i] = renameSerializedPropertyValue(v[i], renamer)
		}
		return v
	case *apitype.SecretV1:
		renameSerializedSecretString(&v.Plaintext, renamer)
		renameSerializedSecretString(&v.Ciphertext, renamer)
		return v
	case map[string]any:
		if sig, hasSig := v[resource.SigKey]; hasSig {
			switch sig {
			case resource.ResourceReferenceSig:
				if urn, ok := v["urn"].(string); ok {
					v["urn"] = string(renamer.renameURN(resource.URN(urn)))
				}
				return v
			case resource.SecretSig:
				renameSerializedSecret(v, "plaintext", renamer)
				renameSerializedSecret(v, "ciphertext", renamer)
				return v
			case resource.OutputValueSig:
				// DeploymentV3 flattens Output values today, but keep this for serialized forms that preserve them.
				if value, ok := v["value"]; ok {
					v["value"] = renameSerializedPropertyValue(value, renamer)
				}
				if deps, ok := v["dependencies"].([]any); ok {
					for i, dep := range deps {
						if urn, ok := dep.(string); ok {
							deps[i] = string(renamer.renameURN(resource.URN(urn)))
						}
					}
				}
				return v
			}
		}
		for k, elem := range v {
			v[k] = renameSerializedPropertyValue(elem, renamer)
		}
		return v
	default:
		return v
	}
}

func renameSerializedSecret(m map[string]any, key string, renamer stackRenamer) {
	raw, ok := m[key].(string)
	if !ok {
		return
	}
	renameSerializedSecretString(&raw, renamer)
	m[key] = raw
}

func renameSerializedSecretString(raw *string, renamer stackRenamer) {
	if raw == nil || *raw == "" {
		return
	}
	var value any
	if err := json.Unmarshal([]byte(*raw), &value); err != nil {
		return
	}
	value = renameSerializedPropertyValue(value, renamer)
	bytes, err := json.Marshal(value)
	if err != nil {
		return
	}
	*raw = string(bytes)
}

// RenameStack changes the `stackName` component of every matching URN in a deployment, including aliases,
// dependencies, provider references, and serialized ResourceReference URNs in property values. Secret contents
// are rewritten when present as plaintext JSON. In addition, it rewrites the name of the root Stack resource itself.
// May optionally change the project/package name as well.
func RenameStack(
	deployment *apitype.DeploymentV3,
	newName tokens.StackName,
	newProject tokens.PackageName,
	opts RenameStackOptions,
) error {
	contract.Requiref(deployment != nil, "deployment", "must not be nil")
	contract.Requiref(!opts.OldName.IsEmpty(), "opts.OldName", "must be set")

	renamer := stackRenamer{
		oldName:    opts.OldName,
		oldProject: opts.OldProject,
		newName:    newName,
		newProject: newProject,
	}

	rewriteState := func(res *apitype.ResourceV3) error {
		contract.Assertf(res != nil, "resource state must not be nil")

		res.URN = renamer.renameURN(res.URN)

		if res.Parent != "" {
			res.Parent = renamer.renameURN(res.Parent)
		}

		for depIdx, dep := range res.Dependencies {
			res.Dependencies[depIdx] = renamer.renameURN(dep)
		}

		for _, propDeps := range res.PropertyDependencies {
			for depIdx, dep := range propDeps {
				propDeps[depIdx] = renamer.renameURN(dep)
			}
		}

		if res.DeletedWith != "" {
			res.DeletedWith = renamer.renameURN(res.DeletedWith)
		}

		for i, replaceWith := range res.ReplaceWith {
			res.ReplaceWith[i] = renamer.renameURN(replaceWith)
		}

		for i, alias := range res.Aliases {
			res.Aliases[i] = renamer.renameURN(alias)
		}

		if res.ViewOf != "" {
			res.ViewOf = renamer.renameURN(res.ViewOf)
		}

		var err error
		res.Provider, err = renameProviderReference(res.Provider, res.URN, renamer, opts)
		if err != nil {
			return err
		}

		renameSerializedPropertyMap(res.Inputs, renamer)
		renameSerializedPropertyMap(res.Outputs, renamer)
		res.ReplacementTrigger = renameSerializedPropertyValue(res.ReplacementTrigger, renamer)

		return nil
	}

	for i := range deployment.Resources {
		if err := rewriteState(&deployment.Resources[i]); err != nil {
			return err
		}
	}

	for i := range deployment.PendingOperations {
		if err := rewriteState(&deployment.PendingOperations[i].Resource); err != nil {
			return err
		}
	}

	return nil
}
