// Copyright 2016-2018, Pulumi Corporation.
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

package cmd

import (
	"fmt"
	"testing"
)

func TestParseTagFilter(t *testing.T) {
	tests := []struct {
		Filter    string
		WantName  string
		WantValue string
		WantError bool
	}{
		// Just tag name
		{Filter: "just tag name", WantName: "just tag name"},
		{Filter: "tag-name123", WantName: "tag-name123"},

		{Filter: "tag-name123:tag value", WantName: "tag-name123", WantValue: "tag value"},
		{Filter: "tag-name123:tag value:with-colon", WantName: "tag-name123", WantValue: "tag value:with-colon"},

		// Error cases
		{Filter: ":", WantError: true},
		{Filter: ":no tag name", WantError: true},
		{Filter: "no tag value:", WantError: true},
	}

	for _, test := range tests {
		name, value, err := parseTagFilter(test.Filter)

		var validationErr string
		if test.WantName != name {
			validationErr = fmt.Sprintf("Tag name not parsed as expected (%v vs. %v)", test.WantName, name)
		} else if test.WantValue != value {
			validationErr = fmt.Sprintf("Tag value not parsed as expected (%v vs. %v)", test.WantValue, value)
		} else if test.WantError != (err != nil) {
			validationErr = fmt.Sprintf("wanted error %v but got error %v", test.WantError, err)
		}

		if validationErr != "" {
			t.Errorf("parseTagFilter(%q) didn't return expected input: %v", test.Filter, validationErr)
		}
	}
}
