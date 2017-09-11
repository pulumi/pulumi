// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package lumidl

import (
	"go/types"
	"reflect"
	"strings"

	"github.com/pulumi/pulumi-fabric/pkg/resource/idl"
	"github.com/pulumi/pulumi-fabric/pkg/util/contract"
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
	if IsResource(obj, t) {
		return true
	}
	spec, _ := IsSpecial(obj)
	return spec
}

// IsResource returns true if a type is a special IDL resource.
func IsResource(obj *types.TypeName, t types.Type) bool {
	contract.Assert(obj != nil)

	// If this is a resource type itself, then we're done.
	if IsSpecialResource(obj) {
		return true
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
					if IsSpecialResource(named.Obj()) {
						return true
					}
				}
			}
		}
	}
	return false
}

type SpecialType int

const (
	NotSpecialType = iota
	SpecialResourceType
	SpecialAssetType
	SpecialArchiveType
)

var (
	idlArchiveType  = reflect.TypeOf(idl.Archive{})
	idlAssetType    = reflect.TypeOf(idl.Asset{})
	idlResourceType = reflect.TypeOf(idl.Resource{})
)

// pkgMatch compares two packages.  If the first is a vendored version of match, it still returns true.
func pkgMatch(pkg string, match string) bool {
	ix := strings.LastIndex(pkg, match)
	return ix != -1 && ix+len(match) == len(pkg)
}

func IsSpecial(obj *types.TypeName) (bool, SpecialType) {
	if obj != nil && pkgMatch(obj.Pkg().Path(), idlResourceType.PkgPath()) {
		switch obj.Name() {
		case idlArchiveType.Name():
			return true, SpecialArchiveType
		case idlAssetType.Name():
			return true, SpecialAssetType
		case idlResourceType.Name():
			return true, SpecialResourceType
		}
	}
	return false, NotSpecialType
}

func IsSpecialResource(obj *types.TypeName) bool {
	spec, kind := IsSpecial(obj)
	return (spec && kind == SpecialResourceType)
}
