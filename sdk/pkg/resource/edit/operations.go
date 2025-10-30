// Copyright 2016-2023, Pulumi Corporation.
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
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/pkg/v3/resource/graph"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
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

// RenameStack changes the `stackName` component of every URN in a deployment. In addition, it rewrites the name of
// the root Stack resource itself. May optionally change the project/package name as well.
func RenameStack(deployment *apitype.DeploymentV3, newName tokens.StackName, newProject tokens.PackageName) error {
	contract.Requiref(deployment != nil, "deployment", "must not be nil")

	rewriteUrn := func(u resource.URN) resource.URN {
		project := u.Project()
		if newProject != "" {
			project = newProject
		}

		// The pulumi:pulumi:Stack resource's name component is of the form `<project>-<stack>` so we want
		// to rename the name portion as well.
		if u.QualifiedType() == resource.RootStackType {
			return resource.NewURN(newName.Q(), project, "", u.QualifiedType(), string(tokens.QName(project)+"-"+newName.Q()))
		}

		return resource.NewURN(tokens.QName(newName.String()), project, "", u.QualifiedType(), u.Name())
	}

	rewriteState := func(res *apitype.ResourceV3) {
		contract.Assertf(res != nil, "resource state must not be nil")

		res.URN = rewriteUrn(res.URN)

		if res.Parent != "" {
			res.Parent = rewriteUrn(res.Parent)
		}

		for depIdx, dep := range res.Dependencies {
			res.Dependencies[depIdx] = rewriteUrn(dep)
		}

		for _, propDeps := range res.PropertyDependencies {
			for depIdx, dep := range propDeps {
				propDeps[depIdx] = rewriteUrn(dep)
			}
		}

		if res.DeletedWith != "" {
			res.DeletedWith = rewriteUrn(res.DeletedWith)
		}

		if res.Provider != "" {
			providerRef, err := providers.ParseReference(res.Provider)
			contract.AssertNoErrorf(err, "failed to parse provider reference from validated checkpoint")

			providerRef, err = providers.NewReference(rewriteUrn(providerRef.URN()), providerRef.ID())
			contract.AssertNoErrorf(err, "failed to generate provider reference from valid reference")

			res.Provider = providerRef.String()
		}
	}

	for i := range deployment.Resources {
		rewriteState(&deployment.Resources[i])
	}

	for i := range deployment.PendingOperations {
		rewriteState(&deployment.PendingOperations[i].Resource)
	}

	return nil
}
