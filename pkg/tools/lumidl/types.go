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

package lumidl

import (
	"go/types"
	"reflect"

	"github.com/pulumi/lumi/pkg/resource/idl"
	"github.com/pulumi/lumi/pkg/util/contract"
)

func IsPrimitive(t types.Type) bool {
	if basic, isbasic := t.(*types.Basic); isbasic {
		switch basic.Kind() {
		case types.Bool, types.Float64, types.String:
			return true
		}
	}
	return false
}

// IsEntity checks whether a type is an entity that can be used by-reference (asset, resource, etc).
func IsEntity(obj *types.TypeName, t types.Type) bool {
	if res, _ := IsResource(obj, t); res {
		return true
	}
	spec, _ := IsSpecial(obj)
	return spec
}

// IsResource checks whether a type is a special IDL resource.  If yes, it returns true for the first boolean, and the
// second boolean indicates whether the resource is named or not.
func IsResource(obj *types.TypeName, t types.Type) (bool, bool) {
	contract.Assert(obj != nil)

	// If this is a resource type itself, then we're done.
	if isres, isname := IsSpecialResource(obj); isres {
		return isres, isname
	}

	// If a named type, fetch the underlying.
	if n, is := t.(*types.Named); is {
		t = n.Underlying()
	}

	if s, is := t.(*types.Struct); is {
		// Otherwise, it's a resource if it has an embedded resource field.
		for i := 0; i < s.NumFields(); i++ {
			fld := s.Field(i)
			if fld.Anonymous() {
				if named, ok := fld.Type().(*types.Named); ok {
					if isres, isname := IsSpecialResource(named.Obj()); isres {
						return isres, isname
					}
				}
			}
		}
	}
	return false, false
}

type SpecialType int

const (
	NotSpecialType = iota
	SpecialResourceType
	SpecialNamedResourceType
	SpecialAssetType
	SpecialArchiveType
)

var (
	idlArchiveType       = reflect.TypeOf(idl.Archive{})
	idlAssetType         = reflect.TypeOf(idl.Asset{})
	idlResourceType      = reflect.TypeOf(idl.Resource{})
	idlNamedResourceType = reflect.TypeOf(idl.NamedResource{})
)

func IsSpecial(obj *types.TypeName) (bool, SpecialType) {
	if obj != nil && obj.Pkg().Path() == idlResourceType.PkgPath() {
		switch obj.Name() {
		case idlArchiveType.Name():
			return true, SpecialArchiveType
		case idlAssetType.Name():
			return true, SpecialAssetType
		case idlResourceType.Name():
			return true, SpecialResourceType
		case idlNamedResourceType.Name():
			return true, SpecialNamedResourceType
		}
	}
	return false, NotSpecialType
}

func IsSpecialResource(obj *types.TypeName) (bool, bool) {
	spec, kind := IsSpecial(obj)
	isres := (spec && kind == SpecialResourceType)
	isnamed := (spec && kind == SpecialNamedResourceType)
	return isres || isnamed, isnamed
}
