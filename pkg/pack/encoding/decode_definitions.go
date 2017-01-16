// Copyright 2016 Marapongo, Inc. All rights reserved.

package encoding

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/marapongo/mu/pkg/pack/ast"
	"github.com/marapongo/mu/pkg/pack/symbols"
	"github.com/marapongo/mu/pkg/util/contract"
)

// completeModule finishes the decoding process for an ast.Module object.  Because a Module's members are polymorphic --
// that is, there can be many kinds, each switching on the AST kind -- we require a custom decoding process.
func completeModule(tree object, mod *ast.Module) error {
	contract.Assert(mod.Members == nil)
	members, err := fieldObject(tree, reflect.TypeOf(ast.Module{}), "members", true)
	if err != nil {
		return err
	}
	if members != nil {
		if mod.Members, err = decodeModuleMembers(*members); err != nil {
			return err
		}
	}
	return nil
}

// decodeModuleMembers decodes a module's members.  Members are polymorphic, requiring custom decoding.
func decodeModuleMembers(tree object) (*ast.ModuleMembers, error) {
	members := make(ast.ModuleMembers)
	for k, v := range tree {
		vobj, err := asObject(v, reflect.TypeOf(ast.Module{}), fmt.Sprintf("members[%v]", k))
		if err != nil {
			return nil, err
		}
		if members[symbols.Token(k)], err = decodeModuleMember(*vobj); err != nil {
			return nil, err
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
	members, err := fieldObject(tree, reflect.TypeOf(ast.Class{}), "members", true)
	if err != nil {
		return nil, err
	}
	if members != nil {
		if class.Members, err = decodeClassMembers(*members); err != nil {
			return nil, err
		}
	}

	return &class, nil
}

// decodeClassMembers decodes a module's members.  Members are polymorphic, requiring custom decoding.
func decodeClassMembers(tree object) (*ast.ClassMembers, error) {
	members := make(ast.ClassMembers)
	for k, v := range tree {
		vobj, err := asObject(v, reflect.TypeOf(ast.Class{}), fmt.Sprintf("members[%v]", k))
		if err != nil {
			return nil, err
		}
		if members[symbols.Token(k)], err = decodeClassMember(*vobj); err != nil {
			return nil, err
		}
	}
	return &members, nil
}

func decodeClassMember(tree object) (ast.ClassMember, error) {
	kind, err := fieldString(tree, reflect.TypeOf(ast.ClassMember(nil)), "kind", true)
	if err != nil {
		return nil, err
	}
	if kind != nil {
		switch *kind {
		case "ClassProperty":
			return decodeClassProperty(tree)
		case "ClassMethod":
			return decodeClassMethod(tree)
		default:
			contract.FailMF("Unrecognized ClassMember kind: %v\n", *kind)
		}
	}
	return nil, nil
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
