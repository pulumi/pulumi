// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package binder

import (
	"testing"

	"github.com/pulumi/pulumi-fabric/pkg/compiler/ast"
	"github.com/pulumi/pulumi-fabric/pkg/compiler/types"
	"github.com/pulumi/pulumi-fabric/pkg/tokens"
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

func TestFreeVars_Dynamic(t *testing.T) {
	// function(foo) foo
	fun := ast.ModuleMethod{
		FunctionNode: ast.FunctionNode{
			Parameters: &[]*ast.LocalVariable{},
			Body: &ast.ExpressionStatement{
				Expression: &ast.LoadDynamicExpression{
					Name: &ast.StringLiteral{
						Value: "foo",
					},
				},
			},
		},
	}
	freeVars := FreeVars(&fun)
	assert.Len(t, freeVars, 1, "expected one free variable")
	assert.Equal(t, tokens.Name("foo"), freeVars[0].Name())
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
						Expression: &ast.TryLoadDynamicExpression{
							Name: &ast.StringLiteral{
								Value: "baz",
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

func TestFreeVars_LocalVariable_2(t *testing.T) {
	// function(foo) { {var bar;} foo; bar; baz; }
	fun := ast.ModuleMethod{
		FunctionNode: ast.FunctionNode{
			Parameters: &[]*ast.LocalVariable{
				makeLocalVariable("foo"),
			},
			Body: &ast.Block{
				Statements: []ast.Statement{
					&ast.MultiStatement{
						Statements: []ast.Statement{
							&ast.LocalVariableDeclaration{
								Local: makeLocalVariable("bar"),
							},
						},
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
						Expression: &ast.TryLoadDynamicExpression{
							Name: &ast.StringLiteral{
								Value: "baz",
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
							{
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
