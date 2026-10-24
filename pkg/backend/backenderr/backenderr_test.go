// Copyright 2019, Pulumi Corporation.
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

package backenderr

import (
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/registry"
	"github.com/stretchr/testify/assert"
)

func TestNotFoundErrorIs(t *testing.T) {
	t.Parallel()
	assert.ErrorIs(t, ErrNotFound, registry.ErrNotFound)
}

func TestStackNotFoundError(t *testing.T) {
	t.Parallel()

	assert.EqualError(t,
		StackNotFoundError{StackName: "dev"},
		"no stack named 'dev' found")
	assert.EqualError(t,
		StackNotFoundError{StackName: "dev", OfferCreate: true},
		"no stack named 'dev' found; use `pulumi stack select dev --create` to create it")
}

func TestNoStackSelectedError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		hasStacks bool
		expected  string
	}{
		{
			name:     "no stacks exist",
			expected: "no stack selected; use `pulumi stack init <name>` to create one",
		},
		{
			name:      "stacks exist",
			hasStacks: true,
			expected: "no stack selected; use `pulumi stack select <name>` to choose one " +
				"(list available stacks with `pulumi stack ls`)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.EqualError(t, NoStackSelectedError{HasStacks: tt.hasStacks}, tt.expected)
		})
	}
}
