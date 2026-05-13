// Copyright 2026, Pulumi Corporation.
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

package ui

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRenderer(t *testing.T) {
	t.Parallel()

	rs := OutputRenderers[string]{Default: "default", JSON: "json"}

	tests := []struct {
		give string
		want string
	}{
		{"", "default"},
		{"default", "default"},
		{"json", "json"},
	}
	for _, tt := range tests {
		t.Run(tt.give, func(t *testing.T) {
			t.Parallel()

			got, err := Renderer(tt.give, rs)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestRenderer_Invalid(t *testing.T) {
	t.Parallel()

	rs := OutputRenderers[string]{Default: "default", JSON: "json"}

	_, err := Renderer("xml", rs)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid --output value")
	require.Contains(t, err.Error(), "xml")
}
