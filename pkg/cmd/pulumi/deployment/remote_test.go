// Copyright 2016-2024, Pulumi Corporation.
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

package deployment

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseEnv(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		input     string
		name      string
		value     string
		errString string
	}{
		"name val":            {input: "FOO=bar", name: "FOO", value: "bar"},
		"name empty val":      {input: "FOO=", name: "FOO"},
		"name val extra seps": {input: "FOO=bar=baz", name: "FOO", value: "bar=baz"},
		"empty":               {input: "", errString: `expected value of the form "NAME=value": missing "=" in ""`},
		"no sep":              {input: "foo", errString: `expected value of the form "NAME=value": missing "=" in "foo"`},
		"empty name val":      {input: "=", errString: `expected non-empty environment name in "="`},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			name, value, err := parseEnv(tc.input)
			if tc.errString != "" {
				assert.EqualError(t, err, tc.errString)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tc.name, name)
			assert.Equal(t, tc.value, value)
		})
	}
}
