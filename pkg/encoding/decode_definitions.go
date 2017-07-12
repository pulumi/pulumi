// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package encoding

import (
	"reflect"

	"github.com/pulumi/lumi/pkg/compiler/ast"
	"github.com/pulumi/lumi/pkg/util/contract"
	"github.com/pulumi/lumi/pkg/util/mapper"
)

func decodeModuleMember(m mapper.Mapper, obj map[string]interface{}) (ast.ModuleMember, error) {
	k, err := mapper.FieldString(obj, reflect.TypeOf((*ast.ModuleMember)(nil)).Elem(), "kind", true)
	if err != nil {
		return nil, err
	}
	if k != nil {
		kind := ast.NodeKind(*k)
		switch kind {
		case ast.ClassKind:
			return decodeClass(m, obj)
		case ast.ModulePropertyKind:
			return decodeModuleProperty(m, obj)
		case ast.ModuleMethodKind:
			return decodeModuleMethod(m, obj)
		default:
			contract.Failf("Unrecognized ModuleMember kind: %v\n", kind)
		}
	}
	return nil, nil
}

func decodeClass(m mapper.Mapper, obj map[string]interface{}) (*ast.Class, error) {
	var class ast.Class
	if err := m.Decode(obj, &class); err != nil {
		return nil, err
	}
	return &class, nil
}

func decodeClassMember(m mapper.Mapper, obj map[string]interface{}) (ast.ClassMember, error) {
	k, err := mapper.FieldString(obj, reflect.TypeOf((*ast.ClassMember)(nil)).Elem(), "kind", true)
	if err != nil {
		return nil, err
	}
	if k != nil {
		kind := ast.NodeKind(*k)
		switch kind {
		case ast.ClassPropertyKind:
			return decodeClassProperty(m, obj)
		case ast.ClassMethodKind:
			return decodeClassMethod(m, obj)
		default:
			contract.Failf("Unrecognized ClassMember kind: %v\n", kind)
		}
	}
	return nil, nil
}

func decodeClassProperty(m mapper.Mapper, obj map[string]interface{}) (*ast.ClassProperty, error) {
	var prop ast.ClassProperty
	if err := m.Decode(obj, &prop); err != nil {
		return nil, err
	}
	return &prop, nil
}

func decodeClassMethod(m mapper.Mapper, obj map[string]interface{}) (*ast.ClassMethod, error) {
	var meth ast.ClassMethod
	if err := m.Decode(obj, &meth); err != nil {
		return nil, err
	}
	return &meth, nil
}

func decodeModuleProperty(m mapper.Mapper, obj map[string]interface{}) (*ast.ModuleProperty, error) {
	var prop ast.ModuleProperty
	if err := m.Decode(obj, &prop); err != nil {
		return nil, err
	}
	return &prop, nil
}

func decodeModuleMethod(m mapper.Mapper, obj map[string]interface{}) (*ast.ModuleMethod, error) {
	var meth ast.ModuleMethod
	if err := m.Decode(obj, &meth); err != nil {
		return nil, err
	}
	return &meth, nil
}
