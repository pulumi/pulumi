// Copyright 2016 Marapongo, Inc. All rights reserved.

package encoding

import (
	"reflect"

	"github.com/marapongo/mu/pkg/pack/ast"
	"github.com/marapongo/mu/pkg/util/contract"
	"github.com/marapongo/mu/pkg/util/mapper"
)

func decodeStatement(m mapper.Mapper, tree mapper.Object) (ast.Statement, error) {
	k, err := mapper.FieldString(tree, reflect.TypeOf((*ast.Statement)(nil)).Elem(), "kind", true)
	if err != nil {
		return nil, err
	}
	if k != nil {
		kind := ast.NodeKind(*k)
		switch kind {
		// Blocks
		case ast.BlockKind:
			return decodeBlock(m, tree)

		// Local variables
		case ast.LocalVariableDeclarationKind:
			return decodeLocalVariableDeclaration(m, tree)

		// Try/catch/finally
		case ast.TryCatchFinallyKind:
			return decodeTryCatchFinally(m, tree)

		// Branches
		case ast.BreakStatementKind:
			return decodeBreakStatement(m, tree)
		case ast.ContinueStatementKind:
			return decodeContinueStatement(m, tree)
		case ast.IfStatementKind:
			return decodeIfStatement(m, tree)
		case ast.LabeledStatementKind:
			return decodeLabeledStatement(m, tree)
		case ast.ReturnStatementKind:
			return decodeReturnStatement(m, tree)
		case ast.ThrowStatementKind:
			return decodeThrowStatement(m, tree)
		case ast.WhileStatementKind:
			return decodeWhileStatement(m, tree)

		// Miscellaneous
		case ast.EmptyStatementKind:
			return decodeEmptyStatement(m, tree)
		case ast.MultiStatementKind:
			return decodeMultiStatement(m, tree)
		case ast.ExpressionStatementKind:
			return decodeExpressionStatement(m, tree)

		default:
			contract.Failf("Unrecognized Statement kind: %v\n", kind)
		}
	}
	return nil, nil
}

func decodeBlock(m mapper.Mapper, tree mapper.Object) (*ast.Block, error) {
	var block ast.Block
	if err := m.Decode(tree, &block); err != nil {
		return nil, err
	}
	return &block, nil
}

func decodeLocalVariableDeclaration(m mapper.Mapper, tree mapper.Object) (*ast.LocalVariableDeclaration, error) {
	var local ast.LocalVariableDeclaration
	if err := m.Decode(tree, &local); err != nil {
		return nil, err
	}
	return &local, nil
}

func decodeTryCatchFinally(m mapper.Mapper, tree mapper.Object) (*ast.TryCatchFinally, error) {
	return nil, nil
}

func decodeBreakStatement(m mapper.Mapper, tree mapper.Object) (*ast.BreakStatement, error) {
	var stmt ast.BreakStatement
	if err := m.Decode(tree, &stmt); err != nil {
		return nil, err
	}
	return &stmt, nil
}

func decodeContinueStatement(m mapper.Mapper, tree mapper.Object) (*ast.ContinueStatement, error) {
	var stmt ast.ContinueStatement
	if err := m.Decode(tree, &stmt); err != nil {
		return nil, err
	}
	return &stmt, nil
}

func decodeIfStatement(m mapper.Mapper, tree mapper.Object) (*ast.IfStatement, error) {
	var stmt ast.IfStatement
	if err := m.Decode(tree, &stmt); err != nil {
		return nil, err
	}
	return &stmt, nil
}

func decodeLabeledStatement(m mapper.Mapper, tree mapper.Object) (*ast.LabeledStatement, error) {
	var stmt ast.LabeledStatement
	if err := m.Decode(tree, &stmt); err != nil {
		return nil, err
	}
	return &stmt, nil
}

func decodeReturnStatement(m mapper.Mapper, tree mapper.Object) (*ast.ReturnStatement, error) {
	var stmt ast.ReturnStatement
	if err := m.Decode(tree, &stmt); err != nil {
		return nil, err
	}
	return &stmt, nil
}

func decodeThrowStatement(m mapper.Mapper, tree mapper.Object) (*ast.ThrowStatement, error) {
	var stmt ast.ThrowStatement
	if err := m.Decode(tree, &stmt); err != nil {
		return nil, err
	}
	return &stmt, nil
}

func decodeWhileStatement(m mapper.Mapper, tree mapper.Object) (*ast.WhileStatement, error) {
	var stmt ast.WhileStatement
	if err := m.Decode(tree, &stmt); err != nil {
		return nil, err
	}
	return &stmt, nil
}

func decodeEmptyStatement(m mapper.Mapper, tree mapper.Object) (*ast.EmptyStatement, error) {
	var stmt ast.EmptyStatement
	if err := m.Decode(tree, &stmt); err != nil {
		return nil, err
	}
	return &stmt, nil
}

func decodeMultiStatement(m mapper.Mapper, tree mapper.Object) (*ast.MultiStatement, error) {
	var stmt ast.MultiStatement
	if err := m.Decode(tree, &stmt); err != nil {
		return nil, err
	}
	return &stmt, nil
}

func decodeExpressionStatement(m mapper.Mapper, tree mapper.Object) (*ast.ExpressionStatement, error) {
	var stmt ast.ExpressionStatement
	if err := m.Decode(tree, &stmt); err != nil {
		return nil, err
	}
	return &stmt, nil
}
