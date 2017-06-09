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
	"github.com/golang/glog"

	"github.com/pulumi/lumi/pkg/tokens"
	"github.com/pulumi/lumi/pkg/util/contract"
)

// resource is a structure containing state associated with a resource.  This resource may have been serialized and
// deserialized, or snapshotted from a live graph of resource objects.  The value's state is not, however, associated
// with any runtime objects in memory that may be actively involved in ongoing computations.
type resource struct {
	id      ID          // the resource's unique ID, assigned by the resource provider (or blank if uncreated).
	urn     URN         // the resource's object urn, a human-friendly, unique name for the resource.
	t       tokens.Type // the resource's type.
	inputs  PropertyMap // the resource's input properties (as specified by the program).
	outputs PropertyMap // the resource's output properties (as specified by the resource provider).
}

// NewResource creates a new resource value from existing resource state information.
func NewResource(id ID, urn URN, t tokens.Type, inputs PropertyMap, outputs PropertyMap) Resource {
	if inputs == nil {
		inputs = make(PropertyMap)
	}
	if outputs == nil {
		outputs = make(PropertyMap)
	}
	return &resource{
		id:      id,
		urn:     urn,
		t:       t,
		inputs:  inputs,
		outputs: outputs,
	}
}

func (r *resource) ID() ID               { return r.id }
func (r *resource) URN() URN             { return r.urn }
func (r *resource) Type() tokens.Type    { return r.t }
func (r *resource) Inputs() PropertyMap  { return r.inputs }
func (r *resource) Outputs() PropertyMap { return r.outputs }

func (r *resource) SetID(id ID) {
	contract.Requiref(!HasID(r), "id", "empty")
	glog.V(9).Infof("Assigning ID=%v to resource w/ URN=%v", id, r.urn)
	r.id = id
}

func (r *resource) SetURN(m URN) {
	contract.Requiref(!HasURN(r), "urn", "empty")
	r.urn = m
}
