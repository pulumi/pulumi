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

package graph

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEmptySet(t *testing.T) {
	set := NewResourceSet()
	assert.True(t, set.Empty())
	assert.Nil(t, set.Elements())
}

func TestSingleton(t *testing.T) {
	r := NewResource("a", nil)
	set := NewResourceSet()
	set.Add(r)
	assert.False(t, set.Empty())
	assert.Len(t, set.Elements(), 1)
	assert.Contains(t, set.Elements(), r)
	assert.True(t, set.Test(r))
}

func TestRemove(t *testing.T) {
	r := NewResource("a", nil)
	set := NewResourceSet()
	set.Add(r)
	assert.False(t, set.Empty())
	assert.Len(t, set.Elements(), 1)
	assert.Contains(t, set.Elements(), r)
	assert.True(t, set.Test(r))
	set.Remove(r)
	assert.True(t, set.Empty())
	assert.Nil(t, set.Elements())
	assert.False(t, set.Test(r))
}

func TestIntersect(t *testing.T) {
	a := NewResource("a", nil)
	b := NewResource("b", nil)
	c := NewResource("c", nil)

	setA := NewResourceSet()
	setA.Add(a)
	setA.Add(b)
	setB := NewResourceSet()
	setB.Add(b)
	setB.Add(c)

	setC := setA.Intersect(setB)
	assert.False(t, setC.Test(a))
	assert.True(t, setC.Test(b))
	assert.False(t, setC.Test(c))
}

func TestNilInMap(t *testing.T) {
	set := NewResourceSet()
	set.Add(nil)
	assert.True(t, set.Empty())
	assert.False(t, set.Test(nil))
	set.Remove(nil)
	assert.True(t, set.Empty())
}
