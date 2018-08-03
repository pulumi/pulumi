// Copyright 2016-2018, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
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

	"github.com/blang/semver"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/resource/plugin"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/workspace"
)

// TestNullPlan creates a plan with no operations.
func TestNullPlan(t *testing.T) {
	t.Parallel()

	ctx, err := plugin.NewContext(cmdutil.Diag(), nil, nil, nil, "", nil)
	assert.Nil(t, err)
	targ := &Target{Name: tokens.QName("null")}
	prev := NewSnapshot(Manifest{}, nil)
	plan := NewPlan(ctx, targ, prev, NullSource, nil, false)
	iter, err := plan.Start(Options{})
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
		ctx, err := plugin.NewContext(cmdutil.Diag(), nil, nil, nil, "", nil)
		assert.Nil(t, err)
		targ := &Target{Name: tokens.QName("errs")}
		prev := NewSnapshot(Manifest{}, nil)
		plan := NewPlan(ctx, targ, prev, &errorSource{err: errors.New("ITERATE"), duringIterate: true}, nil, false)
		iter, err := plan.Start(Options{})
		assert.Nil(t, iter)
		assert.NotNil(t, err)
		assert.Equal(t, "ITERATE", err.Error())
		err = ctx.Close()
		assert.Nil(t, err)
	}

	// Next trigger an error from Next:
	{
		ctx, err := plugin.NewContext(cmdutil.Diag(), nil, nil, nil, "", nil)
		assert.Nil(t, err)
		targ := &Target{Name: tokens.QName("errs")}
		prev := NewSnapshot(Manifest{}, nil)
		plan := NewPlan(ctx, targ, prev, &errorSource{err: errors.New("NEXT"), duringIterate: false}, nil, false)
		iter, err := plan.Start(Options{})
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

func (src *errorSource) Close() error                { return nil }
func (src *errorSource) Project() tokens.PackageName { return "" }
func (src *errorSource) Info() interface{}           { return nil }
func (src *errorSource) IsRefresh() bool             { return false }

func (src *errorSource) Iterate(opts Options) (SourceIterator, error) {
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

func (iter *errorSourceIterator) Next() (SourceEvent, error) {
	return nil, iter.src.err
}

// TestBasicCRUDPlan creates a plan with numerous C(R)UD operations.
func TestBasicCRUDPlan(t *testing.T) {
	t.Parallel()

	// Create a context that the snapshots and plan will use.
	pkg := tokens.Package("testcrud")
	ctx, err := plugin.NewContext(cmdutil.Diag(), &testProviderHost{
		provider: func(propkg tokens.Package, version *semver.Version) (plugin.Provider, error) {
			if propkg != pkg {
				return nil, errors.Errorf("Unexpected request to load package %v; expected just %v", propkg, pkg)
			}
			return &testProvider{
				check: func(urn resource.URN,
					olds, news resource.PropertyMap) (resource.PropertyMap, []plugin.CheckFailure, error) {
					return news, nil, nil // accept all changes.
				},
				diff: func(urn resource.URN, id resource.ID, olds resource.PropertyMap,
					news resource.PropertyMap) (plugin.DiffResult, error) {
					return plugin.DiffResult{}, nil // accept all changes.
				},
				// we don't actually execute the plan, so there's no need to implement the other functions.
			}, nil
		},
	}, nil, nil, "", nil)
	assert.Nil(t, err)

	// Setup a fake namespace/target combination.
	targ := &Target{Name: tokens.QName("crud")}
	ns := targ.Name
	mod := tokens.Module(pkg + ":index")
	pkgname := pkg.Name()
	parentType := tokens.Type("")

	// Some shared tokens and names.
	typA := tokens.Type(mod + ":A")
	namA := tokens.QName("res-a")
	urnA := resource.NewURN(ns, pkgname, parentType, typA, namA)
	typB := tokens.Type(mod + ":B")
	namB := tokens.QName("res-b")
	urnB := resource.NewURN(ns, pkgname, parentType, typB, namB)
	typC := tokens.Type(mod + ":C")
	namC := tokens.QName("res-c")
	urnC := resource.NewURN(ns, pkgname, parentType, typC, namC)
	typD := tokens.Type(mod + ":D")
	namD := tokens.QName("res-d")
	urnD := resource.NewURN(ns, pkgname, parentType, typD, namD)

	// Create the old resources snapshot.
	oldResB := resource.NewState(typB, urnB, true, false, resource.ID("b-b-b"),
		resource.PropertyMap{
			"bf1": resource.NewStringProperty("b-value"),
			"bf2": resource.NewNumberProperty(42),
		},
		nil,
		"",
		false,
		false,
		nil,
		[]string{},
	)
	oldResC := resource.NewState(typC, urnC, true, false, resource.ID("c-c-c"),
		resource.PropertyMap{
			"cf1": resource.NewStringProperty("c-value"),
			"cf2": resource.NewNumberProperty(83),
		},
		resource.PropertyMap{
			"outta1":   resource.NewStringProperty("populated during skip/step"),
			"outta234": resource.NewNumberProperty(99881122),
		},
		"",
		false,
		false,
		nil,
		[]string{},
	)
	oldResD := resource.NewState(typD, urnD, true, false, resource.ID("d-d-d"),
		resource.PropertyMap{
			"df1": resource.NewStringProperty("d-value"),
			"df2": resource.NewNumberProperty(167),
		},
		nil,
		"",
		false,
		false,
		nil,
		[]string{},
	)
	oldsnap := NewSnapshot(Manifest{}, []*resource.State{oldResB, oldResC, oldResD})

	// Create the new resource objects a priori.
	//     - A is created:
	newResA := resource.NewGoal(typA, namA, true, resource.PropertyMap{
		"af1": resource.NewStringProperty("a-value"),
		"af2": resource.NewNumberProperty(42),
	}, "", false, nil)
	newStateA := &testRegEvent{goal: newResA}
	//     - B is updated:
	newResB := resource.NewGoal(typB, namB, true, resource.PropertyMap{
		"bf1": resource.NewStringProperty("b-value"),
		// delete the bf2 field, and add bf3.
		"bf3": resource.NewBoolProperty(true),
	}, "", false, nil)
	newStateB := &testRegEvent{goal: newResB}
	//     - C has no changes:
	newResC := resource.NewGoal(typC, namC, true, resource.PropertyMap{
		"cf1": resource.NewStringProperty("c-value"),
		"cf2": resource.NewNumberProperty(83),
	}, "", false, nil)
	newStateC := &testRegEvent{goal: newResC}
	//     - No D; it is deleted.

	// Use a fixed source that just returns the above predefined objects during planning.
	source := NewFixedSource(pkgname, []SourceEvent{newStateA, newStateB, newStateC})

	// Next up, create a plan from the new and old, and validate its shape.
	plan := NewPlan(ctx, targ, oldsnap, source, nil, false)

	// Next, validate the steps and ensure that we see all of the expected ones.  Note that there aren't any
	// dependencies between the steps, so we must validate it in a way that's insensitive of order.
	seen := make(map[StepOp]int)
	iter, err := plan.Start(Options{})
	assert.Nil(t, err)
	assert.NotNil(t, iter)
	for {
		step, err := iter.Next()
		assert.Nil(t, err)
		if step == nil {
			break
		}

		var state *resource.State
		var urn resource.URN
		var realID bool
		var expectOuts resource.PropertyMap
		switch s := step.(type) {
		case *CreateStep: // A is created
			old := s.Old()
			assert.Nil(t, old)
			new := s.New()
			assert.NotNil(t, new)
			assert.Equal(t, urnA, new.URN)
			assert.Equal(t, newResA.Properties, new.Inputs)
			state = new
			urn, realID = urnA, false
		case *UpdateStep: // B is updated
			old := s.Old()
			assert.NotNil(t, old)
			assert.Equal(t, urnB, old.URN)
			assert.Equal(t, oldResB, old)
			new := s.New()
			assert.NotNil(t, new)
			assert.Equal(t, urnB, new.URN)
			assert.Equal(t, newResB.Properties, new.Inputs)
			state = new
			urn, realID = urnB, true
		case *SameStep: // C is the same
			old := s.Old()
			assert.NotNil(t, old)
			assert.Equal(t, urnC, old.URN)
			assert.Equal(t, oldResC, old)
			new := s.New()
			assert.NotNil(t, s.New())
			assert.Equal(t, urnC, new.URN)
			assert.Equal(t, newResC.Properties, new.Inputs)
			state = new
			urn, realID, expectOuts = urnC, true, oldResC.Outputs
		case *DeleteStep: // D is deleted
			old := s.Old()
			assert.NotNil(t, old)
			assert.Equal(t, urnD, old.URN)
			assert.Equal(t, oldResD, old)
			new := s.New()
			assert.Nil(t, new)
		default:
			t.FailNow() // unexpected step kind.
		}

		_, err = step.Apply(true)
		assert.Nil(t, err)

		op := step.Op()
		if state != nil {
			// Ensure the URN, ID, and properties are populated correctly.
			assert.Equal(t, urn, state.URN,
				"Expected op %v to populate a URN equal to %v", op, urn)

			if realID {
				assert.NotEqual(t, resource.ID(""), state.ID,
					"Expected op %v to populate a real ID (%v)", op, urn)
			} else {
				assert.Equal(t, resource.ID(""), state.ID,
					"Expected op %v to leave ID blank (%v); got %v", op, urn, state.ID)
			}

			if expectOuts != nil {
				props := state.All()
				for k := range expectOuts {
					assert.Equal(t, expectOuts[k], props[k])
				}
			}
		}

		seen[op]++ // track the # of these we've seen so we can validate.
	}
	assert.Equal(t, 1, seen[OpCreate])
	assert.Equal(t, 1, seen[OpUpdate])
	assert.Equal(t, 1, seen[OpDelete])
	assert.Equal(t, 0, seen[OpReplace])
	assert.Equal(t, 0, seen[OpDeleteReplaced])

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

type testRegEvent struct {
	goal   *resource.Goal
	result *RegisterResult
}

var _ RegisterResourceEvent = (*testRegEvent)(nil)

func (g *testRegEvent) event() {}

func (g *testRegEvent) Goal() *resource.Goal {
	return g.goal
}

func (g *testRegEvent) Done(result *RegisterResult) {
	contract.Assertf(g.result == nil, "Attempt to invoke testRegEvent.Done more than once")
	g.result = result
}

type testProviderHost struct {
	analyzer func(nm tokens.QName) (plugin.Analyzer, error)
	provider func(pkg tokens.Package, version *semver.Version) (plugin.Provider, error)
	langhost func(runtime string) (plugin.LanguageRuntime, error)
}

func (host *testProviderHost) SignalCancellation() error {
	return nil
}
func (host *testProviderHost) Close() error {
	return nil
}
func (host *testProviderHost) ServerAddr() string {
	contract.Failf("Host RPC address not available")
	return ""
}
func (host *testProviderHost) Log(sev diag.Severity, urn resource.URN, msg string, streamID int32) {
	cmdutil.Diag().Logf(sev, diag.StreamMessage(urn, msg, streamID))
}
func (host *testProviderHost) ReadLocation(tok tokens.Token) (resource.PropertyValue, error) {
	return resource.PropertyValue{}, errors.New("Invalid location")
}
func (host *testProviderHost) ReadLocations(tok tokens.Token) (resource.PropertyMap, error) {
	return nil, errors.New("Invalid location")
}
func (host *testProviderHost) Analyzer(nm tokens.QName) (plugin.Analyzer, error) {
	return host.analyzer(nm)
}
func (host *testProviderHost) Provider(pkg tokens.Package, version *semver.Version) (plugin.Provider, error) {
	return host.provider(pkg, version)
}
func (host *testProviderHost) LanguageRuntime(runtime string) (plugin.LanguageRuntime, error) {
	return host.langhost(runtime)
}
func (host *testProviderHost) ListPlugins() []workspace.PluginInfo {
	return nil
}
func (host *testProviderHost) EnsurePlugins(plugins []workspace.PluginInfo, kinds plugin.Flags) error {
	return nil
}
func (host *testProviderHost) GetRequiredPlugins(info plugin.ProgInfo,
	kinds plugin.Flags) ([]workspace.PluginInfo, error) {
	return nil, nil
}

type testProvider struct {
	pkg    tokens.Package
	config func(map[config.Key]string) error
	check  func(resource.URN,
		resource.PropertyMap, resource.PropertyMap) (resource.PropertyMap, []plugin.CheckFailure, error)
	create func(resource.URN, resource.PropertyMap) (resource.ID, resource.PropertyMap, resource.Status, error)
	diff   func(resource.URN, resource.ID, resource.PropertyMap, resource.PropertyMap) (plugin.DiffResult, error)
	read   func(resource.URN, resource.ID, resource.PropertyMap) (resource.PropertyMap, error)
	update func(resource.URN, resource.ID,
		resource.PropertyMap, resource.PropertyMap) (resource.PropertyMap, resource.Status, error)
	delete func(resource.URN, resource.ID, resource.PropertyMap) (resource.Status, error)
	invoke func(tokens.ModuleMember, resource.PropertyMap) (resource.PropertyMap, []plugin.CheckFailure, error)
}

func (prov *testProvider) SignalCancellation() error {
	return nil
}
func (prov *testProvider) Close() error {
	return nil
}
func (prov *testProvider) Pkg() tokens.Package {
	return prov.pkg
}
func (prov *testProvider) Configure(vars map[config.Key]string) error {
	return prov.config(vars)
}
func (prov *testProvider) Check(urn resource.URN,
	olds, news resource.PropertyMap, _ bool) (resource.PropertyMap, []plugin.CheckFailure, error) {
	return prov.check(urn, olds, news)
}
func (prov *testProvider) Create(urn resource.URN, props resource.PropertyMap) (resource.ID,
	resource.PropertyMap, resource.Status, error) {
	return prov.create(urn, props)
}
func (prov *testProvider) Read(urn resource.URN, id resource.ID,
	props resource.PropertyMap) (resource.PropertyMap, error) {
	return prov.read(urn, id, props)
}
func (prov *testProvider) Diff(urn resource.URN, id resource.ID,
	olds resource.PropertyMap, news resource.PropertyMap, _ bool) (plugin.DiffResult, error) {
	return prov.diff(urn, id, olds, news)
}
func (prov *testProvider) Update(urn resource.URN, id resource.ID,
	olds resource.PropertyMap, news resource.PropertyMap) (resource.PropertyMap, resource.Status, error) {
	return prov.update(urn, id, olds, news)
}
func (prov *testProvider) Delete(urn resource.URN,
	id resource.ID, props resource.PropertyMap) (resource.Status, error) {
	return prov.delete(urn, id, props)
}
func (prov *testProvider) Invoke(tok tokens.ModuleMember,
	args resource.PropertyMap) (resource.PropertyMap, []plugin.CheckFailure, error) {
	return prov.invoke(tok, args)
}
func (prov *testProvider) GetPluginInfo() (workspace.PluginInfo, error) {
	return workspace.PluginInfo{
		Name: "testProvider",
	}, nil
}
