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

package neo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClassifyApprovalReply(t *testing.T) {
	t.Parallel()

	tests := []struct {
		text             string
		wantApproved     bool
		wantInstructions string
	}{
		// Affirmatives → approved, no instructions.
		{"y", true, ""},
		{"yes", true, ""},
		{"YES", true, ""},
		{"ok", true, ""},
		{"okay", true, ""},
		{"approve", true, ""},
		{"approved", true, ""},
		{"confirm", true, ""},
		{"go ahead", true, ""},
		{"  Go   Ahead ", true, ""}, // whitespace collapses
		{"lgtm", true, ""},
		{"ship it", true, ""},
		// CLI-only additions.
		{"go on", true, ""},
		{"all right", true, ""},
		{"alright", true, ""},
		// Non-affirmatives → denial, text forwarded as instructions.
		{"not on prod", false, "not on prod"},
		{"yes but only on dev", false, "yes but only on dev"}, // compound is not a bare affirmative
		{"no", false, "no"},                                   // rejection phrases are not special-cased
		{"cancel", false, "cancel"},
		{"  trim me  ", false, "trim me"},
		{"", false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			t.Parallel()
			approved, instructions := classifyApprovalReply(tt.text)
			assert.Equal(t, tt.wantApproved, approved)
			assert.Equal(t, tt.wantInstructions, instructions)
		})
	}
}
