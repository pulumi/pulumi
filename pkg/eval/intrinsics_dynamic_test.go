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

package eval

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/lumi/pkg/compiler/ast"
	"github.com/pulumi/lumi/pkg/compiler/binder"
	"github.com/pulumi/lumi/pkg/compiler/core"
	"github.com/pulumi/lumi/pkg/compiler/metadata"
	"github.com/pulumi/lumi/pkg/compiler/types"
	"github.com/pulumi/lumi/pkg/pack"
	"github.com/pulumi/lumi/pkg/tokens"
	"github.com/pulumi/lumi/pkg/util/contract"
	"github.com/pulumi/lumi/pkg/workspace"
)

// newTestEval makes an interpreter that can be used for testing purposes.
func newTestEval() (binder.Binder, Interpreter) {
	pwd, err := os.Getwd()
	contract.Assert(err == nil)
	ctx := core.NewContext(pwd, core.DefaultSink(pwd), nil)
	w, err := workspace.New(ctx)
	contract.Assert(err == nil)
	reader := metadata.NewReader(ctx)
	b := binder.New(w, ctx, reader)
	return b, New(b.Ctx(), nil)
}

var isFunctionIntrin = tokens.ModuleMember("lumi:runtime/dynamic:isFunction")

func makeIsFunctionExprAST(dynamic bool) ast.Expression {
	if dynamic {
		var loadLumiMod ast.Expression = &ast.LoadDynamicExpression{
			Name: &ast.StringLiteral{
				Value: isFunctionIntrin.Module().Name().String(),
			},
		}
		return &ast.LoadDynamicExpression{
			Object: &loadLumiMod,
			Name: &ast.StringLiteral{
				Value: isFunctionIntrin.Name().String(),
			},
		}
	}
	return &ast.LoadLocationExpression{
		Name: &ast.Token{
			Tok: tokens.Token(isFunctionIntrin),
		},
	}
}

func makeTestIsFunctionAST(dynamic bool, realFunc bool) *pack.Package {

	// Make the function body.
	var body []ast.Statement

	// If an intrinsic, we need to import the module so that it's available dynamically through a name.
	if dynamic {
		body = append(body, &ast.Import{
			Referent: &ast.Token{
				Tok: tokens.Token(isFunctionIntrin.Module()),
			},
			Name: &ast.Identifier{
				Ident: tokens.Name(isFunctionIntrin.Module().Name().String()),
			},
		})
	}

	var invokeArg *ast.CallArgument
	if realFunc {
		// for real functions, just pass the isFunction function object itself.
		loadFuncExpr := makeIsFunctionExprAST(dynamic)
		invokeArg = &ast.CallArgument{
			Expr: loadFuncExpr,
		}
	} else {
		// for others, just pass a null literal.
		invokeArg = &ast.CallArgument{
			Expr: &ast.NullLiteral{},
		}
	}

	loadFuncExpr := makeIsFunctionExprAST(dynamic)
	var invokeExpr ast.Expression = &ast.InvokeFunctionExpression{
		CallExpressionNode: ast.CallExpressionNode{
			Arguments: &[]*ast.CallArgument{invokeArg},
		},
		Function: loadFuncExpr,
	}
	body = append(body, &ast.ReturnStatement{
		Expression: &invokeExpr,
	})

	// Now return a package with a default module and single entrypoint main function.
	return &pack.Package{
		Name: "testIsFunction",
		Dependencies: &pack.Dependencies{
			"lumi": "*",
		},
		Modules: &ast.Modules{
			tokens.ModuleName(".default"): &ast.Module{
				DefinitionNode: ast.DefinitionNode{
					Name: &ast.Identifier{
						Ident: tokens.Name(".default"),
					},
				},
				Members: &ast.ModuleMembers{
					tokens.ModuleMemberName(".main"): &ast.ModuleMethod{
						FunctionNode: ast.FunctionNode{
							ReturnType: &ast.TypeToken{
								Tok: types.Bool.TypeToken(),
							},
							Body: &ast.Block{
								Statements: body,
							},
						},
						ModuleMemberNode: ast.ModuleMemberNode{
							DefinitionNode: ast.DefinitionNode{
								Name: &ast.Identifier{
									Ident: tokens.Name(".main"),
								},
							},
						},
					},
				},
			},
		},
	}
}

// TestIsFunction verifies the `lumi:runtime/dynamic:isFunction` intrinsic.
func TestIsFunction(t *testing.T) {
	// variant #1: invoke the function statically, passing a null literal; expect a false return.
	{
		b, e := newTestEval()
		pack := makeTestIsFunctionAST(false, false)
		sym := b.BindPackage(pack)
		ret, uw := e.EvaluatePackage(sym, nil)
		assert.True(t, b.Diag().Success(), "Expected a successful evaluation")
		assert.Nil(t, uw, "Did not expect a out-of-the-ordinary unwind to occur (expected a return)")
		assert.NotNil(t, ret, "Expected a non-nil return value")
		assert.True(t, ret.IsBool(), "Expected a bool return value; got %v", ret.Type())
		assert.Equal(t, ret.BoolValue(), false, "Expected a return value of false; got %v", ret.BoolValue())
	}
	// variant #2: invoke the function dynamically, passing a null literal; expect a false return.
	{
		b, e := newTestEval()
		pack := makeTestIsFunctionAST(true, false)
		sym := b.BindPackage(pack)
		ret, uw := e.EvaluatePackage(sym, nil)
		assert.True(t, b.Diag().Success(), "Expected a successful evaluation")
		assert.Nil(t, uw, "Did not expect a out-of-the-ordinary unwind to occur (expected a return)")
		assert.NotNil(t, ret, "Expected a non-nil return value")
		assert.True(t, ret.IsBool(), "Expected a bool return value; got %v", ret.Type())
		assert.Equal(t, ret.BoolValue(), false, "Expected a return value of false; got %v", ret.BoolValue())
	}
	// variant #3: invoke the function statically, passing a real function; expect a true return.
	{
		b, e := newTestEval()
		pack := makeTestIsFunctionAST(false, true)
		sym := b.BindPackage(pack)
		ret, uw := e.EvaluatePackage(sym, nil)
		assert.True(t, b.Diag().Success(), "Expected a successful evaluation")
		assert.Nil(t, uw, "Did not expect a out-of-the-ordinary unwind to occur (expected a return)")
		assert.NotNil(t, ret, "Expected a non-nil return value")
		assert.True(t, ret.IsBool(), "Expected a bool return value; got %v", ret.Type())
		assert.Equal(t, ret.BoolValue(), true, "Expected a return value of true; got %v", ret.BoolValue())
	}
	// variant #4: invoke the function dynamically, passing a real function; expect a true return.
	{
		b, e := newTestEval()
		pack := makeTestIsFunctionAST(true, true)
		sym := b.BindPackage(pack)
		ret, uw := e.EvaluatePackage(sym, nil)
		assert.True(t, b.Diag().Success(), "Expected a successful evaluation")
		assert.Nil(t, uw, "Did not expect a out-of-the-ordinary unwind to occur (expected a return)")
		assert.NotNil(t, ret, "Expected a non-nil return value")
		assert.True(t, ret.IsBool(), "Expected a bool return value; got %v", ret.Type())
		assert.Equal(t, ret.BoolValue(), true, "Expected a return value of true; got %v", ret.BoolValue())
	}
}
