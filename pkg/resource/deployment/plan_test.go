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

package deployment

import (
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"

	"github.com/pulumi/lumi/pkg/resource"
	"github.com/pulumi/lumi/pkg/resource/plugin"
	"github.com/pulumi/lumi/pkg/tokens"
	"github.com/pulumi/lumi/pkg/util/cmdutil"
)

// TestBasicCRUDPlan creates a plan with numerous C(R)UD operations.
func TestBasicCRUDPlan(t *testing.T) {
	t.Parallel()

	// Setup a fake namespace/target combination.
	ns := tokens.QName("crud")
	pkg := tokens.Package("testcrud")
	mod := tokens.Module(string(pkg) + ":index")

	// Create a context that the snapshots and plan will use.
	ctx := plugin.NewContext(cmdutil.Sink(), &testProviderHost{
		provider: func(propkg tokens.Package) (plugin.Provider, error) {
			if propkg != pkg {
				return nil, errors.Errorf("Unexpected request to load package %v; expected just %v", propkg, pkg)
			}
			return &testProvider{
				check: func(t tokens.Type, props resource.PropertyMap) ([]plugin.CheckFailure, error) {
					return nil, nil // accept all changes.
				},
				inspectChange: func(t tokens.Type, id resource.ID, olds resource.PropertyMap,
					news resource.PropertyMap) ([]string, resource.PropertyMap, error) {
					return nil, nil, nil // accept all changes.
				},
				// we don't actually execute the plan, so there's no need to implement the other functions.
			}, nil
		},
	})

	// Some shared tokens and names.
	typA := tokens.Type(string(mod) + ":A")
	namA := tokens.QName("res-a")
	urnA := resource.NewURN(ns, mod, typA, namA)
	typB := tokens.Type(string(mod) + ":B")
	namB := tokens.QName("res-b")
	urnB := resource.NewURN(ns, mod, typB, namB)
	typC := tokens.Type(string(mod) + ":C")
	namC := tokens.QName("res-c")
	urnC := resource.NewURN(ns, mod, typC, namC)
	typD := tokens.Type(string(mod) + ":D")
	namD := tokens.QName("res-d")
	urnD := resource.NewURN(ns, mod, typD, namD)

	// Create the old resources snapshot.
	oldResB := resource.NewState(typB, urnB, resource.ID("b-b-b"), resource.PropertyMap{
		"bf1": resource.NewStringProperty("b-value"),
		"bf2": resource.NewNumberProperty(42),
	}, nil)
	oldResC := resource.NewState(typC, urnC, resource.ID("c-c-c"), resource.PropertyMap{
		"cf1": resource.NewStringProperty("c-value"),
		"cf2": resource.NewNumberProperty(83),
	}, nil)
	oldResD := resource.NewState(typD, urnD, resource.ID("d-d-d"), resource.PropertyMap{
		"df1": resource.NewStringProperty("d-value"),
		"df2": resource.NewNumberProperty(167),
	}, nil)
	oldsnap := NewSnapshot(ns, pkg, nil, []*resource.State{oldResB, oldResC, oldResD})

	// Create the new resources snapshot.
	//     - A is created:
	newResA := NewResource(resource.ID(""), urnA, typA, resource.PropertyMap{
		"af1": resource.NewStringProperty("a-value"),
		"af2": resource.NewNumberProperty(42),
	}, nil)
	//     - B is updated:
	newResB := NewResource(resource.ID(""), urnB, typB, resource.PropertyMap{
		"bf1": resource.NewStringProperty("b-value"),
		// delete the bf2 field.
		"bf3": resource.NewBoolProperty(true), // add the bf3.
	}, nil)
	//     - C has no changes:
	newResC := NewResource(resource.ID(""), urnC, typC, resource.PropertyMap{
		"cf1": resource.NewStringProperty("c-value"),
		"cf2": resource.NewNumberProperty(83),
	}, nil)
	//     - No D; it is deleted.
	newsnap := NewSnapshot(ctx, ns, pkg, nil, []Resource{newResA, newResB, newResC})

	// Next up, create a plan from the two snapshots, and validate its shape.
	plan, err := NewPlan(ctx, oldsnap, newsnap, nil)
	assert.Nil(t, err)
	assert.False(t, plan.Empty())
	assert.Equal(t, 0, len(plan.Replaces()))
	assert.Equal(t, 1, len(plan.Unchanged()))
	assert.Equal(t, newResC, plan.Unchanged()[oldResC])

	// Next, validate the steps and ensure that we see all of the expected ones.  Note that there aren't any
	// dependencies between the steps, so we must validate it in a way that's insensitive of order.
	seen := make(map[StepOp]int)
	step := plan.Steps()
	assert.NotNil(t, step)
	for step != nil {
		op := step.Op()
		switch op {
		case OpCreate: // A is created
			assert.Nil(t, step.Old())
			assert.Equal(t, newResA, step.New())
			assert.False(t, step.Logical())
		case OpUpdate: // B is updated
			assert.Equal(t, oldResB, step.Old())
			assert.Equal(t, newResB, step.New())
			assert.False(t, step.Logical())
		case OpDelete: // D is deleted
			assert.Equal(t, oldResD, step.Old())
			assert.Nil(t, step.New())
			assert.False(t, step.Logical())
		}

		seen[op]++         // track the # of these we've seen so we can validate.
		step = step.Next() // proceed to the next one.
	}
	assert.Equal(t, 1, seen[OpCreate])
	assert.Equal(t, 0, seen[OpRead])
	assert.Equal(t, 1, seen[OpUpdate])
	assert.Equal(t, 1, seen[OpDelete])
	assert.Equal(t, 0, seen[OpReplace])
	assert.Equal(t, 0, seen[OpReplaceCreate])
	assert.Equal(t, 0, seen[OpReplaceDelete])
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
		resource.PropertyMap, resource.PropertyMap) ([]string, resource.PropertyMap, error)
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
	olds resource.PropertyMap, news resource.PropertyMap) ([]string, resource.PropertyMap, error) {
	return prov.inspectChange(t, id, olds, news)
}
func (prov *testProvider) Update(t tokens.Type, id resource.ID,
	olds resource.PropertyMap, news resource.PropertyMap) (resource.Status, error) {
	return prov.update(t, id, olds, news)
}
func (prov *testProvider) Delete(t tokens.Type, id resource.ID) (resource.Status, error) {
	return prov.delete(res)
}
