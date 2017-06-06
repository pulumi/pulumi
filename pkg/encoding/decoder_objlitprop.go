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

func decodeObjectLiteralProperty(m mapper.Mapper, obj map[string]interface{}) (ast.ObjectLiteralProperty, error) {
	k, err := mapper.FieldString(obj, reflect.TypeOf((*ast.ObjectLiteralProperty)(nil)).Elem(), "kind", true)
	if err != nil {
		return nil, err
	}
	if k != nil {
		kind := ast.NodeKind(*k)
		switch kind {
		case ast.ObjectLiteralNamedPropertyKind:
			return decodeObjectLiteralNamedProperty(m, obj)
		case ast.ObjectLiteralComputedPropertyKind:
			return decodeObjectLiteralComputedProperty(m, obj)
		default:
			contract.Failf("Unrecognized ObjectLiteralProperty kind: %v\n%v\n", kind, obj)
		}
	}
	return nil, nil
}

func decodeObjectLiteralNamedProperty(m mapper.Mapper,
	obj map[string]interface{}) (*ast.ObjectLiteralNamedProperty, error) {
	var p ast.ObjectLiteralNamedProperty
	if err := m.Decode(obj, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

func decodeObjectLiteralComputedProperty(m mapper.Mapper,
	obj map[string]interface{}) (*ast.ObjectLiteralComputedProperty, error) {
	var p ast.ObjectLiteralComputedProperty
	if err := m.Decode(obj, &p); err != nil {
		return nil, err
	}
	return &p, nil
}
