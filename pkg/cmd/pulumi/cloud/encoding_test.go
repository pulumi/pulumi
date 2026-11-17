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

package cloud

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRequiresDoubleEncoding(t *testing.T) {
	t.Parallel()
	cases := []struct {
		paramName string
		want      bool
	}{
		{"accountName", true},
		{"resourceTypeAndId", true},
		{"object", true},
		{"orgName", false},
		{"projectName", false},
		{"stackName", false},
		{"source", false},
		{"publisher", false},
		{"name", false},
		{"version", false},
		{"", false},
	}
	for _, tc := range cases {
		got := requiresDoubleEncoding(tc.paramName)
		assert.Equal(t, tc.want, got, "paramName=%q", tc.paramName)
	}
}

func TestEscapePathParam(t *testing.T) {
	t.Parallel()
	cases := []struct {
		val    string
		double bool
		want   string
	}{
		{"acme", false, "acme"},
		{"acme", true, "acme"},
		{"my/org", false, "my%2Forg"},
		{"my/org", true, "my%252Forg"},
		{"a b", false, "a%20b"},
		{"a b", true, "a%2520b"},
		{"", false, ""},
		{"", true, ""},
	}
	for _, tc := range cases {
		got := escapePathParam(tc.val, tc.double)
		assert.Equal(t, tc.want, got, "val=%q double=%v", tc.val, tc.double)
	}
}
