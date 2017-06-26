// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

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
