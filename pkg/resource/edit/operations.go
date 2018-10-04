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
	// not condemnedRes while keeping track of every resource's parent.
	var newSnapshot []*resource.State
	var children []*resource.State
	for _, res := range snapshot.Resources {
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

	// Otherwise, we're good to go.
	snapshot.Resources = newSnapshot
	return nil
}

// UnprotectResource unprotects a resource.
func UnprotectResource(_ *deploy.Snapshot, res *resource.State) {
	res.Protect = false
}

// LocateResource locates a resource with the given URN within the given snapshot. If the resource exists and is
// uniquely named by the given URN, it is returned with a nil error. If the resource does not exist, LocateResource
// returns nil. If there exist multiple resources in the snapshot with the given URN, an instance of
// AmbiguousResourceError is returned with the full list of ambiguous resources attached to it.
func LocateResource(snap *deploy.Snapshot, urn resource.URN) (*resource.State, error) {
	contract.Require(snap != nil, "snap")

	urnMap := make(map[resource.URN][]*resource.State)
	for _, res := range snap.Resources {
		urnMap[res.URN] = append(urnMap[res.URN], res)
	}

	resources := urnMap[urn]
	switch {
	case len(resources) == 0:
		return nil, nil
	case len(resources) == 1:
		return resources[0], nil
	case len(resources) > 1:
		return nil, AmbiguousResourceError{URN: urn, Resources: resources}
	default:
		contract.Failf("unreachable")
		return nil, nil
	}
}
