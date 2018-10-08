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

package edit

import (
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/resource/graph"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

// OperationFunc is the type of functions that edit resources within a snapshot. The edits are made in-place to the
// given snapshot and pertain to the specific passed-in resource.
type OperationFunc func(*deploy.Snapshot, *resource.State) error

// DeleteResource deletes a given resource from the snapshot, if it is possible to do so. A resource can only be deleted
// from a stack if there do not exist any resources that depend on it or descend from it. If such a resource does exist,
// DeleteResource will return an error instance of `ResourceHasDependenciesError`.
func DeleteResource(snapshot *deploy.Snapshot, condemnedRes *resource.State) error {
	contract.Require(snapshot != nil, "snapshot")
	contract.Require(condemnedRes != nil, "state")

	dg := graph.NewDependencyGraph(snapshot.Resources)
	dependencies := dg.DependingOn(condemnedRes)
	if len(dependencies) != 0 {
		return ResourceHasDependenciesError{Condemned: condemnedRes, Dependencies: dependencies}
	}

	// If there are no resources that depend on condemnedRes, iterate through the snapshot and keep everything that's
	// not condemnedRes.
	var newSnapshot []*resource.State
	var children []*resource.State
	for _, res := range snapshot.Resources {
		// While iterating, keep track of the set of resources that are parented to our condemned resource. We'll only
		// actually perform the deletion if this set is empty, otherwise it is not legal to delete the resource.
		if res.Parent == condemnedRes.URN {
			children = append(children, res)
		}

		if res != condemnedRes {
			newSnapshot = append(newSnapshot, res)
		}
	}

	// If there exists a resource that is the child of condemnedRes, we can't delete it.
	if len(children) != 0 {
		return ResourceHasDependenciesError{Condemned: condemnedRes, Dependencies: children}
	}

	// Otherwise, we're good to go. Writing the new resource list into the snapshot persists the mutations that we have
	// made above.
	snapshot.Resources = newSnapshot
	return nil
}

// UnprotectResource unprotects a resource.
func UnprotectResource(_ *deploy.Snapshot, res *resource.State) error {
	res.Protect = false
	return nil
}

// LocateResource returns all resources in the given shapshot that have the given URN.
func LocateResource(snap *deploy.Snapshot, urn resource.URN) []*resource.State {
	contract.Require(snap != nil, "snap")

	var resources []*resource.State
	for _, res := range snap.Resources {
		if res.URN == urn {
			resources = append(resources, res)
		}
	}

	return resources
}
