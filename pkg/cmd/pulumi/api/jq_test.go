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

package api

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestApplyJQ covers ApplyJQ's output conventions and its error handling.
// String results emit raw (no quotes) so pipelines like `| xargs` work;
// everything else emits compact JSON, one line per result.
func TestApplyJQ(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		data  string
		expr  string
		want  string
		isErr bool
	}{
		{"identity_object", `{"a":1}`, `.`, "{\"a\":1}\n", false},
		{"string_result_raw", `{"name":"acme"}`, `.name`, "acme\n", false},
		{"number_result_json", `{"n":42}`, `.n`, "42\n", false},
		{"bool_result_json", `{"ok":true}`, `.ok`, "true\n", false},
		{"null_result", `{"n":null}`, `.n`, "null\n", false},
		{"array_result_json", `{"xs":[1,2]}`, `.xs`, "[1,2]\n", false},
		{"iterator_one_line_each", `[{"a":1},{"a":2},{"a":3}]`, `.[].a`, "1\n2\n3\n", false},
		{"iterator_strings_raw", `["a","b"]`, `.[]`, "a\nb\n", false},
		{"parse_error", `{}`, `.[`, "", true},
		{"eval_error", `{"a":1}`, `.a | .b`, "", true},
		{"invalid_json_input", `not json`, `.`, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			err := ApplyJQ(&buf, []byte(tc.data), tc.expr)
			if tc.isErr {
				require.Error(t, err)
				var apiErr *APIError
				require.True(t, errors.As(err, &apiErr))
				assert.Equal(t, ErrInvalidFlags, apiErr.Envelope.Error.Code)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, buf.String())
		})
	}
}
