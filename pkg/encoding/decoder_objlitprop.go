// Copyright 2017 Pulumi, Inc. All rights reserved.

package encoding

import (
	"reflect"

	"github.com/pulumi/lumi/pkg/compiler/ast"
	"github.com/pulumi/lumi/pkg/util/contract"
	"github.com/pulumi/lumi/pkg/util/mapper"
)

func decodeObjectLiteralProperty(m mapper.Mapper, tree mapper.Object) (ast.ObjectLiteralProperty, error) {
	k, err := mapper.FieldString(tree, reflect.TypeOf((*ast.ObjectLiteralProperty)(nil)).Elem(), "kind", true)
	if err != nil {
		return nil, err
	}
	if k != nil {
		kind := ast.NodeKind(*k)
		switch kind {
		case ast.ObjectLiteralNamedPropertyKind:
			return decodeObjectLiteralNamedProperty(m, tree)
		case ast.ObjectLiteralComputedPropertyKind:
			return decodeObjectLiteralComputedProperty(m, tree)
		default:
			contract.Failf("Unrecognized ObjectLiteralProperty kind: %v\n%v\n", kind, tree)
		}
	}
	return nil, nil
}

func decodeObjectLiteralNamedProperty(m mapper.Mapper,
	tree mapper.Object) (*ast.ObjectLiteralNamedProperty, error) {
	var p ast.ObjectLiteralNamedProperty
	if err := m.Decode(tree, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

func decodeObjectLiteralComputedProperty(m mapper.Mapper,
	tree mapper.Object) (*ast.ObjectLiteralComputedProperty, error) {
	var p ast.ObjectLiteralComputedProperty
	if err := m.Decode(tree, &p); err != nil {
		return nil, err
	}
	return &p, nil
}
