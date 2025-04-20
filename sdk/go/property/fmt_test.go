// Copyright 2016-2025, Pulumi Corporation.
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

package property

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/urn"
	"github.com/stretchr/testify/assert"
)

func TestGoStringValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value    Value
		expected string
	}{
		{
			value:    New(true),
			expected: `property.New(true)`,
		},
		{
			value:    New(123.0),
			expected: `property.New(123.0)`,
		},
		{
			value:    New(0.0),
			expected: `property.New(0.0)`,
		},
		{
			value:    New(Null),
			expected: `property.New(property.Null)`,
		},
		{
			value:    New(Computed),
			expected: `property.New(property.Computed)`,
		},
		{
			value:    New([]Value{}),
			expected: `property.New(property.Array{})`,
		},
		{
			value:    New([]Value{New(true), New(1.23)}),
			expected: `property.New([]property.Value{property.New(true), property.New(1.23)})`,
		},
		{
			value:    New(map[string]Value{}),
			expected: `property.New(property.Map{})`,
		},
		{
			value:    New(map[string]Value{"key": New(false)}),
			expected: `property.New(map[string]property.Value{"key":property.New(false)})`,
		},
		{
			value:    New(true).WithSecret(true),
			expected: `property.New(true).WithSecret(true)`,
		},
		{
			value:    New("s").WithDependencies([]urn.URN{"urn1", "urn2"}),
			expected: `property.New("s").WithDependencies([]urn.URN{"urn1", "urn2"})`,
		},
		{
			value:    New("s").WithDependencies([]urn.URN{"urn1"}).WithSecret(true),
			expected: `property.New("s").WithSecret(true).WithDependencies([]urn.URN{"urn1"})`,
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, tt.value.GoString())
		})
	}
}

func TestGoStringArray(t *testing.T) {
	t.Parallel()

	t.Run("empty", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, `property.Array{}`, (Array{}).GoString())
	})

	t.Run("not empty", func(t *testing.T) {
		t.Parallel()

		const expt = `property.NewArray([]property.Value{property.New(true), property.New(false)})`
		assert.Equal(t, expt, NewArray([]Value{New(true), New(false)}).GoString())
	})
}

func TestGoStringMap(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, `property.Map{}`, (Map{}).GoString())
	})

	t.Run("not empty", func(t *testing.T) {
		t.Parallel()

		const expt = `property.NewMap(map[string]property.Value{"k1":property.New(true), "k2":property.New(false)})`
		assert.Equal(t, expt, NewMap(map[string]Value{"k1": New(true), "k2": New(false)}).GoString())
	})
}
