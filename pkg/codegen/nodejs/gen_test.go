// Copyright 2016-2020, Pulumi Corporation.
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

package nodejs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var safeEnumNameTests = []struct {
	input    string
	expected string
}{
	{"Microsoft-Windows-Shell-Setup", "MicrosoftWindowsShellSetup"},
	{"readonly", "readonly"},
	{"Microsoft.Batch", "MicrosoftBatch"},
	{"TCP", "TCP"},
	{"SystemAssigned, UserAssigned", "SystemAssignedUserAssigned"},
	{"SystemAssigned,UserAssigned", "SystemAssignedUserAssigned"},
	{"storage_optimized_l1", "storage_optimized_l1"},
}

func TestSafeEnumName(t *testing.T) {
	for _, tt := range safeEnumNameTests {
		t.Run(tt.input, func(t *testing.T) {
			result := safeEnumName(tt.input)
			if result != tt.expected {
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
