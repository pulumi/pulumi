// Copyright 2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ui

import (
	"testing"
)

func Test_MakeJSONString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     interface{}
		multiline bool
		expected  string
	}{
		{
			name:      "simple-string/multiline",
			input:     map[string]interface{}{"my_password": "password"},
			multiline: true,
			expected: `{
  "my_password": "password"
}
`,
		},
		{
			name:     "simple-string",
			input:    map[string]interface{}{"my_password": "password"},
			expected: `{"my_password":"password"}`,
		},
		{
			name:      "special-char-string/multiline",
			input:     map[string]interface{}{"special_password": "pass&word"},
			multiline: true,
			expected: `{
  "special_password": "pass&word"
}
`,
		},
		{
			name:     "special-char-string",
			input:    map[string]interface{}{"special_password": "pass&word"},
			expected: `{"special_password":"pass&word"}`,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := MakeJSONString(tt.input, tt.multiline)
			if err != nil {
				t.Errorf("MakeJSONString() error = %v", err)
				return
			}
			if got != tt.expected {
				t.Errorf("MakeJSONString() got = %v, expected %v", got, tt.expected)
			}
		})
	}
}
