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
	"github.com/pulumi/lumi/pkg/compiler/types/predef"
	"github.com/pulumi/lumi/pkg/eval/rt"
	"github.com/pulumi/lumi/pkg/tokens"
	"github.com/pulumi/lumi/pkg/util/contract"
)

// Object is a live resource object, connected to state that may change due to evaluation.
type Object interface {
	Resource
	Obj() *rt.Object
}

type object struct {
	urn URN        // the resource's object urn, a human-friendly, unique name for the resource.
	obj *rt.Object // the resource's live object reference.
}

// NewObject creates a new resource object out of the runtime object provided.  The context is used to resolve
// dependencies between resources and must contain all references that could be encountered.
func NewObject(obj *rt.Object) Object {
	contract.Assertf(predef.IsResourceType(obj.Type()), "Expected a resource type")
	return &object{obj: obj}
}

func (r *object) URN() URN          { return r.urn }
func (r *object) Obj() *rt.Object   { return r.obj }
func (r *object) Type() tokens.Type { return r.obj.Type().TypeToken() }

func (r *object) SetURN(m URN) {
	contract.Requiref(!HasURN(r), "urn", "empty")
	r.urn = m
}
