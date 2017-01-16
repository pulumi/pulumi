// Copyright 2016 Marapongo, Inc. All rights reserved.

package encoding

import (
	//"errors"
	"fmt"
	"reflect"

	"github.com/marapongo/mu/pkg/pack/ast"
	//"github.com/marapongo/mu/pkg/pack/symbols"
	"github.com/marapongo/mu/pkg/util/contract"
)

func completeBlock(tree object, block *ast.Block) error {
	contract.Assert(block != nil)
	contract.Assert(len(block.Statements) == 0)
	stmts, err := fieldObject(tree, reflect.TypeOf(ast.Block{}), "statements", true)
	if err != nil {
		return err
	}
	if stmts != nil {
		if block.Statements, err = decodeBlockStatements(*stmts); err != nil {
			return err
		}
	}
	return nil
}

func decodeBlockStatements(tree object) ([]ast.Statement, error) {
	var stmts []ast.Statement
	for i, v := range tree {
		s, err := asObject(v, reflect.TypeOf(ast.Block{}), fmt.Sprintf("statements[%v]", i))
		if err != nil {
			return nil, err
		}
		stmt, err := decodeStatement(*s)
		if err != nil {
			return nil, err
		}
		stmts = append(stmts, stmt)
	}
	return stmts, nil
}

func decodeStatement(tree object) (ast.Statement, error) {
	k, err := fieldString(tree, reflect.TypeOf(ast.Statement(nil)), "kind", true)
	if err != nil {
		return nil, err
	}
	if k != nil {
		kind := ast.NodeKind(*k)
		switch kind {
		// Blocks
		case ast.BlockKind:
			return decodeBlock(tree)

		// Local variables
		case ast.LocalVariableDeclarationKind:
			return decodeLocalVariableDeclaration(tree)

		// Try/catch/finally
		case ast.TryCatchFinallyKind:
			return decodeTryCatchFinally(tree)

		// Branches
		case ast.BreakStatementKind:
			return decodeBreakStatement(tree)
		case ast.ContinueStatementKind:
			return decodeContinueStatement(tree)
		case ast.IfStatementKind:
			return decodeIfStatement(tree)
		case ast.LabeledStatementKind:
			return decodeLabeledStatement(tree)
		case ast.ReturnStatementKind:
			return decodeReturnStatement(tree)
		case ast.ThrowStatementKind:
			return decodeThrowStatement(tree)
		case ast.WhileStatementKind:
			return decodeWhileStatement(tree)

		// Miscellaneous
		case ast.EmptyStatementKind:
			return decodeEmptyStatement(tree)
		case ast.MultiStatementKind:
			return decodeMultiStatement(tree)
		case ast.ExpressionStatementKind:
			return decodeExpressionStatement(tree)

		default:
			contract.FailMF("Unrecognized Statement kind: %v\n", kind)
		}
	}
	return nil, nil
}

func decodeBlock(tree object) (*ast.Block, error) {
	// Block has some common metadata that can be decoded using tag-directed decoding, but its statements are
	// polymorphic and so must be decoded by hand.
	var block ast.Block
	if err := decode(tree, &block); err != nil {
		return nil, err
	}
	if err := completeBlock(tree, &block); err != nil {
		return nil, err
	}
	return &block, nil
}

func decodeLocalVariableDeclaration(tree object) (*ast.LocalVariableDeclaration, error) {
	// LocalVariableDeclaration is a simple struct, so we can rely entirely on tag-directed decoding.
	var local ast.LocalVariableDeclaration
	if err := decode(tree, &local); err != nil {
		return nil, err
	}
	return &local, nil
}

func decodeTryCatchFinally(tree object) (*ast.TryCatchFinally, error) {
	return nil, nil
}

func decodeBreakStatement(tree object) (*ast.BreakStatement, error) {
	// BreakStatement is a simple struct, so we can rely entirely on tag-directed decoding.
	var stmt ast.BreakStatement
	if err := decode(tree, &stmt); err != nil {
		return nil, err
	}
	return &stmt, nil
}

func decodeContinueStatement(tree object) (*ast.ContinueStatement, error) {
	// ContinueStatement is a simple struct, so we can rely entirely on tag-directed decoding.
	var stmt ast.ContinueStatement
	if err := decode(tree, &stmt); err != nil {
		return nil, err
	}
	return &stmt, nil
}

func decodeIfStatement(tree object) (*ast.IfStatement, error) {
	return nil, nil
}

func decodeLabeledStatement(tree object) (*ast.LabeledStatement, error) {
	return nil, nil
}

func decodeReturnStatement(tree object) (*ast.ReturnStatement, error) {
	return nil, nil
}

func decodeThrowStatement(tree object) (*ast.ThrowStatement, error) {
	return nil, nil
}

func decodeWhileStatement(tree object) (*ast.WhileStatement, error) {
	return nil, nil
}

func decodeEmptyStatement(tree object) (*ast.EmptyStatement, error) {
	return nil, nil
}

func decodeMultiStatement(tree object) (*ast.EmptyStatement, error) {
	return nil, nil
}

func decodeExpressionStatement(tree object) (*ast.EmptyStatement, error) {
	return nil, nil
}
