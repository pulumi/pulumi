// Copyright 2016-2022, Pulumi Corporation.
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
	"fmt"
	"reflect"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
)

func TestStateBuilder(t *testing.T) {
	t.Parallel()

	t.Run("Update parent, no-op", func(t *testing.T) {
		t.Parallel()

		// Arrange.
		s0 := &resource.State{
			URN:      "urn:pulumi:stack::project::type:name",
			Provider: "urn:pulumi:providers::pkgA::prov::v1",
			Dependencies: []resource.URN{
				"urn:pulumi:stack::project::type:name::depA",
				"urn:pulumi:stack::project::type:name::depB",
			},
			PropertyDependencies: map[resource.PropertyKey][]resource.URN{
				"propA": {
					"urn:pulumi:stack::project::type:name::propDepA",
					"urn:pulumi:stack::project::type:name::propDepB",
				},
			},
			DeletedWith: "urn:pulumi:stack::project::type:name::deletedWith",
		}

		sBefore := s0.Copy()

		// Act.
		//
		// Parent is missing and we don't target other dependency types, so this should be a no-op.
		sAfter := newStateBuilder(sBefore).withAllUpdatedDependencies(
			identity,  /*updateProviderRef*/
			panicWith, /*updateURN*/
			justParent,
		).build()

		// Assert.
		assert.Same(t, sBefore, sAfter)
		assert.Equal(t, s0, sAfter)
		assert.Equal(
			t,
			reflect.ValueOf(s0.Dependencies).Pointer(),
			reflect.ValueOf(sAfter.Dependencies).Pointer(),
		)
		assert.Equal(
			t,
			reflect.ValueOf(s0.PropertyDependencies).Pointer(),
			reflect.ValueOf(sAfter.PropertyDependencies).Pointer(),
		)
	})

	t.Run("Update parent, identity", func(t *testing.T) {
		t.Parallel()

		// Arrange.
		s0 := &resource.State{
			URN:      "urn:pulumi:stack::project::type:name",
			Provider: "urn:pulumi:providers::pkgA::prov::v1",
			Parent:   "urn:pulumi:stack::project::type:name::parent",
			Dependencies: []resource.URN{
				"urn:pulumi:stack::project::type:name::depA",
				"urn:pulumi:stack::project::type:name::depB",
			},
			PropertyDependencies: map[resource.PropertyKey][]resource.URN{
				"propA": {
					"urn:pulumi:stack::project::type:name::propDepA",
					"urn:pulumi:stack::project::type:name::propDepB",
				},
			},
			DeletedWith: "urn:pulumi:stack::project::type:name::deletedWith",
		}

		sBefore := s0.Copy()

		// Act.
		//
		// Parent will not change since we are passing identity, and we don't target other dependency types, so this should
		// be a no-op.
		sAfter := newStateBuilder(sBefore).withAllUpdatedDependencies(
			identity, /*updateProviderRef*/
			identity, /*updateURN*/
			justParent,
		).build()

		// Assert.
		assert.Same(t, sBefore, sAfter)
		assert.Equal(t, s0, sAfter)
		assert.Equal(
			t,
			reflect.ValueOf(s0.Dependencies).Pointer(),
			reflect.ValueOf(sAfter.Dependencies).Pointer(),
		)
		assert.Equal(
			t,
			reflect.ValueOf(s0.PropertyDependencies).Pointer(),
			reflect.ValueOf(sAfter.PropertyDependencies).Pointer(),
		)
	})

	t.Run("Update parent, modify", func(t *testing.T) {
		t.Parallel()

		// Arrange.
		s0 := &resource.State{
			URN:      "urn:pulumi:stack::project::type:name",
			Provider: "urn:pulumi:providers::pkgA::prov::v1",
			Parent:   "urn:pulumi:stack::project::type:name::parent",
			Dependencies: []resource.URN{
				"urn:pulumi:stack::project::type:name::depA",
				"urn:pulumi:stack::project::type:name::depB",
			},
			PropertyDependencies: map[resource.PropertyKey][]resource.URN{
				"propA": {
					"urn:pulumi:stack::project::type:name::propDepA",
					"urn:pulumi:stack::project::type:name::propDepB",
				},
			},
			DeletedWith: "urn:pulumi:stack::project::type:name::deletedWith",
		}

		sBefore := s0.Copy()

		// Act.
		//
		// Parent will change, so we'll get a new pointer. Everything else should stay the same.
		sAfter := newStateBuilder(sBefore).withAllUpdatedDependencies(
			identity,  /*updateProviderRef*/
			emphasize, /*updateURN*/
			justParent,
		).build()

		// Assert.
		assert.NotSame(t, sBefore, sAfter)
		assert.Equal(t, s0.Provider, sAfter.Provider)
		assert.Equal(t, emphasize(s0.Parent), sAfter.Parent)
		assert.Equal(
			t,
			reflect.ValueOf(s0.Dependencies).Pointer(),
			reflect.ValueOf(sAfter.Dependencies).Pointer(),
		)
		assert.Equal(
			t,
			reflect.ValueOf(s0.PropertyDependencies).Pointer(),
			reflect.ValueOf(sAfter.PropertyDependencies).Pointer(),
		)
		assert.Equal(t, s0.DeletedWith, sAfter.DeletedWith)
	})

	t.Run("Update provider, no-op", func(t *testing.T) {
		t.Parallel()

		// Arrange.
		s0 := &resource.State{
			URN:    "urn:pulumi:stack::project::type:name",
			Parent: "urn:pulumi:stack::project::type:name::parent",
			Dependencies: []resource.URN{
				"urn:pulumi:stack::project::type:name::depA",
				"urn:pulumi:stack::project::type:name::depB",
			},
			PropertyDependencies: map[resource.PropertyKey][]resource.URN{
				"propA": {
					"urn:pulumi:stack::project::type:name::propDepA",
					"urn:pulumi:stack::project::type:name::propDepB",
				},
			},
			DeletedWith: "urn:pulumi:stack::project::type:name::deletedWith",
		}

		sBefore := s0.Copy()

		// Act.
		//
		// Provider is missing and we don't target other dependency types, so this should be a no-op.
		sAfter := newStateBuilder(sBefore).withAllUpdatedDependencies(
			panicWith, /*updateProviderRef*/
			panicWith, /*updateURN*/
			justProvider,
		).build()

		// Assert.
		assert.Same(t, sBefore, sAfter)
		assert.Equal(t, s0, sAfter)
		assert.Equal(
			t,
			reflect.ValueOf(s0.Dependencies).Pointer(),
			reflect.ValueOf(sAfter.Dependencies).Pointer(),
		)
		assert.Equal(
			t,
			reflect.ValueOf(s0.PropertyDependencies).Pointer(),
			reflect.ValueOf(sAfter.PropertyDependencies).Pointer(),
		)
	})

	t.Run("Update provider, identity", func(t *testing.T) {
		t.Parallel()

		// Arrange.
		s0 := &resource.State{
			URN:      "urn:pulumi:stack::project::type:name",
			Provider: "urn:pulumi:providers::pkgA::prov::v1",
			Parent:   "urn:pulumi:stack::project::type:name::parent",
			Dependencies: []resource.URN{
				"urn:pulumi:stack::project::type:name::depA",
				"urn:pulumi:stack::project::type:name::depB",
			},
			PropertyDependencies: map[resource.PropertyKey][]resource.URN{
				"propA": {
					"urn:pulumi:stack::project::type:name::propDepA",
					"urn:pulumi:stack::project::type:name::propDepB",
				},
			},
			DeletedWith: "urn:pulumi:stack::project::type:name::deletedWith",
		}

		sBefore := s0.Copy()

		// Act.
		//
		// Provider will not change since we are passing identity, and we don't target other dependency types, so this
		// should be a no-op.
		sAfter := newStateBuilder(sBefore).withAllUpdatedDependencies(
			identity,  /*updateProviderRef*/
			panicWith, /*updateURN*/
			justProvider,
		).build()

		// Assert.
		assert.Same(t, sBefore, sAfter)
		assert.Equal(t, s0, sAfter)
		assert.Equal(
			t,
			reflect.ValueOf(s0.Dependencies).Pointer(),
			reflect.ValueOf(sAfter.Dependencies).Pointer(),
		)
		assert.Equal(
			t,
			reflect.ValueOf(s0.PropertyDependencies).Pointer(),
			reflect.ValueOf(sAfter.PropertyDependencies).Pointer(),
		)
	})

	t.Run("Update provider, modify", func(t *testing.T) {
		t.Parallel()

		// Arrange.
		s0 := &resource.State{
			URN:      "urn:pulumi:stack::project::type:name",
			Provider: "urn:pulumi:providers::pkgA::prov::v1",
			Parent:   "urn:pulumi:stack::project::type:name::parent",
			Dependencies: []resource.URN{
				"urn:pulumi:stack::project::type:name::depA",
				"urn:pulumi:stack::project::type:name::depB",
			},
			PropertyDependencies: map[resource.PropertyKey][]resource.URN{
				"propA": {
					"urn:pulumi:stack::project::type:name::propDepA",
					"urn:pulumi:stack::project::type:name::propDepB",
				},
			},
			DeletedWith: "urn:pulumi:stack::project::type:name::deletedWith",
		}

		sBefore := s0.Copy()

		// Act.
		//
		// Provider will change, so we'll get a new pointer. Everything else should stay the same.
		sAfter := newStateBuilder(sBefore).withAllUpdatedDependencies(
			emphasize, /*updateProviderRef*/
			panicWith, /*updateURN*/
			justProvider,
		).build()

		// Assert.
		assert.NotSame(t, sBefore, sAfter)
		assert.Equal(t, emphasize(s0.Provider), sAfter.Provider)
		assert.Equal(t, s0.Parent, sAfter.Parent)
		assert.Equal(
			t,
			reflect.ValueOf(s0.Dependencies).Pointer(),
			reflect.ValueOf(sAfter.Dependencies).Pointer(),
		)
		assert.Equal(
			t,
			reflect.ValueOf(s0.PropertyDependencies).Pointer(),
			reflect.ValueOf(sAfter.PropertyDependencies).Pointer(),
		)
		assert.Equal(t, s0.DeletedWith, sAfter.DeletedWith)
	})

	t.Run("Update dependencies, no-op", func(t *testing.T) {
		t.Parallel()

		// Arrange.
		s0 := &resource.State{
			URN:      "urn:pulumi:stack::project::type:name",
			Provider: "urn:pulumi:providers::pkgA::prov::v1",
			Parent:   "urn:pulumi:stack::project::type:name::parent",
			PropertyDependencies: map[resource.PropertyKey][]resource.URN{
				"propA": {
					"urn:pulumi:stack::project::type:name::propDepA",
					"urn:pulumi:stack::project::type:name::propDepB",
				},
			},
			DeletedWith: "urn:pulumi:stack::project::type:name::deletedWith",
		}

		sBefore := s0.Copy()

		// Act.
		//
		// Dependencies are missing and we don't target other dependency types, so this should be a no-op.
		sAfter := newStateBuilder(sBefore).withAllUpdatedDependencies(
			identity,  /*updateProviderRef*/
			panicWith, /*updateURN*/
			justDependencies,
		).build()

		// Assert.
		assert.Same(t, sBefore, sAfter)
		assert.Equal(t, s0, sAfter)
		assert.Equal(
			t,
			reflect.ValueOf(s0.Dependencies).Pointer(),
			reflect.ValueOf(sAfter.Dependencies).Pointer(),
		)
		assert.Equal(
			t,
			reflect.ValueOf(s0.PropertyDependencies).Pointer(),
			reflect.ValueOf(sAfter.PropertyDependencies).Pointer(),
		)
	})

	t.Run("Update dependencies, identity", func(t *testing.T) {
		t.Parallel()

		// Arrange.
		s0 := &resource.State{
			URN:      "urn:pulumi:stack::project::type:name",
			Provider: "urn:pulumi:providers::pkgA::prov::v1",
			Parent:   "urn:pulumi:stack::project::type:name::parent",
			Dependencies: []resource.URN{
				"urn:pulumi:stack::project::type:name::depA",
				"urn:pulumi:stack::project::type:name::depB",
			},
			PropertyDependencies: map[resource.PropertyKey][]resource.URN{
				"propA": {
					"urn:pulumi:stack::project::type:name::propDepA",
					"urn:pulumi:stack::project::type:name::propDepB",
				},
			},
			DeletedWith: "urn:pulumi:stack::project::type:name::deletedWith",
		}

		sBefore := s0.Copy()

		// Act.
		//
		// Dependencies will not change since we are passing identity, and we don't target other dependency types, so this
		// should be a no-op.
		sAfter := newStateBuilder(sBefore).withAllUpdatedDependencies(
			identity, /*updateProviderRef*/
			identity, /*updateURN*/
			justDependencies,
		).build()

		// Assert.
		assert.Same(t, sBefore, sAfter)
		assert.Equal(t, s0, sAfter)
		assert.Equal(
			t,
			reflect.ValueOf(s0.Dependencies).Pointer(),
			reflect.ValueOf(sAfter.Dependencies).Pointer(),
		)
		assert.Equal(
			t,
			reflect.ValueOf(s0.PropertyDependencies).Pointer(),
			reflect.ValueOf(sAfter.PropertyDependencies).Pointer(),
		)
	})

	t.Run("Update dependencies, modify", func(t *testing.T) {
		t.Parallel()

		// Arrange.
		s0 := &resource.State{
			URN:      "urn:pulumi:stack::project::type:name",
			Provider: "urn:pulumi:providers::pkgA::prov::v1",
			Parent:   "urn:pulumi:stack::project::type:name::parent",
			Dependencies: []resource.URN{
				"urn:pulumi:stack::project::type:name::depA",
				"urn:pulumi:stack::project::type:name::depB",
			},
			PropertyDependencies: map[resource.PropertyKey][]resource.URN{
				"propA": {
					"urn:pulumi:stack::project::type:name::propDepA",
					"urn:pulumi:stack::project::type:name::propDepB",
				},
			},
			DeletedWith: "urn:pulumi:stack::project::type:name::deletedWith",
		}

		sBefore := s0.Copy()

		// Act.
		//
		// Dependencies will change, so we'll get a new pointer. Everything else should stay the same.
		sAfter := newStateBuilder(sBefore).withAllUpdatedDependencies(
			identity,  /*updateProviderRef*/
			emphasize, /*updateURN*/
			justDependencies,
		).build()

		// Assert.
		assert.NotSame(t, sBefore, sAfter)
		assert.Equal(t, s0.Provider, sAfter.Provider)
		assert.Equal(t, s0.Parent, sAfter.Parent)
		assert.Equal(t, emphasize(s0.Dependencies[0]), sAfter.Dependencies[0])
		assert.Equal(
			t,
			reflect.ValueOf(s0.PropertyDependencies).Pointer(),
			reflect.ValueOf(sAfter.PropertyDependencies).Pointer(),
		)
		assert.Equal(t, s0.DeletedWith, sAfter.DeletedWith)
	})

	t.Run("Update property dependencies, no-op", func(t *testing.T) {
		t.Parallel()

		// Arrange.
		s0 := &resource.State{
			URN:      "urn:pulumi:stack::project::type:name",
			Provider: "urn:pulumi:providers::pkgA::prov::v1",
			Parent:   "urn:pulumi:stack::project::type:name::parent",
			Dependencies: []resource.URN{
				"urn:pulumi:stack::project::type:name::depA",
				"urn:pulumi:stack::project::type:name::depB",
			},
			DeletedWith: "urn:pulumi:stack::project::type:name::deletedWith",
		}

		sBefore := s0.Copy()

		// Act.
		//
		// Property dependencies are missing and we don't target other dependency types, so this should be a no-op.
		sAfter := newStateBuilder(sBefore).withAllUpdatedDependencies(
			identity,  /*updateProviderRef*/
			panicWith, /*updateURN*/
			justPropertyDependencies,
		).build()

		// Assert.
		assert.Same(t, sBefore, sAfter)
		assert.Equal(t, s0, sAfter)
		assert.Equal(
			t,
			reflect.ValueOf(s0.Dependencies).Pointer(),
			reflect.ValueOf(sAfter.Dependencies).Pointer(),
		)
		assert.Equal(
			t,
			reflect.ValueOf(s0.PropertyDependencies).Pointer(),
			reflect.ValueOf(sAfter.PropertyDependencies).Pointer(),
		)
	})

	t.Run("Update property dependencies, identity", func(t *testing.T) {
		t.Parallel()

		// Arrange.
		s0 := &resource.State{
			URN:      "urn:pulumi:stack::project::type:name",
			Provider: "urn:pulumi:providers::pkgA::prov::v1",
			Parent:   "urn:pulumi:stack::project::type:name::parent",
			Dependencies: []resource.URN{
				"urn:pulumi:stack::project::type:name::depA",
				"urn:pulumi:stack::project::type:name::depB",
			},
			PropertyDependencies: map[resource.PropertyKey][]resource.URN{
				"propA": {
					"urn:pulumi:stack::project::type:name::propDepA",
					"urn:pulumi:stack::project::type:name::propDepB",
				},
			},
			DeletedWith: "urn:pulumi:stack::project::type:name::deletedWith",
		}

		sBefore := s0.Copy()

		// Act.
		//
		// Property dependencies will not change since we are passing identity, and we don't target other dependency types,
		// so this should be a no-op.
		sAfter := newStateBuilder(sBefore).withAllUpdatedDependencies(
			identity, /*updateProviderRef*/
			identity, /*updateURN*/
			justPropertyDependencies,
		).build()

		// Assert.
		assert.Same(t, sBefore, sAfter)
		assert.Equal(t, s0, sAfter)
		assert.Equal(
			t,
			reflect.ValueOf(s0.Dependencies).Pointer(),
			reflect.ValueOf(sAfter.Dependencies).Pointer(),
		)
		assert.Equal(
			t,
			reflect.ValueOf(s0.PropertyDependencies).Pointer(),
			reflect.ValueOf(sAfter.PropertyDependencies).Pointer(),
		)
	})

	t.Run("Update property dependencies, modify", func(t *testing.T) {
		t.Parallel()

		// Arrange.
		s0 := &resource.State{
			URN:      "urn:pulumi:stack::project::type:name",
			Provider: "urn:pulumi:providers::pkgA::prov::v1",
			Parent:   "urn:pulumi:stack::project::type:name::parent",
			Dependencies: []resource.URN{
				"urn:pulumi:stack::project::type:name::depA",
				"urn:pulumi:stack::project::type:name::depB",
			},
			PropertyDependencies: map[resource.PropertyKey][]resource.URN{
				"propA": {
					"urn:pulumi:stack::project::type:name::propDepA",
					"urn:pulumi:stack::project::type:name::propDepB",
				},
			},
			DeletedWith: "urn:pulumi:stack::project::type:name::deletedWith",
		}

		sBefore := s0.Copy()

		// Act.
		//
		// Property dependencies will change, so we'll get a new pointer. Everything else should stay the same.
		sAfter := newStateBuilder(sBefore).withAllUpdatedDependencies(
			identity,  /*updateProviderRef*/
			emphasize, /*updateURN*/
			justPropertyDependencies,
		).build()

		// Assert.
		assert.NotSame(t, sBefore, sAfter)
		assert.Equal(t, s0.Provider, sAfter.Provider)
		assert.Equal(t, s0.Parent, sAfter.Parent)
		assert.Equal(
			t,
			reflect.ValueOf(s0.Dependencies).Pointer(),
			reflect.ValueOf(sAfter.Dependencies).Pointer(),
		)
		assert.Equal(t, emphasize(s0.PropertyDependencies["propA"][0]), sAfter.PropertyDependencies["propA"][0])
		assert.Equal(t, s0.DeletedWith, sAfter.DeletedWith)
	})

	t.Run("Update deleted with, no-op", func(t *testing.T) {
		t.Parallel()

		// Arrange.
		s0 := &resource.State{
			URN:      "urn:pulumi:stack::project::type:name",
			Provider: "urn:pulumi:providers::pkgA::prov::v1",
			Parent:   "urn:pulumi:stack::project::type:name::parent",
			Dependencies: []resource.URN{
				"urn:pulumi:stack::project::type:name::depA",
				"urn:pulumi:stack::project::type:name::depB",
			},
			PropertyDependencies: map[resource.PropertyKey][]resource.URN{
				"propA": {
					"urn:pulumi:stack::project::type:name::propDepA",
					"urn:pulumi:stack::project::type:name::propDepB",
				},
			},
		}

		sBefore := s0.Copy()

		// Act.
		//
		// Deleted with is missing and we don't target other dependency types, so this should be a no-op.
		sAfter := newStateBuilder(sBefore).withAllUpdatedDependencies(
			identity,  /*updateProviderRef*/
			panicWith, /*updateURN*/
			justDeletedWith,
		).build()

		// Assert.
		assert.Same(t, sBefore, sAfter)
		assert.Equal(t, s0, sAfter)
		assert.Equal(
			t,
			reflect.ValueOf(s0.Dependencies).Pointer(),
			reflect.ValueOf(sAfter.Dependencies).Pointer(),
		)
		assert.Equal(
			t,
			reflect.ValueOf(s0.PropertyDependencies).Pointer(),
			reflect.ValueOf(sAfter.PropertyDependencies).Pointer(),
		)
	})

	t.Run("Update deleted with, identity", func(t *testing.T) {
		t.Parallel()

		// Arrange.
		s0 := &resource.State{
			URN:      "urn:pulumi:stack::project::type:name",
			Provider: "urn:pulumi:providers::pkgA::prov::v1",
			Parent:   "urn:pulumi:stack::project::type:name::parent",
			Dependencies: []resource.URN{
				"urn:pulumi:stack::project::type:name::depA",
				"urn:pulumi:stack::project::type:name::depB",
			},
			PropertyDependencies: map[resource.PropertyKey][]resource.URN{
				"propA": {
					"urn:pulumi:stack::project::type:name::propDepA",
					"urn:pulumi:stack::project::type:name::propDepB",
				},
			},
			DeletedWith: "urn:pulumi:stack::project::type:name::deletedWith",
		}

		sBefore := s0.Copy()

		// Act.
		//
		// Deleted with will not change since we are passing identity, and we don't target other dependency types, so this
		// should be a no-op.
		sAfter := newStateBuilder(sBefore).withAllUpdatedDependencies(
			identity, /*updateProviderRef*/
			identity, /*updateURN*/
			justDeletedWith,
		).build()

		// Assert.
		assert.Same(t, sBefore, sAfter)
		assert.Equal(t, s0, sAfter)
		assert.Equal(
			t,
			reflect.ValueOf(s0.Dependencies).Pointer(),
			reflect.ValueOf(sAfter.Dependencies).Pointer(),
		)
		assert.Equal(
			t,
			reflect.ValueOf(s0.PropertyDependencies).Pointer(),
			reflect.ValueOf(sAfter.PropertyDependencies).Pointer(),
		)
	})

	t.Run("Update deleted with, modify", func(t *testing.T) {
		t.Parallel()

		// Arrange.
		s0 := &resource.State{
			URN:      "urn:pulumi:stack::project::type:name",
			Provider: "urn:pulumi:providers::pkgA::prov::v1",
			Parent:   "urn:pulumi:stack::project::type:name::parent",
			Dependencies: []resource.URN{
				"urn:pulumi:stack::project::type:name::depA",
				"urn:pulumi:stack::project::type:name::depB",
			},
			PropertyDependencies: map[resource.PropertyKey][]resource.URN{
				"propA": {
					"urn:pulumi:stack::project::type:name::propDepA",
					"urn:pulumi:stack::project::type:name::propDepB",
				},
			},
			DeletedWith: "urn:pulumi:stack::project::type:name::deletedWith",
		}

		sBefore := s0.Copy()

		// Act.
		//
		// Deleted with will change, so we'll get a new pointer. Everything else should stay the same.
		sAfter := newStateBuilder(sBefore).withAllUpdatedDependencies(
			identity,  /*updateProviderRef*/
			emphasize, /*updateURN*/
			justDeletedWith,
		).build()

		// Assert.
		assert.NotSame(t, sBefore, sAfter)
		assert.Equal(t, s0.Provider, sAfter.Provider)
		assert.Equal(t, s0.Parent, sAfter.Parent)
		assert.Equal(
			t,
			reflect.ValueOf(s0.Dependencies).Pointer(),
			reflect.ValueOf(sAfter.Dependencies).Pointer(),
		)
		assert.Equal(
			t,
			reflect.ValueOf(s0.PropertyDependencies).Pointer(),
			reflect.ValueOf(sAfter.PropertyDependencies).Pointer(),
		)
		assert.Equal(t, emphasize(s0.DeletedWith), sAfter.DeletedWith)
	})
}

func justProvider(dep resource.StateDependency) bool {
	return false
}

func justParent(dep resource.StateDependency) bool {
	return dep.Type == resource.ResourceParent
}

func justDependencies(dep resource.StateDependency) bool {
	return dep.Type == resource.ResourceDependency
}

func justPropertyDependencies(dep resource.StateDependency) bool {
	return dep.Type == resource.ResourcePropertyDependency
}

func justDeletedWith(dep resource.StateDependency) bool {
	return dep.Type == resource.ResourceDeletedWith
}

func panicWith[T any](t T) T {
	panic(fmt.Sprintf("Intentional panic: received %v", t))
}

func identity[T any](t T) T {
	return t
}

func emphasize[T ~string](t T) T {
	return t + "!"
}
