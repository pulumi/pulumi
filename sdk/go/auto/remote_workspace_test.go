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

package auto

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsFullyQualifiedStackName(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		input    string
		expected bool
	}{
		"fully qualified": {input: "owner/project/stack", expected: true},
		"empty":           {input: "", expected: false},
		"name":            {input: "name", expected: false},
		"name & owner":    {input: "owner/name", expected: false},
		"sep":             {input: "/", expected: false},
		"two seps":        {input: "//", expected: false},
		"three seps":      {input: "///", expected: false},
		"invalid":         {input: "owner/project/stack/wat", expected: false},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			actual := isFullyQualifiedStackName(tc.input)
			assert.Equal(t, tc.expected, actual)
		})
	}
}
