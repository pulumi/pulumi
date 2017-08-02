// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package eval

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi-fabric/pkg/compiler/ast"
	"github.com/pulumi/pulumi-fabric/pkg/compiler/binder"
	"github.com/pulumi/pulumi-fabric/pkg/compiler/core"
	"github.com/pulumi/pulumi-fabric/pkg/compiler/metadata"
	"github.com/pulumi/pulumi-fabric/pkg/compiler/symbols"
	"github.com/pulumi/pulumi-fabric/pkg/compiler/types"
	"github.com/pulumi/pulumi-fabric/pkg/diag"
	"github.com/pulumi/pulumi-fabric/pkg/eval/rt"
	"github.com/pulumi/pulumi-fabric/pkg/pack"
	"github.com/pulumi/pulumi-fabric/pkg/tokens"
	"github.com/pulumi/pulumi-fabric/pkg/util/contract"
	"github.com/pulumi/pulumi-fabric/pkg/workspace"
)

// newTestEval makes an interpreter that can be used for testing purposes.
func newTestEval(hooks Hooks) (binder.Binder, Interpreter) {
	pwd, err := os.Getwd()
	contract.Assert(err == nil)
	ctx := core.NewContext(pwd, core.DefaultSink(pwd), nil)
	w, err := workspace.New(ctx)
	contract.Assert(err == nil)
	reader := metadata.NewReader(ctx)
	b := binder.New(w, ctx, reader)
	return b, New(b.Ctx(), hooks)
}

// TestEvalOns verifies that all exit paths trigger the eval hooks OnStart and OnDone callbacks.
func TestEvalOns(t *testing.T) {
	t.Parallel()

	// First, a package with no default module and no entrypoint.
	{
		pkg := makeTestPackage("nada")
		hooks := &evalOns{}
		b, e := newTestEval(hooks)
		sym := b.BindPackage(pkg)
		ret, uw := e.EvaluatePackage(sym, nil)
		assert.False(t, b.Diag().Success())
		assert.Nil(t, ret)
		assert.Nil(t, uw)
		assert.Equal(t, 1, hooks.onStart)
		assert.Equal(t, 1, hooks.onEnterPackage)
		assert.Equal(t, 0, hooks.onEnterModule)
		assert.Equal(t, 0, hooks.onEnterFunction)
		assert.Equal(t, 0, hooks.onObjectInit)
		assert.Equal(t, 1, hooks.onDone)
	}
	// Now a package with a default module, but still no entrypoint.
	{
		pkg := makeTestPackage("nomain")
		addTestDefaultModule(pkg, false, nil)
		hooks := &evalOns{}
		b, e := newTestEval(hooks)
		sym := b.BindPackage(pkg)
		ret, uw := e.EvaluatePackage(sym, nil)
		assert.False(t, b.Diag().Success())
		assert.Nil(t, ret)
		assert.Nil(t, uw)
		assert.Equal(t, 1, hooks.onStart)
		assert.Equal(t, 1, hooks.onEnterPackage)
		assert.Equal(t, 1, hooks.onEnterModule)
		assert.Equal(t, 0, hooks.onEnterFunction)
		assert.Equal(t, 0, hooks.onObjectInit)
		assert.Equal(t, 1, hooks.onDone)
	}
	// Finally, a package that passes: it has a default module *and* an entrypoint.
	{
		pkg := makeTestPackage("hasboth")
		addTestDefaultModule(pkg, true, nil)
		hooks := &evalOns{}
		b, e := newTestEval(hooks)
		sym := b.BindPackage(pkg)
		ret, uw := e.EvaluatePackage(sym, nil)
		assert.True(t, b.Diag().Success())
		assert.Nil(t, ret)
		assert.Nil(t, uw)
		assert.Equal(t, 1, hooks.onStart)
		assert.Equal(t, 1, hooks.onEnterPackage)
		assert.Equal(t, 1, hooks.onEnterModule)
		assert.Equal(t, 1, hooks.onEnterFunction)
		assert.Equal(t, 0, hooks.onObjectInit)
		assert.Equal(t, 1, hooks.onDone)
	}
}

type evalOns struct {
	onStart         int
	onEnterPackage  int
	onEnterModule   int
	onEnterFunction int
	onObjectInit    int
	onDone          int
}

func (e *evalOns) OnStart() *rt.Unwind {
	e.onStart++
	return nil
}
func (e *evalOns) OnEnterPackage(pkg *symbols.Package) (*rt.Unwind, func()) {
	e.onEnterPackage++
	return nil, nil
}
func (e *evalOns) OnEnterModule(sym *symbols.Module) (*rt.Unwind, func()) {
	e.onEnterModule++
	return nil, nil
}
func (e *evalOns) OnEnterFunction(fnc symbols.Function, args []*rt.Object) (*rt.Unwind, func()) {
	e.onEnterFunction++
	return nil, nil
}
func (e *evalOns) OnObjectInit(tree diag.Diagable, o *rt.Object) *rt.Unwind {
	e.onObjectInit++
	return nil
}
func (e *evalOns) OnDone(uw *rt.Unwind) *rt.Unwind {
	e.onDone++
	return nil
}

// makeTestPackage creates a Lumi package for testing a series of statements to be invoked.
func makeTestPackage(name tokens.PackageName) *pack.Package {
	return &pack.Package{
		Name:    name,
		Modules: &ast.Modules{},
	}
}

// addTestDefaultModule adds a default module and, optionally, a main entrypoint to an existing test package.
func addTestDefaultModule(pkg *pack.Package, hasMain bool, main []ast.Statement) {
	defName := tokens.ModuleName(".default")
	(*pkg.Modules)[defName] = &ast.Module{
		DefinitionNode: ast.DefinitionNode{
			Name: &ast.Identifier{
				Ident: tokens.Name(defName),
			},
		},
		Members: &ast.ModuleMembers{},
	}
	if hasMain {
		mainName := tokens.ModuleMemberName(".main")
		(*(*pkg.Modules)[defName].Members)[mainName] = &ast.ModuleMethod{
			FunctionNode: ast.FunctionNode{
				ReturnType: &ast.TypeToken{
					Tok: types.Dynamic.TypeToken(),
				},
				Body: &ast.Block{
					Statements: main,
				},
			},
			ModuleMemberNode: ast.ModuleMemberNode{
				DefinitionNode: ast.DefinitionNode{
					Name: &ast.Identifier{
						Ident: tokens.Name(mainName),
					},
				},
			},
		}
	}
}
