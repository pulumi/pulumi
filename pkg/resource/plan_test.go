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

package resource

import (
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"

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
	ctx := NewContext(cmdutil.Sink(), &testProviderHost{
		provider: func(propkg tokens.Package) (Provider, error) {
			if propkg != pkg {
				return nil, errors.Errorf("Unexpected request to load package %v; expected just %v", propkg, pkg)
			}
			return &testProvider{
				check: func(res Resource) ([]CheckFailure, error) {
					return nil, nil // accept all changes.
				},
				inspectChange: func(old, new Resource, computed PropertyMap) ([]string, PropertyMap, error) {
					return nil, nil, nil // accept all changes.
				},
				// we don't actually execute the plan, so there's no need to implement the other functions.
			}, nil
		},
	})

	// Some shared tokens and names.
	typA := tokens.Type(string(mod) + ":A")
	namA := tokens.QName("res-a")
	urnA := NewURN(ns, mod, typA, namA)
	typB := tokens.Type(string(mod) + ":B")
	namB := tokens.QName("res-b")
	urnB := NewURN(ns, mod, typB, namB)
	typC := tokens.Type(string(mod) + ":C")
	namC := tokens.QName("res-c")
	urnC := NewURN(ns, mod, typC, namC)
	typD := tokens.Type(string(mod) + ":D")
	namD := tokens.QName("res-d")
	urnD := NewURN(ns, mod, typD, namD)

	// Create the old resources snapshot.
	oldResB := NewResource(ID("b-b-b"), urnB, typB, PropertyMap{
		"bf1": NewStringProperty("b-value"),
		"bf2": NewNumberProperty(42),
	})
	oldResC := NewResource(ID("c-c-c"), urnC, typC, PropertyMap{
		"cf1": NewStringProperty("c-value"),
		"cf2": NewNumberProperty(83),
	})
	oldResD := NewResource(ID("d-d-d"), urnD, typD, PropertyMap{
		"df1": NewStringProperty("d-value"),
		"df2": NewNumberProperty(167),
	})
	oldsnap := NewSnapshot(ctx, ns, pkg, nil, []Resource{oldResB, oldResC, oldResD})

	// Create the new resources snapshot.
	//     - A is created:
	newResA := NewResource(ID(""), urnA, typA, PropertyMap{
		"af1": NewStringProperty("a-value"),
		"af2": NewNumberProperty(42),
	})
	//     - B is updated:
	newResB := NewResource(ID(""), urnB, typB, PropertyMap{
		"bf1": NewStringProperty("b-value"),
		// delete the bf2 field.
		"bf3": NewBoolProperty(true), // add the bf3.
	})
	//     - C has no changes:
	newResC := NewResource(ID(""), urnC, typC, PropertyMap{
		"cf1": NewStringProperty("c-value"),
		"cf2": NewNumberProperty(83),
	})
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
	analyzer func(nm tokens.QName) (Analyzer, error)
	provider func(pkg tokens.Package) (Provider, error)
}

func (prov *testProviderHost) Close() error {
	return nil
}
func (host *testProviderHost) Analyzer(nm tokens.QName) (Analyzer, error) {
	return host.analyzer(nm)
}
func (host *testProviderHost) Provider(pkg tokens.Package) (Provider, error) {
	return host.provider(pkg)
}

type testProvider struct {
	pkg           tokens.Package
	check         func(Resource) ([]CheckFailure, error)
	name          func(Resource) (tokens.QName, error)
	create        func(Resource) (State, error)
	get           func(Resource) error
	inspectChange func(Resource, Resource, PropertyMap) ([]string, PropertyMap, error)
	update        func(Resource, Resource) (State, error)
	delete        func(Resource) (State, error)
}

func (prov *testProvider) Close() error {
	return nil
}
func (prov *testProvider) Pkg() tokens.Package {
	return prov.pkg
}
func (prov *testProvider) Check(res Resource) ([]CheckFailure, error) {
	return prov.check(res)
}
func (prov *testProvider) Name(res Resource) (tokens.QName, error) {
	return prov.name(res)
}
func (prov *testProvider) Create(res Resource) (State, error) {
	return prov.create(res)
}
func (prov *testProvider) Get(res Resource) error {
	return prov.get(res)
}
func (prov *testProvider) InspectChange(old Resource, new Resource,
	computed PropertyMap) ([]string, PropertyMap, error) {
	return prov.inspectChange(old, new, computed)
}
func (prov *testProvider) Update(old Resource, new Resource) (State, error) {
	return prov.update(old, new)
}
func (prov *testProvider) Delete(res Resource) (State, error) { return prov.delete(res) }
