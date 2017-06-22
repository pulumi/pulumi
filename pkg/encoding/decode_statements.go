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

func decodeStatement(m mapper.Mapper, obj map[string]interface{}) (ast.Statement, error) {
	k, err := mapper.FieldString(obj, reflect.TypeOf((*ast.Statement)(nil)).Elem(), "kind", true)
	if err != nil {
		return nil, err
	}
	if k != nil {
		kind := ast.NodeKind(*k)
		switch kind {
		// Imports
		case ast.ImportKind:
			return decodeImport(m, obj)

		// Blocks
		case ast.BlockKind:
			return decodeBlock(m, obj)

		// Local variables
		case ast.LocalVariableDeclarationKind:
			return decodeLocalVariableDeclaration(m, obj)

		// Try/catch/finally
		case ast.TryCatchFinallyKind:
			return decodeTryCatchFinally(m, obj)

		// Branches
		case ast.BreakStatementKind:
			return decodeBreakStatement(m, obj)
		case ast.ContinueStatementKind:
			return decodeContinueStatement(m, obj)
		case ast.IfStatementKind:
			return decodeIfStatement(m, obj)
		case ast.SwitchStatementKind:
			return decodeSwitchStatement(m, obj)
		case ast.LabeledStatementKind:
			return decodeLabeledStatement(m, obj)
		case ast.ReturnStatementKind:
			return decodeReturnStatement(m, obj)
		case ast.ThrowStatementKind:
			return decodeThrowStatement(m, obj)
		case ast.WhileStatementKind:
			return decodeWhileStatement(m, obj)
		case ast.ForStatementKind:
			return decodeForStatement(m, obj)

		// Miscellaneous
		case ast.EmptyStatementKind:
			return decodeEmptyStatement(m, obj)
		case ast.MultiStatementKind:
			return decodeMultiStatement(m, obj)
		case ast.ExpressionStatementKind:
			return decodeExpressionStatement(m, obj)

		default:
			contract.Failf("Unrecognized Statement kind: %v\n", kind)
		}
	}
	return nil, nil
}

func decodeImport(m mapper.Mapper, obj map[string]interface{}) (*ast.Import, error) {
	var imp ast.Import
	if err := m.Decode(obj, &imp); err != nil {
		return nil, err
	}
	return &imp, nil
}

func decodeBlock(m mapper.Mapper, obj map[string]interface{}) (*ast.Block, error) {
	var block ast.Block
	if err := m.Decode(obj, &block); err != nil {
		return nil, err
	}
	return &block, nil
}

func decodeLocalVariableDeclaration(m mapper.Mapper,
	obj map[string]interface{}) (*ast.LocalVariableDeclaration, error) {
	var local ast.LocalVariableDeclaration
	if err := m.Decode(obj, &local); err != nil {
		return nil, err
	}
	return &local, nil
}

func decodeTryCatchFinally(m mapper.Mapper, obj map[string]interface{}) (*ast.TryCatchFinally, error) {
	return nil, nil
}

func decodeBreakStatement(m mapper.Mapper, obj map[string]interface{}) (*ast.BreakStatement, error) {
	var stmt ast.BreakStatement
	if err := m.Decode(obj, &stmt); err != nil {
		return nil, err
	}
	return &stmt, nil
}

func decodeContinueStatement(m mapper.Mapper, obj map[string]interface{}) (*ast.ContinueStatement, error) {
	var stmt ast.ContinueStatement
	if err := m.Decode(obj, &stmt); err != nil {
		return nil, err
	}
	return &stmt, nil
}

func decodeIfStatement(m mapper.Mapper, obj map[string]interface{}) (*ast.IfStatement, error) {
	var stmt ast.IfStatement
	if err := m.Decode(obj, &stmt); err != nil {
		return nil, err
	}
	return &stmt, nil
}

func decodeSwitchStatement(m mapper.Mapper, obj map[string]interface{}) (*ast.SwitchStatement, error) {
	var stmt ast.SwitchStatement
	if err := m.Decode(obj, &stmt); err != nil {
		return nil, err
	}
	return &stmt, nil
}

func decodeLabeledStatement(m mapper.Mapper, obj map[string]interface{}) (*ast.LabeledStatement, error) {
	var stmt ast.LabeledStatement
	if err := m.Decode(obj, &stmt); err != nil {
		return nil, err
	}
	return &stmt, nil
}

func decodeReturnStatement(m mapper.Mapper, obj map[string]interface{}) (*ast.ReturnStatement, error) {
	var stmt ast.ReturnStatement
	if err := m.Decode(obj, &stmt); err != nil {
		return nil, err
	}
	return &stmt, nil
}

func decodeThrowStatement(m mapper.Mapper, obj map[string]interface{}) (*ast.ThrowStatement, error) {
	var stmt ast.ThrowStatement
	if err := m.Decode(obj, &stmt); err != nil {
		return nil, err
	}
	return &stmt, nil
}

func decodeWhileStatement(m mapper.Mapper, obj map[string]interface{}) (*ast.WhileStatement, error) {
	var stmt ast.WhileStatement
	if err := m.Decode(obj, &stmt); err != nil {
		return nil, err
	}
	return &stmt, nil
}

func decodeForStatement(m mapper.Mapper, obj map[string]interface{}) (*ast.ForStatement, error) {
	var stmt ast.ForStatement
	if err := m.Decode(obj, &stmt); err != nil {
		return nil, err
	}
	return &stmt, nil
}

func decodeEmptyStatement(m mapper.Mapper, obj map[string]interface{}) (*ast.EmptyStatement, error) {
	var stmt ast.EmptyStatement
	if err := m.Decode(obj, &stmt); err != nil {
		return nil, err
	}
	return &stmt, nil
}

func decodeMultiStatement(m mapper.Mapper, obj map[string]interface{}) (*ast.MultiStatement, error) {
	var stmt ast.MultiStatement
	if err := m.Decode(obj, &stmt); err != nil {
		return nil, err
	}
	return &stmt, nil
}

func decodeExpressionStatement(m mapper.Mapper, obj map[string]interface{}) (*ast.ExpressionStatement, error) {
	var stmt ast.ExpressionStatement
	if err := m.Decode(obj, &stmt); err != nil {
		return nil, err
	}
	return &stmt, nil
}
