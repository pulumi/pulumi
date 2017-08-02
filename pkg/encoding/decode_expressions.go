// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package encoding

import (
	"reflect"

	"github.com/pulumi/pulumi-fabric/pkg/compiler/ast"
	"github.com/pulumi/pulumi-fabric/pkg/util/contract"
	"github.com/pulumi/pulumi-fabric/pkg/util/mapper"
)

func decodeExpression(m mapper.Mapper, obj map[string]interface{}) (ast.Expression, error) {
	k, err := mapper.FieldString(obj, reflect.TypeOf((*ast.Expression)(nil)).Elem(), "kind", true)
	if err != nil {
		return nil, err
	}
	if k != nil {
		kind := ast.NodeKind(*k)
		switch kind {
		// Literals
		case ast.NullLiteralKind:
			return decodeNullLiteral(m, obj)
		case ast.BoolLiteralKind:
			return decodeBoolLiteral(m, obj)
		case ast.NumberLiteralKind:
			return decodeNumberLiteral(m, obj)
		case ast.StringLiteralKind:
			return decodeStringLiteral(m, obj)
		case ast.ArrayLiteralKind:
			return decodeArrayLiteral(m, obj)
		case ast.ObjectLiteralKind:
			return decodeObjectLiteral(m, obj)

		// Loads
		case ast.LoadLocationExpressionKind:
			return decodeLoadLocationExpression(m, obj)
		case ast.LoadDynamicExpressionKind:
			return decodeLoadDynamicExpression(m, obj)
		case ast.TryLoadDynamicExpressionKind:
			return decodeTryLoadDynamicExpression(m, obj)

		// Functions
		case ast.NewExpressionKind:
			return decodeNewExpression(m, obj)
		case ast.InvokeFunctionExpressionKind:
			return decodeInvokeFunctionExpression(m, obj)
		case ast.LambdaExpressionKind:
			return decodeLambdaExpression(m, obj)

		// Operators
		case ast.UnaryOperatorExpressionKind:
			return decodeUnaryOperatorExpression(m, obj)
		case ast.BinaryOperatorExpressionKind:
			return decodeBinaryOperatorExpression(m, obj)

		// Type testing
		case ast.CastExpressionKind:
			return decodeCastExpression(m, obj)
		case ast.IsInstExpressionKind:
			return decodeIsInstExpression(m, obj)
		case ast.TypeOfExpressionKind:
			return decodeTypeOfExpression(m, obj)

		// Miscellaneous
		case ast.ConditionalExpressionKind:
			return decodeConditionalExpression(m, obj)
		case ast.SequenceExpressionKind:
			return decodeSequenceExpression(m, obj)

		default:
			contract.Failf("Unrecognized Expression kind: %v\n%v\n", kind, obj)
		}
	}
	return nil, nil
}

func decodeNullLiteral(m mapper.Mapper, obj map[string]interface{}) (*ast.NullLiteral, error) {
	var lit ast.NullLiteral
	if err := m.Decode(obj, &lit); err != nil {
		return nil, err
	}
	return &lit, nil
}

func decodeBoolLiteral(m mapper.Mapper, obj map[string]interface{}) (*ast.BoolLiteral, error) {
	var lit ast.BoolLiteral
	if err := m.Decode(obj, &lit); err != nil {
		return nil, err
	}
	return &lit, nil
}

func decodeNumberLiteral(m mapper.Mapper, obj map[string]interface{}) (*ast.NumberLiteral, error) {
	var lit ast.NumberLiteral
	if err := m.Decode(obj, &lit); err != nil {
		return nil, err
	}
	return &lit, nil
}

func decodeStringLiteral(m mapper.Mapper, obj map[string]interface{}) (*ast.StringLiteral, error) {
	var lit ast.StringLiteral
	if err := m.Decode(obj, &lit); err != nil {
		return nil, err
	}
	return &lit, nil
}

func decodeArrayLiteral(m mapper.Mapper, obj map[string]interface{}) (*ast.ArrayLiteral, error) {
	var lit ast.ArrayLiteral
	if err := m.Decode(obj, &lit); err != nil {
		return nil, err
	}
	return &lit, nil
}

func decodeObjectLiteral(m mapper.Mapper, obj map[string]interface{}) (*ast.ObjectLiteral, error) {
	var lit ast.ObjectLiteral
	if err := m.Decode(obj, &lit); err != nil {
		return nil, err
	}
	return &lit, nil
}

func decodeLoadLocationExpression(m mapper.Mapper, obj map[string]interface{}) (*ast.LoadLocationExpression, error) {
	var expr ast.LoadLocationExpression
	if err := m.Decode(obj, &expr); err != nil {
		return nil, err
	}
	return &expr, nil
}

func decodeLoadDynamicExpression(m mapper.Mapper, obj map[string]interface{}) (*ast.LoadDynamicExpression, error) {
	var expr ast.LoadDynamicExpression
	if err := m.Decode(obj, &expr); err != nil {
		return nil, err
	}
	return &expr, nil
}

func decodeTryLoadDynamicExpression(m mapper.Mapper,
	obj map[string]interface{}) (*ast.TryLoadDynamicExpression, error) {
	var expr ast.TryLoadDynamicExpression
	if err := m.Decode(obj, &expr); err != nil {
		return nil, err
	}
	return &expr, nil
}

func decodeNewExpression(m mapper.Mapper, obj map[string]interface{}) (*ast.NewExpression, error) {
	var expr ast.NewExpression
	if err := m.Decode(obj, &expr); err != nil {
		return nil, err
	}
	return &expr, nil
}

func decodeInvokeFunctionExpression(m mapper.Mapper,
	obj map[string]interface{}) (*ast.InvokeFunctionExpression, error) {
	var expr ast.InvokeFunctionExpression
	if err := m.Decode(obj, &expr); err != nil {
		return nil, err
	}
	return &expr, nil
}

func decodeLambdaExpression(m mapper.Mapper, obj map[string]interface{}) (*ast.LambdaExpression, error) {
	var expr ast.LambdaExpression
	if err := m.Decode(obj, &expr); err != nil {
		return nil, err
	}
	return &expr, nil
}

func decodeUnaryOperatorExpression(m mapper.Mapper,
	obj map[string]interface{}) (*ast.UnaryOperatorExpression, error) {
	var expr ast.UnaryOperatorExpression
	if err := m.Decode(obj, &expr); err != nil {
		return nil, err
	}
	return &expr, nil
}

func decodeBinaryOperatorExpression(m mapper.Mapper,
	obj map[string]interface{}) (*ast.BinaryOperatorExpression, error) {
	var expr ast.BinaryOperatorExpression
	if err := m.Decode(obj, &expr); err != nil {
		return nil, err
	}
	return &expr, nil
}

func decodeCastExpression(m mapper.Mapper, obj map[string]interface{}) (*ast.CastExpression, error) {
	var expr ast.CastExpression
	if err := m.Decode(obj, &expr); err != nil {
		return nil, err
	}
	return &expr, nil
}

func decodeIsInstExpression(m mapper.Mapper, obj map[string]interface{}) (*ast.IsInstExpression, error) {
	var expr ast.IsInstExpression
	if err := m.Decode(obj, &expr); err != nil {
		return nil, err
	}
	return &expr, nil
}

func decodeTypeOfExpression(m mapper.Mapper, obj map[string]interface{}) (*ast.TypeOfExpression, error) {
	var expr ast.TypeOfExpression
	if err := m.Decode(obj, &expr); err != nil {
		return nil, err
	}
	return &expr, nil
}

func decodeConditionalExpression(m mapper.Mapper, obj map[string]interface{}) (*ast.ConditionalExpression, error) {
	var expr ast.ConditionalExpression
	if err := m.Decode(obj, &expr); err != nil {
		return nil, err
	}
	return &expr, nil
}

func decodeSequenceExpression(m mapper.Mapper, obj map[string]interface{}) (*ast.SequenceExpression, error) {
	var expr ast.SequenceExpression
	if err := m.Decode(obj, &expr); err != nil {
		return nil, err
	}
	return &expr, nil
}
