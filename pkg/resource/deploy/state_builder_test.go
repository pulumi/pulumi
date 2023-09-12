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
	"reflect"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
)

func TestStateBuilder(t *testing.T) {
	t.Parallel()
	s0 := &resource.State{
		URN: "urn:pulumi:prod::st::cust:res:R::a-res",
	}

	s1 := &resource.State{
		URN:      "urn:pulumi:prod::st::cust:res:R$cust:res:R::a-res",
		Parent:   "urn:pulumi:prod::st::cust:res:R::my-res",
		Provider: "urn:pulumi:prod::st::cust:prov:P::my-prov",
		Dependencies: []resource.URN{
			"urn:pulumi:prod::st::cust:res:R::a",
			"urn:pulumi:prod::st::cust:res:R::b",
		},
		PropertyDependencies: map[resource.PropertyKey][]resource.URN{
			"id": {
				"urn:pulumi:prod::st::cust:res:R::a",
				"urn:pulumi:prod::st::cust:res:R::b",
			},
		},
	}

	identityURN := func(urn resource.URN) resource.URN {
		return urn
	}

	emphasizeURN := func(urn resource.URN) resource.URN {
		return urn + "!"
	}

	identityStr := func(urn string) string {
		return urn
	}

	emphasizeStr := func(urn string) string {
		return urn + "!"
	}

	panicURN := func(urn resource.URN) resource.URN {
		panic("INTENTIONAL")
	}

	panicStr := func(urn string) string {
		panic("INTENTIONAL")
	}

	samePtr := func(a, b interface{}) bool {
		return reflect.ValueOf(a).Pointer() == reflect.ValueOf(b).Pointer()
	}

	t.Run("withUpdatedParent", func(t *testing.T) {
		t.Parallel()
		s := newStateBuilder(s0).withUpdatedParent(panicURN).build()
		assert.Same(t, s0, s)

		s = newStateBuilder(s1).withUpdatedParent(identityURN).build()
		assert.Same(t, s, s1)
		assert.Equal(t, s1.Parent, s.Parent)

		s = newStateBuilder(s1).withUpdatedParent(emphasizeURN).build()
		assert.NotSame(t, s, s1)
		assert.Equal(t, emphasizeURN(s1.Parent), s.Parent)
	})

	t.Run("withUpdatedProvider", func(t *testing.T) {
		t.Parallel()
		s := newStateBuilder(s0).withUpdatedProvider(panicStr).build()
		assert.Same(t, s, s0)

		s = newStateBuilder(s1).withUpdatedProvider(identityStr).build()
		assert.Same(t, s, s1)
		assert.Equal(t, s1.Parent, s.Parent)

		s = newStateBuilder(s1).withUpdatedProvider(emphasizeStr).build()
		assert.NotSame(t, s, s1)
		assert.Equal(t, emphasizeStr(s1.Provider), s.Provider)
	})

	t.Run("withUpdatedDependencies", func(t *testing.T) {
		t.Parallel()
		s := newStateBuilder(s0).withUpdatedDependencies(panicURN).build()
		assert.Same(t, s, s0)

		s = newStateBuilder(s1).withUpdatedDependencies(identityURN).build()
		assert.Same(t, s, s1)
		assert.True(t, samePtr(s.Dependencies, s1.Dependencies))

		s = newStateBuilder(s1).withUpdatedDependencies(emphasizeURN).build()
		assert.NotSame(t, s, s1)
		assert.Equal(t, s.Dependencies[0], emphasizeURN(s1.Dependencies[0]))
	})

	t.Run("withUpdatedPropertyDependencies", func(t *testing.T) {
		t.Parallel()
		s := newStateBuilder(s0).withUpdatedPropertyDependencies(panicURN).build()
		assert.Same(t, s, s0)

		s = newStateBuilder(s1).withUpdatedPropertyDependencies(identityURN).build()
		assert.Same(t, s, s1)
		assert.True(t, samePtr(s.PropertyDependencies, s.PropertyDependencies))

		s = newStateBuilder(s1).withUpdatedPropertyDependencies(emphasizeURN).build()
		assert.NotSame(t, s, s1)
		assert.Equal(t, s.PropertyDependencies["id"][0],
			emphasizeURN(s1.PropertyDependencies["id"][0]))
	})
}
