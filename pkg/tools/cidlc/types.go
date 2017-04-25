// Copyright 2017 Pulumi, Inc. All rights reserved.

package cidlc

import (
	"go/types"
	"reflect"

	"github.com/pulumi/coconut/pkg/resource/idl"
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

var (
	idlResourceType      = reflect.TypeOf(idl.Resource{})
	idlNamedResourceType = reflect.TypeOf(idl.NamedResource{})
)

// IsResource checks whether a type is a special IDL resource.  If yes, it returns true for the first boolean, and the
// second boolean indicates whether the resource is named or not.
func IsResource(obj *types.TypeName, t types.Type) (bool, bool) {
	// If this is a resource type itself, then we're done.
	if isres, isname := isResourceObj(obj); isres {
		return isres, isname
	}

	if s, is := t.(*types.Struct); is {
		// Otherwise, it's a resource if it has an embedded resource field.
		for i := 0; i < s.NumFields(); i++ {
			fld := s.Field(i)
			if fld.Anonymous() {
				if named, ok := fld.Type().(*types.Named); ok {
					if isres, isname := isResourceObj(named.Obj()); isres {
						return isres, isname
					}
				}
			}
		}
	}
	return false, false
}

func isResourceObj(obj *types.TypeName) (bool, bool) {
	if obj != nil && obj.Pkg().Path() == idlResourceType.PkgPath() {
		switch obj.Name() {
		case idlResourceType.Name():
			return true, false
		case idlNamedResourceType.Name():
			return true, true
		}
	}
	return false, false
}
