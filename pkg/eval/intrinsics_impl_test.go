// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package eval

import (
	"fmt"
	"os"
	"testing"

	"github.com/pulumi/pulumi-fabric/pkg/compiler/ast"
	"github.com/pulumi/pulumi-fabric/pkg/compiler/binder"
	"github.com/pulumi/pulumi-fabric/pkg/compiler/core"
	"github.com/pulumi/pulumi-fabric/pkg/compiler/metadata"
	"github.com/pulumi/pulumi-fabric/pkg/compiler/types"
	"github.com/pulumi/pulumi-fabric/pkg/eval/rt"
	"github.com/pulumi/pulumi-fabric/pkg/pack"
	"github.com/pulumi/pulumi-fabric/pkg/tokens"
	"github.com/pulumi/pulumi-fabric/pkg/util/contract"
	"github.com/pulumi/pulumi-fabric/pkg/workspace"
	"github.com/stretchr/testify/assert"
)

// intrin carries the qualified name and signature for an intrinsic function
type intrin struct {
	moduleMember tokens.ModuleMember
	paramTypes   []tokens.Type
	returnType   tokens.Type
}

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

func makeFakeIntrinsicDefinition(intrin intrin) *ast.Module {
	moduleMemberName := intrin.moduleMember.Name()
	moduleName := tokens.Name(intrin.moduleMember.Module().Name())
	functionName := tokens.Name(intrin.moduleMember.Name())
	intrinToken := tokens.Token(intrin.moduleMember)

	var params []*ast.LocalVariable
	for i, pty := range intrin.paramTypes {
		params = append(params, &ast.LocalVariable{
			DefinitionNode: ast.DefinitionNode{
				Name: &ast.Identifier{
					Ident: tokens.Name(fmt.Sprintf("x%d", i)),
				},
			},
			VariableNode: ast.VariableNode{
				Type: &ast.TypeToken{
					Tok: pty,
				},
			},
		})
	}

	return &ast.Module{
		DefinitionNode: ast.DefinitionNode{
			Name: &ast.Identifier{
				Ident: moduleName,
			},
		},
		Exports: &ast.ModuleExports{
			moduleMemberName: &ast.Export{
				DefinitionNode: ast.DefinitionNode{
					Name: &ast.Identifier{
						Ident: functionName,
					},
				},
				Referent: &ast.Token{
					Tok: intrinToken,
				},
			},
		},
		Members: &ast.ModuleMembers{
			moduleMemberName: &ast.ModuleMethod{
				FunctionNode: ast.FunctionNode{
					Parameters: &params,
					ReturnType: &ast.TypeToken{
						Tok: intrin.returnType,
					},
					Body: &ast.Block{
						Statements: []ast.Statement{
							&ast.ThrowStatement{
								Expression: &ast.StringLiteral{
									Value: "unreachable",
								},
							},
						},
					},
				},
				ModuleMemberNode: ast.ModuleMemberNode{
					DefinitionNode: ast.DefinitionNode{
						Name: &ast.Identifier{
							Ident: functionName,
						},
					},
				},
			},
		},
	}
}

// makeTestPackage creates a Lumi package for testing a series of statements to be invoked
func makeTestPackage(body []ast.Statement, fakeIntrinsicDefinition *ast.Module) *pack.Package {
	return &pack.Package{
		Name: "lumirt",
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
								Tok: types.Dynamic.TypeToken(),
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
			tokens.ModuleName("index"): fakeIntrinsicDefinition,
		},
	}
}

// makeInvokeIntrinsicAST creates the AST for invoking a requested intrinsic, potentially dynamically, with a
// provided argument list.  It returns the statements to invoke.
func makeInvokeIntrinsicAST(intrin tokens.ModuleMember, dynamic bool, args []ast.Expression) []ast.Statement {
	var loadFuncExpr ast.Expression
	if dynamic {
		var loadLumiMod ast.Expression = &ast.LoadDynamicExpression{
			Name: &ast.StringLiteral{
				Value: intrin.Module().Name().String(),
			},
		}
		loadFuncExpr = &ast.LoadDynamicExpression{
			Object: &loadLumiMod,
			Name: &ast.StringLiteral{
				Value: intrin.Name().String(),
			},
		}
	} else {
		loadFuncExpr = &ast.LoadLocationExpression{
			Name: &ast.Token{
				Tok: tokens.Token(intrin),
			},
		}
	}
	var arguments []*ast.CallArgument
	for _, arg := range args {
		arguments = append(arguments, &ast.CallArgument{Expr: arg})
	}

	var invokeExpr ast.Expression = &ast.InvokeFunctionExpression{
		CallExpressionNode: ast.CallExpressionNode{
			Arguments: &arguments,
		},
		Function: loadFuncExpr,
	}

	return []ast.Statement{
		&ast.Import{
			Referent: &ast.Token{
				Tok: tokens.Token(intrin.Module()),
			},
			Name: &ast.Identifier{
				Ident: tokens.Name(intrin.Module().Name().String()),
			},
		},
		&ast.ReturnStatement{
			Expression: &invokeExpr,
		},
	}
}

// invokeIntrinsic creates an AST for calling the intrinsic with the provided arguments and evaluates that AST in a
// fresh evaluator.  It returns the resulting object and unwind, as well as the binder used during evaluation.  If the
// dynamic flag is true, the function is loaded dynamically, else it is loaded statically through a reference to the
// intrinsic symbol.
func invokeIntrinsic(intrin intrin, dynamic bool, args []ast.Expression) (binder.Binder,
	*rt.Object, *rt.Unwind) {
	b, e := newTestEval()
	body := makeInvokeIntrinsicAST(intrin.moduleMember, dynamic, args)
	fakeIntrinsicDefinitionModule := makeFakeIntrinsicDefinition(intrin)
	pack := makeTestPackage(body, fakeIntrinsicDefinitionModule)
	sym := b.BindPackage(pack)
	ret, uw := e.EvaluatePackage(sym, nil)
	return b, ret, uw
}

// TestIsFunction verifies the `lumirt:index:isFunction` intrinsic.
func Test_IsFunction(t *testing.T) {
	t.Parallel()

	isFunctionIntrin := intrin{
		moduleMember: tokens.ModuleMember("lumirt:index:isFunction"),
		paramTypes:   []tokens.Type{types.Object.TypeToken()},
		returnType:   types.Bool.TypeToken(),
	}
	aFunction := &ast.LoadLocationExpression{
		Name: &ast.Token{
			Tok: tokens.Token(isFunctionIntrin.moduleMember),
		},
	}
	notAFunction := &ast.NullLiteral{}

	// variant #1: invoke the function statically, passing a null literal; expect a false return.
	{
		b, ret, uw := invokeIntrinsic(isFunctionIntrin, false, []ast.Expression{notAFunction})
		assert.True(t, b.Diag().Success(), "Expected a successful evaluation")
		assert.Nil(t, uw, "Did not expect a out-of-the-ordinary unwind to occur (expected a return)")
		assert.NotNil(t, ret, "Expected a non-nil return value")
		assert.True(t, ret.IsBool(), "Unexpected return type: %v", ret.Type())
		val := ret.BoolValue()
		assert.Equal(t, false, val, "Unexpected return value: %v", val)
	}
	// variant #2: invoke the function dynamically, passing a null literal; expect a false return.
	{
		b, ret, uw := invokeIntrinsic(isFunctionIntrin, true, []ast.Expression{notAFunction})
		assert.True(t, b.Diag().Success(), "Expected a successful evaluation")
		assert.Nil(t, uw, "Did not expect a out-of-the-ordinary unwind to occur (expected a return)")
		assert.NotNil(t, ret, "Expected a non-nil return value")
		assert.True(t, ret.IsBool(), "Unexpected return type: %v", ret.Type())
		val := ret.BoolValue()
		assert.Equal(t, false, val, "Unexpected return value: %v", val)
	}
	// variant #3: invoke the function statically, passing a real function; expect a true return.
	{
		b, ret, uw := invokeIntrinsic(isFunctionIntrin, false, []ast.Expression{aFunction})
		assert.True(t, b.Diag().Success(), "Expected a successful evaluation")
		assert.Nil(t, uw, "Did not expect a out-of-the-ordinary unwind to occur (expected a return)")
		assert.NotNil(t, ret, "Expected a non-nil return value")
		assert.True(t, ret.IsBool(), "Unexpected return type: %v", ret.Type())
		val := ret.BoolValue()
		assert.Equal(t, true, val, "Unexpected return value: %v", val)
	}
	// variant #4: invoke the function dynamically, passing a real function; expect a true return.
	{
		b, ret, uw := invokeIntrinsic(isFunctionIntrin, true, []ast.Expression{aFunction})
		assert.True(t, b.Diag().Success(), "Expected a successful evaluation")
		assert.Nil(t, uw, "Did not expect a out-of-the-ordinary unwind to occur (expected a return)")
		assert.NotNil(t, ret, "Expected a non-nil return value")
		assert.True(t, ret.IsBool(), "Unexpected return type: %v", ret.Type())
		val := ret.BoolValue()
		assert.Equal(t, true, val, "Unexpected return value: %v", val)
	}
}

func Test_JsonStringify(t *testing.T) {
	t.Parallel()

	jsonStringifyIntrin := intrin{
		moduleMember: tokens.ModuleMember("lumirt:index:jsonStringify"),
		paramTypes:   []tokens.Type{types.Object.TypeToken()},
		returnType:   types.String.TypeToken(),
	}

	{
		//jsonStringify(`a
		//b"`)
		obj := &ast.StringLiteral{
			Value: `a
b"`,
		}
		b, ret, uw := invokeIntrinsic(jsonStringifyIntrin, true, []ast.Expression{obj})
		assert.True(t, b.Diag().Success(), "Expected a successful evaluation")
		assert.Nil(t, uw, "Did not expect a out-of-the-ordinary unwind to occur (expected a return)")
		assert.NotNil(t, ret, "Expected a non-nil return value")
		assert.True(t, ret.IsString(), "Unexpected return type: %v", ret.Type())
		val := ret.StringValue()
		assert.Equal(t, "\"a\\nb\\\"\"", val, "Unexpected return value: %v", val)
	}
}

func Test_SerializeClosure(t *testing.T) {
	t.Parallel()

	serializeClosure := intrin{
		moduleMember: tokens.ModuleMember("lumirt:index:serializeClosure"),
		paramTypes:   []tokens.Type{types.Dynamic.TypeToken()},
		returnType:   types.Dynamic.TypeToken(),
	}

	{
		// let f = () => 12
		f := &ast.LambdaExpression{
			FunctionNode: ast.FunctionNode{
				Body: &ast.ExpressionStatement{
					Expression: &ast.NumberLiteral{
						Value: 12.0,
					},
				},
				Parameters: &[]*ast.LocalVariable{},
				ReturnType: &ast.TypeToken{
					Tok: types.Number.TypeToken(),
				},
			},
			SourceLanguage: ".js",
			SourceText:     "return function() { return 12; }",
		}

		b, ret, uw := invokeIntrinsic(serializeClosure, true, []ast.Expression{f})
		assert.True(t, b.Diag().Success(), "Expected a successful evaluation")
		assert.Nil(t, uw, "Did not expect a out-of-the-ordinary unwind to occur (expected a return)")
		assert.NotNil(t, ret, "Expected a non-nil return value")

		code := ret.GetPropertyAddr("code", false, true)
		assert.NotNil(t, code, "Expected a non-nil 'code' property")
		assert.True(t, code.Obj().IsString(), "Expected 'code' to be a string")
		assert.Equal(t, "return function() { return 12; }", code.Obj().StringValue(), "Expected 'code' to be a string")

		signature := ret.GetPropertyAddr("signature", false, true)
		assert.NotNil(t, signature, "Expected a non-nil 'signature' property")
		assert.True(t, signature.Obj().IsString(), "Expected 'signature' to be a string")
		assert.Equal(t, "()number",
			signature.Obj().StringValue(), "Expected 'signature' to be a string")

		language := ret.GetPropertyAddr("language", false, true)
		assert.NotNil(t, language, "Expected a non-nil 'language' property")
		assert.True(t, language.Obj().IsString(), "Expected 'language' to be a string")
		assert.Equal(t, ".js",
			language.Obj().StringValue(), "Expected 'language' to be a string")

		environment := ret.GetPropertyAddr("environment", false, true)
		assert.NotNil(t, environment, "Expected a non-nil 'environment' property")
		envProps := environment.Obj().Properties()
		assert.Len(t, envProps.Stable(), 0, "Expected 0 variables in the environment")
	}
}
