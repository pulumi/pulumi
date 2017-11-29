// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package deploy

import (
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/plugin"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

// TestNullPlan creates a plan with no operations.
func TestNullPlan(t *testing.T) {
	t.Parallel()

	ctx, err := plugin.NewContext(cmdutil.Diag(), nil, nil)
	assert.Nil(t, err)
	targ := &Target{Name: tokens.QName("null")}
	prev := NewSnapshot(targ.Name, time.Now(), nil)
	plan := NewPlan(ctx, targ, prev, NullSource, nil)
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
		ctx, err := plugin.NewContext(cmdutil.Diag(), nil, nil)
		assert.Nil(t, err)
		targ := &Target{Name: tokens.QName("errs")}
		prev := NewSnapshot(targ.Name, time.Now(), nil)
		plan := NewPlan(ctx, targ, prev, &errorSource{err: errors.New("ITERATE"), duringIterate: true}, nil)
		iter, err := plan.Start(Options{})
		assert.Nil(t, iter)
		assert.NotNil(t, err)
		assert.Equal(t, "ITERATE", err.Error())
		err = ctx.Close()
		assert.Nil(t, err)
	}

	// Next trigger an error from Next:
	{
		ctx, err := plugin.NewContext(cmdutil.Diag(), nil, nil)
		assert.Nil(t, err)
		targ := &Target{Name: tokens.QName("errs")}
		prev := NewSnapshot(targ.Name, time.Now(), nil)
		plan := NewPlan(ctx, targ, prev, &errorSource{err: errors.New("NEXT"), duringIterate: false}, nil)
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

func (src *errorSource) Close() error {
	return nil // nothing to do.
}

func (src *errorSource) Pkg() tokens.PackageName {
	return ""
}

func (src *errorSource) Info() interface{} {
	return nil
}

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
		provider: func(propkg tokens.Package) (plugin.Provider, error) {
			if propkg != pkg {
				return nil, errors.Errorf("Unexpected request to load package %v; expected just %v", propkg, pkg)
			}
			return &testProvider{
				check: func(urn resource.URN,
					props resource.PropertyMap) (resource.PropertyMap, []plugin.CheckFailure, error) {
					return nil, nil, nil // accept all changes.
				},
				diff: func(urn resource.URN, id resource.ID, olds resource.PropertyMap,
					news resource.PropertyMap) (plugin.DiffResult, error) {
					return plugin.DiffResult{}, nil // accept all changes.
				},
				// we don't actually execute the plan, so there's no need to implement the other functions.
			}, nil
		},
	}, nil)
	assert.Nil(t, err)

	// Setup a fake namespace/target combination.
	targ := &Target{Name: tokens.QName("crud")}
	ns := targ.Name
	mod := tokens.Module(pkg + ":index")
	pkgname := pkg.Name()

	// Some shared tokens and names.
	typA := tokens.Type(mod + ":A")
	namA := tokens.QName("res-a")
	urnA := resource.NewURN(ns, pkgname, typA, namA)
	typB := tokens.Type(mod + ":B")
	namB := tokens.QName("res-b")
	urnB := resource.NewURN(ns, pkgname, typB, namB)
	typC := tokens.Type(mod + ":C")
	namC := tokens.QName("res-c")
	urnC := resource.NewURN(ns, pkgname, typC, namC)
	typD := tokens.Type(mod + ":D")
	namD := tokens.QName("res-d")
	urnD := resource.NewURN(ns, pkgname, typD, namD)

	// Create the old resources snapshot.
	oldResB := resource.NewState(typB, urnB, true, false, resource.ID("b-b-b"),
		resource.PropertyMap{
			"bf1": resource.NewStringProperty("b-value"),
			"bf2": resource.NewNumberProperty(42),
		},
		make(resource.PropertyMap),
		nil,
		"",
	)
	oldResC := resource.NewState(typC, urnC, true, false, resource.ID("c-c-c"),
		resource.PropertyMap{
			"cf1": resource.NewStringProperty("c-value"),
			"cf2": resource.NewNumberProperty(83),
		},
		make(resource.PropertyMap),
		resource.PropertyMap{
			"outta1":   resource.NewStringProperty("populated during skip/step"),
			"outta234": resource.NewNumberProperty(99881122),
		},
		"",
	)
	oldResD := resource.NewState(typD, urnD, true, false, resource.ID("d-d-d"),
		resource.PropertyMap{
			"df1": resource.NewStringProperty("d-value"),
			"df2": resource.NewNumberProperty(167),
		},
		make(resource.PropertyMap),
		nil,
		"",
	)
	oldsnap := NewSnapshot(ns, time.Now(), []*resource.State{oldResB, oldResC, oldResD})

	// Create the new resource objects a priori.
	//     - A is created:
	newResA := resource.NewGoal(typA, namA, true, resource.PropertyMap{
		"af1": resource.NewStringProperty("a-value"),
		"af2": resource.NewNumberProperty(42),
	}, "")
	newStateA := &testBeginReg{goal: newResA}
	//     - B is updated:
	newResB := resource.NewGoal(typB, namB, true, resource.PropertyMap{
		"bf1": resource.NewStringProperty("b-value"),
		// delete the bf2 field, and add bf3.
		"bf3": resource.NewBoolProperty(true),
	}, "")
	newStateB := &testBeginReg{goal: newResB}
	//     - C has no changes:
	newResC := resource.NewGoal(typC, namC, true, resource.PropertyMap{
		"cf1": resource.NewStringProperty("c-value"),
		"cf2": resource.NewNumberProperty(83),
	}, "")
	newStateC := &testBeginReg{goal: newResC}
	//     - No D; it is deleted.

	// Use a fixed source that just returns the above predefined objects during planning.
	source := NewFixedSource(pkgname, []SourceEvent{newStateA, newStateB, newStateC})

	// Next up, create a plan from the new and old, and validate its shape.
	plan := NewPlan(ctx, targ, oldsnap, source, nil)

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
			assert.Equal(t, newResA.Properties, new.AllInputs())
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
			assert.Equal(t, newResB.Properties, new.AllInputs())
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
			assert.Equal(t, newResC.Properties, new.AllInputs())
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

		var state *FinalState
		_, state, err = step.Apply(true)
		assert.Nil(t, err)

		op := step.Op()
		if state != nil {
			// The state should be non-nil by now and it should have a URN.
			assert.NotNil(t, state.State)

			// Ensure the URN, ID, and properties are populated correctly.
			assert.Equal(t, urn, state.State.URN,
				"Expected op %v to populate a URN equal to %v", op, urn)

			if realID {
				assert.NotEqual(t, resource.ID(""), state.State.ID,
					"Expected op %v to populate a real ID (%v)", op, urn)
			} else {
				assert.Equal(t, resource.ID(""), state.State.ID,
					"Expected op %v to leave ID blank (%v); got %v", op, urn, state.State.ID)
			}

			if expectOuts != nil {
				props := state.State.All()
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

type testBeginReg struct {
	goal *resource.Goal
	urn  resource.URN
}

var _ BeginRegisterResourceEvent = (*testBeginReg)(nil)

func (g *testBeginReg) event() {}

func (g *testBeginReg) Goal() *resource.Goal {
	return g.goal
}

func (g *testBeginReg) Done(urn resource.URN) {
	contract.Assertf(g.urn == "", "Attempt to invoke testBeginReg.Done more than once")
	g.urn = urn
}

type testProviderHost struct {
	analyzer func(nm tokens.QName) (plugin.Analyzer, error)
	provider func(pkg tokens.Package) (plugin.Provider, error)
	langhost func(runtime string, monitorAddr string) (plugin.LanguageRuntime, error)
}

func (host *testProviderHost) Close() error {
	return nil
}
func (host *testProviderHost) ServerAddr() string {
	contract.Failf("Host RPC address not available")
	return ""
}
func (host *testProviderHost) Log(sev diag.Severity, msg string) {
	cmdutil.Diag().Logf(sev, diag.RawMessage(msg))
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
func (host *testProviderHost) Provider(pkg tokens.Package) (plugin.Provider, error) {
	return host.provider(pkg)
}
func (host *testProviderHost) LanguageRuntime(runtime string, monitorAddr string) (plugin.LanguageRuntime, error) {
	return host.langhost(runtime, monitorAddr)
}

type testProvider struct {
	pkg    tokens.Package
	config func(map[tokens.ModuleMember]string) error
	check  func(resource.URN, resource.PropertyMap) (resource.PropertyMap, []plugin.CheckFailure, error)
	create func(resource.URN, resource.PropertyMap) (resource.ID, resource.PropertyMap, resource.Status, error)
	diff   func(resource.URN, resource.ID, resource.PropertyMap, resource.PropertyMap) (plugin.DiffResult, error)
	update func(resource.URN, resource.ID,
		resource.PropertyMap, resource.PropertyMap) (resource.PropertyMap, resource.Status, error)
	delete func(resource.URN, resource.ID, resource.PropertyMap) (resource.Status, error)
	invoke func(tokens.ModuleMember, resource.PropertyMap) (resource.PropertyMap, []plugin.CheckFailure, error)
}

func (prov *testProvider) Close() error {
	return nil
}
func (prov *testProvider) Pkg() tokens.Package {
	return prov.pkg
}
func (prov *testProvider) Configure(vars map[tokens.ModuleMember]string) error {
	return prov.config(vars)
}
func (prov *testProvider) Check(urn resource.URN,
	props resource.PropertyMap) (resource.PropertyMap, []plugin.CheckFailure, error) {
	return prov.check(urn, props)
}
func (prov *testProvider) Create(urn resource.URN, props resource.PropertyMap) (resource.ID,
	resource.PropertyMap, resource.Status, error) {
	return prov.create(urn, props)
}
func (prov *testProvider) Diff(urn resource.URN, id resource.ID,
	olds resource.PropertyMap, news resource.PropertyMap) (plugin.DiffResult, error) {
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
