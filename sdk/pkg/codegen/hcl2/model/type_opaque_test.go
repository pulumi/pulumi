// Copyright 2022-2024, Pulumi Corporation.
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

package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOpaqueEquality(t *testing.T) {
	t.Parallel()

	x := NewOpaqueType("x")
	x2 := NewOpaqueType("x")

	assert.True(t, x.Equals(x2))
	assert.True(t, x2.Equals(x))

	assert.True(t, x.equals(x2, map[Type]struct{}{}))
	assert.True(t, x2.equals(x, map[Type]struct{}{}))
}

func TestOpaqueInequality(t *testing.T) {
	t.Parallel()

	x := NewOpaqueType("x")
	y := NewOpaqueType("y")

	assert.False(t, x.Equals(y))
	assert.False(t, y.Equals(x))

	assert.False(t, x.equals(y, map[Type]struct{}{}))
	assert.False(t, y.equals(x, map[Type]struct{}{}))
}
