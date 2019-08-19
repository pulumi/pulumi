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

	"github.com/stretchr/testify/assert"
)

func TestParseTagFilter(t *testing.T) {
	tests := []struct {
		Filter    string
		WantName  string
		WantValue string
		WantError bool
	}{
		// Just tag name
		{Filter: ":", WantName: ":"},
		{Filter: "just tag name", WantName: "just tag name"},
		{Filter: "tag-name123", WantName: "tag-name123"},

		{Filter: "tag-name123=tag value", WantName: "tag-name123", WantValue: "tag value"},
		{Filter: "tag-name123=tag value:with-colon", WantName: "tag-name123", WantValue: "tag value:with-colon"},
		{Filter: "tag-name123=tag value=with-equal", WantName: "tag-name123", WantValue: "tag value=with-equal"},

		// Error cases
		{Filter: "=", WantError: true},
		{Filter: "=no tag name", WantError: true},
		{Filter: "no tag value=", WantError: true},
	}

	for _, test := range tests {
		name, value, err := parseTagFilter(test.Filter)
		assert.Equal(t, test.WantName, name)
		assert.Equal(t, test.WantValue, value)
		assert.Equal(t, test.WantError, (err != nil), fmt.Sprintf("Got error: %v", err))
	}
}
