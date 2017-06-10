// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package deployment

import (
	"github.com/pulumi/lumi/pkg/compiler/core"
	"github.com/pulumi/lumi/pkg/resource"
	"github.com/pulumi/lumi/pkg/tokens"
)

// Snapshot is a view of a collection of resources in an environment at a point in time.  It describes resources; their
// IDs, names, and properties; their dependencies; and more.  A snapshot is a diffable entity and can be used to create
// or apply an infrastructure deployment plan in order to make reality match the snapshot state.
type Snapshot struct {
	Namespace tokens.QName      // the namespace target being deployed into.
	Pkg       tokens.Package    // the package from which this snapshot came.
	Args      core.Args         // the arguments used to compile this package.
	Resources []*resource.State // fetches all resources and their associated states.
}

// NewSnapshot creates a snapshot from the given arguments.  The resources must be in topologically sorted order.
func NewSnapshot(ns tokens.QName, pkg tokens.Package, args core.Args, resources []*resource.State) *Snapshot {
	return &Snapshot{
		Namespace: ns,
		Pkg:       pkg,
		Args:      args,
		Resources: resources,
	}
}
