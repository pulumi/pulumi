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

package resource

import (
	"github.com/pulumi/lumi/pkg/tokens"
	"github.com/pulumi/lumi/pkg/util/contract"
)

// State is a structure containing state associated with a resource.  This resource may have been serialized and
// deserialized, or snapshotted from a live graph of resource objects.  The value's state is not, however, associated
// with any runtime objects in memory that may be actively involved in ongoing computations.
type State struct {
	t       tokens.Type // the resource's type.
	urn     URN         // the resource's object urn, a human-friendly, unique name for the resource.
	id      ID          // the resource's unique ID, assigned by the resource provider (or blank if uncreated).
	inputs  PropertyMap // the resource's input properties (as specified by the program).
	outputs PropertyMap // the resource's output properties (as specified by the resource provider).
}

var _ Resource = (*State)(nil)

// NewState creates a new resource value from existing resource state information.
func NewState(t tokens.Type, urn URN, id ID, inputs PropertyMap, outputs PropertyMap) *State {
	contract.Assert(t != "")
	contract.Assert(urn != "")
	contract.Assert(id != "")
	contract.Assert(inputs != nil)
	contract.Assert(outputs != nil)
	return &State{
		t:       t,
		urn:     urn,
		id:      id,
		inputs:  inputs,
		outputs: outputs,
	}
}

func (r *State) ID() ID               { return r.id }
func (r *State) URN() URN             { return r.urn }
func (r *State) Type() tokens.Type    { return r.t }
func (r *State) Inputs() PropertyMap  { return r.inputs }
func (r *State) Outputs() PropertyMap { return r.outputs }
