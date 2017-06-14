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

package deploy

import (
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"

	"github.com/pulumi/lumi/pkg/compiler/ast"
	"github.com/pulumi/lumi/pkg/compiler/symbols"
	"github.com/pulumi/lumi/pkg/compiler/types/predef"
	"github.com/pulumi/lumi/pkg/eval/rt"
	"github.com/pulumi/lumi/pkg/pack"
	"github.com/pulumi/lumi/pkg/resource"
	"github.com/pulumi/lumi/pkg/resource/plugin"
	"github.com/pulumi/lumi/pkg/tokens"
	"github.com/pulumi/lumi/pkg/util/cmdutil"
)

// TestNullPlan creates a plan with no operations.
func TestNullPlan(t *testing.T) {
	t.Parallel()

	ctx := plugin.NewContext(cmdutil.Diag(), nil)
	targ := &Target{Name: tokens.QName("null")}
	prev := NewSnapshot(targ.Name, nil, nil)
	plan := NewPlan(ctx, targ, prev, NullSource, nil)
	iter, err := plan.Iterate()
	assert.Nil(t, err)
	assert.NotNil(t, iter)
	next, err := iter.Next()
	assert.Nil(t, err)
	assert.Nil(t, next)
	err = ctx.Close()
	assert.Nil(t, err)
}

// TestErrorPlan creates a plan that immediately fails with an unhandled error.
func TestErrorPlan(t *testing.T) {
	t.Parallel()

	// First trigger an error from Iterate:
	{
		ctx := plugin.NewContext(cmdutil.Diag(), nil)
		targ := &Target{Name: tokens.QName("errs")}
		prev := NewSnapshot(targ.Name, nil, nil)
		plan := NewPlan(ctx, targ, prev, &errorSource{err: errors.New("ITERATE"), duringIterate: true}, nil)
		iter, err := plan.Iterate()
		assert.Nil(t, iter)
		assert.NotNil(t, err)
		assert.Equal(t, "ITERATE", err.Error())
		err = ctx.Close()
		assert.Nil(t, err)
	}

	// Next trigger an error from Next:
	{
		ctx := plugin.NewContext(cmdutil.Diag(), nil)
		targ := &Target{Name: tokens.QName("errs")}
		prev := NewSnapshot(targ.Name, nil, nil)
		plan := NewPlan(ctx, targ, prev, &errorSource{err: errors.New("NEXT"), duringIterate: false}, nil)
		iter, err := plan.Iterate()
		assert.Nil(t, err)
		assert.NotNil(t, iter)
		next, err := iter.Next()
		assert.Nil(t, next)
		assert.NotNil(t, err)
		assert.Equal(t, "NEXT", err.Error())
		err = ctx.Close()
		assert.Nil(t, err)
	}
}

// An errorSource returns an error from either iterate or next, depending on the flag.
type errorSource struct {
	err           error // the error to return.
	duringIterate bool  // if true, the error happens in Iterate; else, Next.
}

func (src *errorSource) Close() error {
	return nil // nothing to do.
}

func (src *errorSource) Info() interface{} {
	return nil
}

func (src *errorSource) Iterate() (SourceIterator, error) {
	if src.duringIterate {
		return nil, src.err
	}
	return &errorSourceIterator{src: src}, nil
}

type errorSourceIterator struct {
	src *errorSource
}

func (iter *errorSourceIterator) Close() error {
	return nil // nothing to do.
}

func (iter *errorSourceIterator) Next() (*resource.Object, tokens.Module, error) {
	return nil, "", iter.src.err
}

// TestBasicCRUDPlan creates a plan with numerous C(R)UD operations.
func TestBasicCRUDPlan(t *testing.T) {
	t.Parallel()

	// Setup a fake namespace/target combination.
	targ := &Target{Name: tokens.QName("crud")}
	ns := targ.Name
	base := fakeResourceBase()
	pkg, mod, newResourceType := fakeTestResources(tokens.PackageName("testcrud"), tokens.ModuleName("index"))

	// Create a context that the snapshots and plan will use.
	ctx := plugin.NewContext(cmdutil.Diag(), &testProviderHost{
		provider: func(propkg tokens.Package) (plugin.Provider, error) {
			if propkg != pkg.Tok {
				return nil, errors.Errorf("Unexpected request to load package %v; expected just %v", propkg, pkg)
			}
			return &testProvider{
				check: func(t tokens.Type, props resource.PropertyMap) ([]plugin.CheckFailure, error) {
					return nil, nil // accept all changes.
				},
				name: func(_ tokens.Type, props resource.PropertyMap) (tokens.QName, error) {
					name, has := props["name"]
					assert.True(t, has)
					assert.True(t, name.IsString())
					return tokens.QName(name.StringValue()), nil
				},
				inspectChange: func(t tokens.Type, id resource.ID, olds resource.PropertyMap,
					news resource.PropertyMap) ([]resource.PropertyKey, resource.PropertyMap, error) {
					return nil, nil, nil // accept all changes.
				},
				// we don't actually execute the plan, so there's no need to implement the other functions.
			}, nil
		},
	})

	// Some shared tokens and names.
	typA := newResourceType(tokens.TypeName("A"), base)
	namA := tokens.QName("res-a")
	urnA := resource.NewURN(ns, mod.Tok, typA.Tok, namA)
	typB := newResourceType(tokens.TypeName("B"), base)
	namB := tokens.QName("res-b")
	urnB := resource.NewURN(ns, mod.Tok, typB.Tok, namB)
	typC := newResourceType(tokens.TypeName("C"), base)
	namC := tokens.QName("res-c")
	urnC := resource.NewURN(ns, mod.Tok, typC.Tok, namC)
	typD := newResourceType(tokens.TypeName("D"), base)
	namD := tokens.QName("res-d")
	urnD := resource.NewURN(ns, mod.Tok, typD.Tok, namD)

	// Create the old resources snapshot.
	oldResB := resource.NewState(typB.Tok, urnB, resource.ID("b-b-b"), resource.PropertyMap{
		"name": resource.NewStringProperty(namB.String()),
		"bf1":  resource.NewStringProperty("b-value"),
		"bf2":  resource.NewNumberProperty(42),
	}, nil)
	oldResC := resource.NewState(typC.Tok, urnC, resource.ID("c-c-c"), resource.PropertyMap{
		"name": resource.NewStringProperty(namC.String()),
		"cf1":  resource.NewStringProperty("c-value"),
		"cf2":  resource.NewNumberProperty(83),
	}, nil)
	oldResD := resource.NewState(typD.Tok, urnD, resource.ID("d-d-d"), resource.PropertyMap{
		"name": resource.NewStringProperty(namD.String()),
		"df1":  resource.NewStringProperty("d-value"),
		"df2":  resource.NewNumberProperty(167),
	}, nil)
	oldsnap := NewSnapshot(ns, []*resource.State{oldResB, oldResC, oldResD}, nil)

	// Create the new resource objects a priori.
	//     - A is created:
	newObjA := rt.NewObject(typA, nil, nil, nil)
	newObjA.Properties().InitAddr(rt.PropertyKey("name"), rt.NewStringObject(namA.String()), false, nil, nil)
	newObjA.Properties().InitAddr(rt.PropertyKey("af1"), rt.NewStringObject("a-value"), false, nil, nil)
	newObjA.Properties().InitAddr(rt.PropertyKey("af2"), rt.NewNumberObject(42), false, nil, nil)
	newResA := resource.NewObject(newObjA)
	newResAProps := newResA.CopyProperties()
	//     - B is updated:
	newObjB := rt.NewObject(typB, nil, nil, nil)
	newObjB.Properties().InitAddr(rt.PropertyKey("name"), rt.NewStringObject(namB.String()), false, nil, nil)
	newObjB.Properties().InitAddr(rt.PropertyKey("bf1"), rt.NewStringObject("b-value"), false, nil, nil)
	// delete the bf2 field, and add bf3.
	newObjB.Properties().InitAddr(rt.PropertyKey("bf3"), rt.True, false, nil, nil)
	newResB := resource.NewObject(newObjB)
	newResBProps := newResB.CopyProperties()
	//     - C has no changes:
	newObjC := rt.NewObject(typC, nil, nil, nil)
	newObjC.Properties().InitAddr(rt.PropertyKey("name"), rt.NewStringObject(namC.String()), false, nil, nil)
	newObjC.Properties().InitAddr(rt.PropertyKey("cf1"), rt.NewStringObject("c-value"), false, nil, nil)
	newObjC.Properties().InitAddr(rt.PropertyKey("cf2"), rt.NewNumberObject(83), false, nil, nil)
	newResC := resource.NewObject(newObjC)
	//     - No D; it is deleted.

	// Use a fixed source that just returns the above predefined objects during planning.
	new := NewFixedSource(mod.Tok, []*resource.Object{newResA, newResB, newResC})

	// Next up, create a plan from the new and old, and validate its shape.
	plan := NewPlan(ctx, targ, oldsnap, new, nil)

	// Next, validate the steps and ensure that we see all of the expected ones.  Note that there aren't any
	// dependencies between the steps, so we must validate it in a way that's insensitive of order.
	seen := make(map[StepOp]int)
	iter, err := plan.Iterate()
	assert.Nil(t, err)
	assert.NotNil(t, iter)
	for {
		step, err := iter.Next()
		assert.Nil(t, err)
		if step == nil {
			break
		}

		op := step.Op()
		switch op {
		case OpCreate: // A is created
			old := step.Old()
			new := step.New()
			assert.Nil(t, old)
			assert.NotNil(t, new)
			assert.Equal(t, urnA, new.URN())
			assert.Equal(t, newResAProps, step.Inputs())
		case OpUpdate: // B is updated
			old := step.Old()
			new := step.New()
			assert.NotNil(t, old)
			assert.Equal(t, urnB, old.URN())
			assert.Equal(t, oldResB, old)
			assert.NotNil(t, new)
			assert.Equal(t, urnB, new.URN())
			assert.Equal(t, newResBProps, step.Inputs())
		case OpDelete: // D is deleted
			old := step.Old()
			new := step.New()
			assert.NotNil(t, old)
			assert.Equal(t, urnD, old.URN())
			assert.Equal(t, oldResD, old)
			assert.Nil(t, new)
		}

		seen[op]++ // track the # of these we've seen so we can validate.
	}
	assert.Equal(t, 1, seen[OpCreate])
	assert.Equal(t, 1, seen[OpUpdate])
	assert.Equal(t, 1, seen[OpDelete])
	assert.Equal(t, 0, seen[OpReplaceCreate])
	assert.Equal(t, 0, seen[OpReplaceDelete])

	assert.Equal(t, 1, len(iter.Creates()))
	assert.True(t, iter.Creates()[urnA])
	assert.Equal(t, 1, len(iter.Updates()))
	assert.True(t, iter.Updates()[urnB])
	assert.Equal(t, 0, len(iter.Replaces()))
	assert.Equal(t, 1, len(iter.Sames()))
	assert.True(t, iter.Sames()[urnC])
	assert.Equal(t, 1, len(iter.Deletes()))
	assert.True(t, iter.Deletes()[urnD])
}

// fakeResourceBase news up a resource type so that it looks like a predefined resource type.
func fakeResourceBase() symbols.Type {
	_, _, fact := fakeTestResources(predef.LumiStdlib.Name(), predef.LumiStdlibResourceModule.Name())
	return fact(predef.LumiStdlibResourceClass.Name(), nil)
}

// fakeTestResources creates a fake package, module, and factory for resource types.
func fakeTestResources(pkgnm tokens.PackageName,
	modnm tokens.ModuleName) (*symbols.Package, *symbols.Module, func(tokens.TypeName, symbols.Type) *symbols.Class) {
	pkg := symbols.NewPackageSym(&pack.Package{
		Name: pkgnm,
	})
	mod := symbols.NewModuleSym(&ast.Module{
		DefinitionNode: ast.DefinitionNode{
			Name: &ast.Identifier{Ident: tokens.Name(modnm)},
		},
	}, pkg)
	pkg.Modules[mod.Tok.Name()] = mod
	return pkg, mod, func(tynm tokens.TypeName, base symbols.Type) *symbols.Class {
		cls := symbols.NewClassSym(&ast.Class{
			ModuleMemberNode: ast.ModuleMemberNode{
				DefinitionNode: ast.DefinitionNode{
					Name: &ast.Identifier{Ident: tokens.Name(tynm)},
				},
			},
		}, mod, base, nil)
		mod.Members[cls.MemberName()] = cls
		return cls
	}
}

type testProviderHost struct {
	analyzer func(nm tokens.QName) (plugin.Analyzer, error)
	provider func(pkg tokens.Package) (plugin.Provider, error)
}

func (host *testProviderHost) Close() error {
	return nil
}
func (host *testProviderHost) Analyzer(nm tokens.QName) (plugin.Analyzer, error) {
	return host.analyzer(nm)
}
func (host *testProviderHost) Provider(pkg tokens.Package) (plugin.Provider, error) {
	return host.provider(pkg)
}

type testProvider struct {
	pkg           tokens.Package
	check         func(tokens.Type, resource.PropertyMap) ([]plugin.CheckFailure, error)
	name          func(tokens.Type, resource.PropertyMap) (tokens.QName, error)
	create        func(tokens.Type, resource.PropertyMap) (resource.ID, resource.Status, error)
	get           func(tokens.Type, resource.ID) (resource.PropertyMap, error)
	inspectChange func(tokens.Type, resource.ID,
		resource.PropertyMap, resource.PropertyMap) ([]resource.PropertyKey, resource.PropertyMap, error)
	update func(tokens.Type, resource.ID,
		resource.PropertyMap, resource.PropertyMap) (resource.Status, error)
	delete func(tokens.Type, resource.ID) (resource.Status, error)
}

func (prov *testProvider) Close() error {
	return nil
}
func (prov *testProvider) Pkg() tokens.Package {
	return prov.pkg
}
func (prov *testProvider) Check(t tokens.Type, props resource.PropertyMap) ([]plugin.CheckFailure, error) {
	return prov.check(t, props)
}
func (prov *testProvider) Name(t tokens.Type, props resource.PropertyMap) (tokens.QName, error) {
	return prov.name(t, props)
}
func (prov *testProvider) Create(t tokens.Type, props resource.PropertyMap) (resource.ID, resource.Status, error) {
	return prov.create(t, props)
}
func (prov *testProvider) Get(t tokens.Type, id resource.ID) (resource.PropertyMap, error) {
	return prov.get(t, id)
}
func (prov *testProvider) InspectChange(t tokens.Type, id resource.ID,
	olds resource.PropertyMap, news resource.PropertyMap) ([]resource.PropertyKey, resource.PropertyMap, error) {
	return prov.inspectChange(t, id, olds, news)
}
func (prov *testProvider) Update(t tokens.Type, id resource.ID,
	olds resource.PropertyMap, news resource.PropertyMap) (resource.Status, error) {
	return prov.update(t, id, olds, news)
}
func (prov *testProvider) Delete(t tokens.Type, id resource.ID) (resource.Status, error) {
	return prov.delete(t, id)
}
