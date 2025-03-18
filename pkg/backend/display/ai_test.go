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

package display

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTruncateWithMiddleOut(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxChars int
		want     string
	}{
		{
			name:     "under limit",
			input:    "short content",
			maxChars: 100,
			want:     "short content",
		},
		{
			name:     "exact limit",
			input:    "12345",
			maxChars: 5,
			want:     "12345",
		},
		{
			name:     "needs truncation",
			input:    "start middle1 middle2 end",
			maxChars: 22,
			want:     "st... (truncated) ...d",
		},
		{
			name:     "single long line",
			input:    "abcdefghijklmnopqrstuvwxyz",
			maxChars: 25,
			want:     "abc... (truncated) ...xyz",
		},
		{
			name:     "empty input",
			input:    "",
			maxChars: 10,
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TruncateWithMiddleOut(tt.input, tt.maxChars)
			assert.Equal(t, tt.want, got)

			// Verify the result is under the character limit
			if len(tt.input) > 0 {
				totalChars := len(got)
				assert.LessOrEqual(t, totalChars, tt.maxChars, "result exceeds character limit")
			}
		})
	}
}
