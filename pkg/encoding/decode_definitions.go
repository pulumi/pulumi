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

package encoding

import (
	"reflect"

	"github.com/pulumi/lumi/pkg/compiler/ast"
	"github.com/pulumi/lumi/pkg/util/contract"
	"github.com/pulumi/lumi/pkg/util/mapper"
)

func decodeModuleMember(m mapper.Mapper, tree mapper.Object) (ast.ModuleMember, error) {
	k, err := mapper.FieldString(tree, reflect.TypeOf((*ast.ModuleMember)(nil)).Elem(), "kind", true)
	if err != nil {
		return nil, err
	}
	if k != nil {
		kind := ast.NodeKind(*k)
		switch kind {
		case ast.ClassKind:
			return decodeClass(m, tree)
		case ast.ModulePropertyKind:
			return decodeModuleProperty(m, tree)
		case ast.ModuleMethodKind:
			return decodeModuleMethod(m, tree)
		default:
			contract.Failf("Unrecognized ModuleMember kind: %v\n", kind)
		}
	}
	return nil, nil
}

func decodeClass(m mapper.Mapper, tree mapper.Object) (*ast.Class, error) {
	var class ast.Class
	if err := m.Decode(tree, &class); err != nil {
		return nil, err
	}
	return &class, nil
}

func decodeClassMember(m mapper.Mapper, tree mapper.Object) (ast.ClassMember, error) {
	k, err := mapper.FieldString(tree, reflect.TypeOf((*ast.ClassMember)(nil)).Elem(), "kind", true)
	if err != nil {
		return nil, err
	}
	if k != nil {
		kind := ast.NodeKind(*k)
		switch kind {
		case ast.ClassPropertyKind:
			return decodeClassProperty(m, tree)
		case ast.ClassMethodKind:
			return decodeClassMethod(m, tree)
		default:
			contract.Failf("Unrecognized ClassMember kind: %v\n", kind)
		}
	}
	return nil, nil
}

func decodeClassProperty(m mapper.Mapper, tree mapper.Object) (*ast.ClassProperty, error) {
	var prop ast.ClassProperty
	if err := m.Decode(tree, &prop); err != nil {
		return nil, err
	}
	return &prop, nil
}

func decodeClassMethod(m mapper.Mapper, tree mapper.Object) (*ast.ClassMethod, error) {
	var meth ast.ClassMethod
	if err := m.Decode(tree, &meth); err != nil {
		return nil, err
	}
	return &meth, nil
}

func decodeModuleProperty(m mapper.Mapper, tree mapper.Object) (*ast.ModuleProperty, error) {
	var prop ast.ModuleProperty
	if err := m.Decode(tree, &prop); err != nil {
		return nil, err
	}
	return &prop, nil
}

func decodeModuleMethod(m mapper.Mapper, tree mapper.Object) (*ast.ModuleMethod, error) {
	var meth ast.ModuleMethod
	if err := m.Decode(tree, &meth); err != nil {
		return nil, err
	}
	return &meth, nil
}
