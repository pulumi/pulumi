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
)

// Resource is an instance of a resource with an ID, type, and bag of state.
type Resource interface {
	ID() ID               // the resource's unique ID assigned by the provider (or blank if uncreated).
	SetID(id ID)          // assignes an ID to this resource, for those under creation.
	URN() URN             // the resource's object urn, a human-friendly, unique name for the resource.
	SetURN(m URN)         // assignes a URN to this resource, for those under creation.
	Type() tokens.Type    // the resource's type.
	Inputs() PropertyMap  // the resource's input properties (as specified by the program).
	Outputs() PropertyMap // the resource's output properties (as specified by the resource provider).
}

// State is returned when an error has occurred during a resource provider operation.  It indicates whether the
// operation could be rolled back cleanly (OK).  If not, it means the resource was left in an indeterminate state.
type State int

const (
	StateOK State = iota
	StateUnknown
)

// HasID returns true if the given resource has been assigned an ID.
func HasID(r Resource) bool {
	return r.ID() != ""
}

// HasURN returns true if the given resource has been assigned a URN.
func HasURN(r Resource) bool {
	return r.URN() != ""
}

// CopyOutputs copies all output properties from a src resource to the instance.
func CopyOutputs(src Resource, dst Resource) {
	src.Outputs().ShallowCloneInto(dst.Outputs())
}

// ShallowClone clones a resource object so that any modifications to it are not reflected in the original.  Note that
// the property map is only shallowly cloned so any mutations deep within it may get reflected in the original.
func ShallowClone(r Resource) Resource {
	return &resource{
		id:      r.ID(),
		urn:     r.URN(),
		t:       r.Type(),
		inputs:  r.Inputs().ShallowClone(),
		outputs: r.Outputs().ShallowClone(),
	}
}
