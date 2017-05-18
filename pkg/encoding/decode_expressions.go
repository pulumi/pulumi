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

func decodeExpression(m mapper.Mapper, tree mapper.Object) (ast.Expression, error) {
	k, err := mapper.FieldString(tree, reflect.TypeOf((*ast.Expression)(nil)).Elem(), "kind", true)
	if err != nil {
		return nil, err
	}
	if k != nil {
		kind := ast.NodeKind(*k)
		switch kind {
		// Literals
		case ast.NullLiteralKind:
			return decodeNullLiteral(m, tree)
		case ast.BoolLiteralKind:
			return decodeBoolLiteral(m, tree)
		case ast.NumberLiteralKind:
			return decodeNumberLiteral(m, tree)
		case ast.StringLiteralKind:
			return decodeStringLiteral(m, tree)
		case ast.ArrayLiteralKind:
			return decodeArrayLiteral(m, tree)
		case ast.ObjectLiteralKind:
			return decodeObjectLiteral(m, tree)

		// Loads
		case ast.LoadLocationExpressionKind:
			return decodeLoadLocationExpression(m, tree)
		case ast.LoadDynamicExpressionKind:
			return decodeLoadDynamicExpression(m, tree)
		case ast.TryLoadDynamicExpressionKind:
			return decodeTryLoadDynamicExpression(m, tree)

		// Functions
		case ast.NewExpressionKind:
			return decodeNewExpression(m, tree)
		case ast.InvokeFunctionExpressionKind:
			return decodeInvokeFunctionExpression(m, tree)
		case ast.LambdaExpressionKind:
			return decodeLambdaExpression(m, tree)

		// Operators
		case ast.UnaryOperatorExpressionKind:
			return decodeUnaryOperatorExpression(m, tree)
		case ast.BinaryOperatorExpressionKind:
			return decodeBinaryOperatorExpression(m, tree)

		// Type testing
		case ast.CastExpressionKind:
			return decodeCastExpression(m, tree)
		case ast.IsInstExpressionKind:
			return decodeIsInstExpression(m, tree)
		case ast.TypeOfExpressionKind:
			return decodeTypeOfExpression(m, tree)

		// Miscellaneous
		case ast.ConditionalExpressionKind:
			return decodeConditionalExpression(m, tree)
		case ast.SequenceExpressionKind:
			return decodeSequenceExpression(m, tree)

		default:
			contract.Failf("Unrecognized Expression kind: %v\n%v\n", kind, tree)
		}
	}
	return nil, nil
}

func decodeNullLiteral(m mapper.Mapper, tree mapper.Object) (*ast.NullLiteral, error) {
	var lit ast.NullLiteral
	if err := m.Decode(tree, &lit); err != nil {
		return nil, err
	}
	return &lit, nil
}

func decodeBoolLiteral(m mapper.Mapper, tree mapper.Object) (*ast.BoolLiteral, error) {
	var lit ast.BoolLiteral
	if err := m.Decode(tree, &lit); err != nil {
		return nil, err
	}
	return &lit, nil
}

func decodeNumberLiteral(m mapper.Mapper, tree mapper.Object) (*ast.NumberLiteral, error) {
	var lit ast.NumberLiteral
	if err := m.Decode(tree, &lit); err != nil {
		return nil, err
	}
	return &lit, nil
}

func decodeStringLiteral(m mapper.Mapper, tree mapper.Object) (*ast.StringLiteral, error) {
	var lit ast.StringLiteral
	if err := m.Decode(tree, &lit); err != nil {
		return nil, err
	}
	return &lit, nil
}

func decodeArrayLiteral(m mapper.Mapper, tree mapper.Object) (*ast.ArrayLiteral, error) {
	var lit ast.ArrayLiteral
	if err := m.Decode(tree, &lit); err != nil {
		return nil, err
	}
	return &lit, nil
}

func decodeObjectLiteral(m mapper.Mapper, tree mapper.Object) (*ast.ObjectLiteral, error) {
	var lit ast.ObjectLiteral
	if err := m.Decode(tree, &lit); err != nil {
		return nil, err
	}
	return &lit, nil
}

func decodeLoadLocationExpression(m mapper.Mapper, tree mapper.Object) (*ast.LoadLocationExpression, error) {
	var expr ast.LoadLocationExpression
	if err := m.Decode(tree, &expr); err != nil {
		return nil, err
	}
	return &expr, nil
}

func decodeLoadDynamicExpression(m mapper.Mapper, tree mapper.Object) (*ast.LoadDynamicExpression, error) {
	var expr ast.LoadDynamicExpression
	if err := m.Decode(tree, &expr); err != nil {
		return nil, err
	}
	return &expr, nil
}

func decodeTryLoadDynamicExpression(m mapper.Mapper, tree mapper.Object) (*ast.TryLoadDynamicExpression, error) {
	var expr ast.TryLoadDynamicExpression
	if err := m.Decode(tree, &expr); err != nil {
		return nil, err
	}
	return &expr, nil
}

func decodeNewExpression(m mapper.Mapper, tree mapper.Object) (*ast.NewExpression, error) {
	var expr ast.NewExpression
	if err := m.Decode(tree, &expr); err != nil {
		return nil, err
	}
	return &expr, nil
}

func decodeInvokeFunctionExpression(m mapper.Mapper, tree mapper.Object) (*ast.InvokeFunctionExpression, error) {
	var expr ast.InvokeFunctionExpression
	if err := m.Decode(tree, &expr); err != nil {
		return nil, err
	}
	return &expr, nil
}

func decodeLambdaExpression(m mapper.Mapper, tree mapper.Object) (*ast.LambdaExpression, error) {
	var expr ast.LambdaExpression
	if err := m.Decode(tree, &expr); err != nil {
		return nil, err
	}
	return &expr, nil
}

func decodeUnaryOperatorExpression(m mapper.Mapper, tree mapper.Object) (*ast.UnaryOperatorExpression, error) {
	var expr ast.UnaryOperatorExpression
	if err := m.Decode(tree, &expr); err != nil {
		return nil, err
	}
	return &expr, nil
}

func decodeBinaryOperatorExpression(m mapper.Mapper, tree mapper.Object) (*ast.BinaryOperatorExpression, error) {
	var expr ast.BinaryOperatorExpression
	if err := m.Decode(tree, &expr); err != nil {
		return nil, err
	}
	return &expr, nil
}

func decodeCastExpression(m mapper.Mapper, tree mapper.Object) (*ast.CastExpression, error) {
	var expr ast.CastExpression
	if err := m.Decode(tree, &expr); err != nil {
		return nil, err
	}
	return &expr, nil
}

func decodeIsInstExpression(m mapper.Mapper, tree mapper.Object) (*ast.IsInstExpression, error) {
	var expr ast.IsInstExpression
	if err := m.Decode(tree, &expr); err != nil {
		return nil, err
	}
	return &expr, nil
}

func decodeTypeOfExpression(m mapper.Mapper, tree mapper.Object) (*ast.TypeOfExpression, error) {
	var expr ast.TypeOfExpression
	if err := m.Decode(tree, &expr); err != nil {
		return nil, err
	}
	return &expr, nil
}

func decodeConditionalExpression(m mapper.Mapper, tree mapper.Object) (*ast.ConditionalExpression, error) {
	var expr ast.ConditionalExpression
	if err := m.Decode(tree, &expr); err != nil {
		return nil, err
	}
	return &expr, nil
}

func decodeSequenceExpression(m mapper.Mapper, tree mapper.Object) (*ast.SequenceExpression, error) {
	var expr ast.SequenceExpression
	if err := m.Decode(tree, &expr); err != nil {
		return nil, err
	}
	return &expr, nil
}
