// Copyright 2016 Marapongo, Inc. All rights reserved.

// Because of the complex structure of the MuPack and MuIL metadata formats, we cannot rely on the standard JSON
// marshaling and unmarshaling routines.  Instead, we will need to do it mostly "by hand".  This package does that.
package encoding

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/marapongo/mu/pkg/encoding"
	"github.com/marapongo/mu/pkg/pack"
	"github.com/marapongo/mu/pkg/pack/ast"
	"github.com/marapongo/mu/pkg/pack/symbols"
	"github.com/marapongo/mu/pkg/util/contract"
)

// Decode unmarshals the entire contents of the given byte array into a Package object.
func Decode(m encoding.Marshaler, b []byte) (*pack.Package, error) {
	// First convert the whole contents of the metadata into a map.  Although it would be more efficient to walk the
	// token stream, token by token, this allows us to reuse existing YAML packages in addition to JSON ones.
	var tree object
	if err := m.Unmarshal(b, &tree); err != nil {
		return nil, err
	}
	return decodePackage(tree)
}

// decodePackage decodes the tree into a freshly allocated Package, or returns an error if something goes wrong.
func decodePackage(tree object) (*pack.Package, error) {
	var pack pack.Package

	// First use tag-directed decoding for the simple parts of the struct.
	if err := decode(tree, &pack); err != nil {
		return nil, err
	}

	// Assuming that worked, we must now decode each module's members explicitly.
	if pack.Modules != nil {
		modstree := tree["modules"].(map[string]interface{})
		for mname, mvalue := range *pack.Modules {
			modtree := modstree[string(mname)].(map[string]interface{})
			if err := completeModule(modtree, mvalue); err != nil {
				return nil, err
			}
		}
	}

	return &pack, nil
}

// completeModule finishes the decoding process for an ast.Module object.  Because a Module's members are polymorphic --
// that is, there can be many kinds, each switching on the AST kind -- we require a custom decoding process.
func completeModule(tree object, mod *ast.Module) error {
	contract.Assert(mod.Members == nil)
	if m, has := tree["members"]; has {
		if mm, ok := m.(map[string]interface{}); ok {
			var err error
			if mod.Members, err = decodeModuleMembers(mm); err != nil {
				return err
			}
		} else {
			return errWrongType(
				reflect.TypeOf(ast.Module{}), "members",
				reflect.TypeOf(make(map[string]interface{})), reflect.TypeOf(m))
		}
	}
	return nil
}

// decodeModuleMembers decodes a module's members.  Members are polymorphic, requiring custom decoding.
func decodeModuleMembers(tree object) (*ast.ModuleMembers, error) {
	members := make(ast.ModuleMembers)
	for k, v := range tree {
		if vobj, ok := v.(map[string]interface{}); ok {
			var err error
			if members[symbols.Token(k)], err = decodeModuleMember(vobj); err != nil {
				return nil, err
			}
		} else {
			return nil, errWrongType(
				reflect.TypeOf(ast.Module{}), fmt.Sprintf("members[%v]", k),
				reflect.TypeOf(map[string]interface{}{}), reflect.TypeOf(v))
		}
	}
	return &members, nil
}

func decodeModuleMember(tree object) (ast.ModuleMember, error) {
	if kind, has := tree["kind"]; has {
		if skind, ok := kind.(string); ok {
			switch skind {
			case "Class":
				return decodeClass(tree)
			case "Export":
				return decodeExport(tree)
			case "ModuleProperty":
				return decodeModuleProperty(tree)
			case "ModuleMethod":
				return decodeModuleMethod(tree)
			default:
				contract.FailMF("Unrecognized ModuleMember kind: %v\n", skind)
				return nil, nil
			}
		} else {
			return nil, errWrongType(
				reflect.TypeOf(ast.ModuleMember(nil)), "kind",
				reflect.TypeOf(""), reflect.TypeOf(kind))
		}
	} else {
		return nil, errors.New("Module member is missing required `kind` property")
	}
}

func decodeClass(tree object) (*ast.Class, error) {
	var class ast.Class
	if err := decode(tree, &class); err != nil {
		return nil, err
	}

	// Now decode the members by hand, since they are polymorphic.
	contract.Assert(class.Members == nil)
	if m, has := tree["members"]; has {
		if mm, ok := m.(map[string]interface{}); ok {
			var err error
			if class.Members, err = decodeClassMembers(mm); err != nil {
				return nil, err
			}
		} else {
			return nil, errWrongType(
				reflect.TypeOf(ast.Class{}), "members",
				reflect.TypeOf(make(map[string]interface{})), reflect.TypeOf(m))
		}
	}

	return &class, nil
}

// decodeClassMembers decodes a module's members.  Members are polymorphic, requiring custom decoding.
func decodeClassMembers(tree object) (*ast.ClassMembers, error) {
	members := make(ast.ClassMembers)
	for k, v := range tree {
		if vobj, ok := v.(map[string]interface{}); ok {
			var err error
			if members[symbols.Token(k)], err = decodeClassMember(vobj); err != nil {
				return nil, err
			}
		} else {
			return nil, errWrongType(
				reflect.TypeOf(ast.Class{}), fmt.Sprintf("members[%v]", k),
				reflect.TypeOf(map[string]interface{}{}), reflect.TypeOf(v))
		}
	}
	return &members, nil
}

func decodeClassMember(tree object) (ast.ClassMember, error) {
	if kind, has := tree["kind"]; has {
		if skind, ok := kind.(string); ok {
			switch skind {
			case "ClassProperty":
				return decodeClassProperty(tree)
			case "ClassMethod":
				return decodeClassMethod(tree)
			default:
				contract.FailMF("Unrecognized ClassMember kind: %v\n", skind)
				return nil, nil
			}
		} else {
			return nil, errWrongType(
				reflect.TypeOf(ast.ClassMember(nil)), "kind",
				reflect.TypeOf(""), reflect.TypeOf(kind))
		}
	} else {
		return nil, errors.New("Class member is missing required `kind` property")
	}
}

func decodeClassProperty(tree object) (*ast.ClassProperty, error) {
	// ClassProperty is a simple struct, so we can rely entirely on tag-directed decoding.
	var prop ast.ClassProperty
	if err := decode(tree, &prop); err != nil {
		return nil, err
	}
	return &prop, nil
}

func decodeClassMethod(tree object) (*ast.ClassMethod, error) {
	// First decode the simple parts of the method using tag-directed decoding.
	var meth ast.ClassMethod
	if err := decode(tree, &meth); err != nil {
		return nil, err
	}

	// Next, the body of the method requires an AST-like discriminated union, so we must do it explicitly.
	// TODO: do this.

	return &meth, nil
}

func decodeExport(tree object) (*ast.Export, error) {
	// Export is a simple struct, so we can rely entirely on tag-directed decoding.
	var export ast.Export
	if err := decode(tree, &export); err != nil {
		return nil, err
	}
	return &export, nil
}

func decodeModuleProperty(tree object) (*ast.ModuleProperty, error) {
	// ModuleProperty is a simple struct, so we can rely entirely on tag-directed decoding.
	var prop ast.ModuleProperty
	if err := decode(tree, &prop); err != nil {
		return nil, err
	}
	return &prop, nil
}

func decodeModuleMethod(tree object) (*ast.ModuleMethod, error) {
	// First decode the simple parts of the method using tag-directed decoding.
	var meth ast.ModuleMethod
	if err := decode(tree, &meth); err != nil {
		return nil, err
	}

	// Next, the body of the method requires an AST-like discriminated union, so we must do it explicitly.
	// TODO: do this.

	return &meth, nil
}
