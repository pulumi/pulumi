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

package binder

import (
	"testing"

	"github.com/pulumi/lumi/pkg/compiler/ast"
	"github.com/pulumi/lumi/pkg/compiler/types"
	"github.com/pulumi/lumi/pkg/tokens"
	"github.com/stretchr/testify/assert"
)

func makeLocalVariable(name string) *ast.LocalVariable {
	return &ast.LocalVariable{
		DefinitionNode: ast.DefinitionNode{
			Name: &ast.Identifier{
				Ident: tokens.Name(name),
			},
		},
		VariableNode: ast.VariableNode{
			Type: &ast.TypeToken{
				Tok: types.Object.TypeToken(),
			},
		},
	}
}

func expressionRef(expr ast.Expression) *ast.Expression {
	return &expr
}

func TestFreeVars_Parameter(t *testing.T) {
	// function(foo) foo
	fun := ast.ModuleMethod{
		FunctionNode: ast.FunctionNode{
			Parameters: &[]*ast.LocalVariable{
				makeLocalVariable("foo"),
			},
			Body: &ast.ExpressionStatement{
				Expression: &ast.LoadLocationExpression{
					Name: &ast.Token{
						Tok: tokens.Token("foo"),
					},
				},
			},
		},
	}
	freeVars := FreeVars(&fun)
	assert.Len(t, freeVars, 0, "expected no free variables")
}

func TestFreeVars_LocalVariable(t *testing.T) {
	// function(foo) { var bar; foo; bar; baz; }
	fun := ast.ModuleMethod{
		FunctionNode: ast.FunctionNode{
			Parameters: &[]*ast.LocalVariable{
				makeLocalVariable("foo"),
			},
			Body: &ast.Block{
				Statements: []ast.Statement{
					&ast.LocalVariableDeclaration{
						Local: makeLocalVariable("bar"),
					},
					&ast.ExpressionStatement{
						Expression: &ast.LoadLocationExpression{
							Name: &ast.Token{
								Tok: tokens.Token("foo"),
							},
						},
					},
					&ast.ExpressionStatement{
						Expression: &ast.LoadLocationExpression{
							Name: &ast.Token{
								Tok: tokens.Token("bar"),
							},
						},
					},
					&ast.ExpressionStatement{
						Expression: &ast.LoadLocationExpression{
							Name: &ast.Token{
								Tok: tokens.Token("baz"),
							},
						},
					},
				},
			},
		},
	}
	freeVars := FreeVars(&fun)
	assert.Len(t, freeVars, 1, "expected one free variable")
	assert.Equal(t, tokens.Name("baz"), freeVars[0].Name())
}

func TestFreeVars_Member(t *testing.T) {
	// function(foo) foo.bar
	fun := ast.ModuleMethod{
		FunctionNode: ast.FunctionNode{
			Parameters: &[]*ast.LocalVariable{
				makeLocalVariable("foo"),
			},
			Body: &ast.ExpressionStatement{
				Expression: &ast.LoadLocationExpression{
					Object: expressionRef(&ast.LoadLocationExpression{
						Name: &ast.Token{
							Tok: tokens.Token("foo"),
						},
					}),
					Name: &ast.Token{
						Tok: tokens.Token("bar"),
					},
				},
			},
		},
	}
	freeVars := FreeVars(&fun)
	assert.Len(t, freeVars, 0, "expected no free variables")
}

func TestFreeVars_Lambda(t *testing.T) {
	// function(foo) ((bar) => bar)(foo)
	fun := ast.ModuleMethod{
		FunctionNode: ast.FunctionNode{
			Parameters: &[]*ast.LocalVariable{
				makeLocalVariable("foo"),
			},
			Body: &ast.ExpressionStatement{
				Expression: &ast.InvokeFunctionExpression{
					Function: &ast.LambdaExpression{
						FunctionNode: ast.FunctionNode{
							Parameters: &[]*ast.LocalVariable{
								makeLocalVariable("bar"),
							},
							Body: &ast.ExpressionStatement{
								Expression: &ast.LoadLocationExpression{
									Name: &ast.Token{
										Tok: tokens.Token("bar"),
									},
								},
							},
						},
					},
					CallExpressionNode: ast.CallExpressionNode{
						Arguments: &[]*ast.CallArgument{
							&ast.CallArgument{
								Expr: &ast.LoadLocationExpression{
									Name: &ast.Token{
										Tok: tokens.Token("foo"),
									},
								},
							},
						},
					},
				},
			},
		},
	}
	freeVars := FreeVars(&fun)
	assert.Len(t, freeVars, 0, "expected no free variables")
}
