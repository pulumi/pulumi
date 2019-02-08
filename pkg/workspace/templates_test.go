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

package workspace

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetValidDefaultProjectName(t *testing.T) {
	// Valid names remain the same.
	for _, name := range getValidProjectNamePrefixes() {
		assert.Equal(t, name, getValidProjectName(name))
	}
	assert.Equal(t, "foo", getValidProjectName("foo"))
	assert.Equal(t, "foo1", getValidProjectName("foo1"))
	assert.Equal(t, "foo-", getValidProjectName("foo-"))
	assert.Equal(t, "foo-bar", getValidProjectName("foo-bar"))
	assert.Equal(t, "foo_", getValidProjectName("foo_"))
	assert.Equal(t, "foo_bar", getValidProjectName("foo_bar"))
	assert.Equal(t, "foo.", getValidProjectName("foo."))
	assert.Equal(t, "foo.bar", getValidProjectName("foo.bar"))

	// Invalid characters are left off.
	assert.Equal(t, "foo", getValidProjectName("!foo"))
	assert.Equal(t, "foo", getValidProjectName("@foo"))
	assert.Equal(t, "foo", getValidProjectName("#foo"))
	assert.Equal(t, "foo", getValidProjectName("$foo"))
	assert.Equal(t, "foo", getValidProjectName("%foo"))
	assert.Equal(t, "foo", getValidProjectName("^foo"))
	assert.Equal(t, "foo", getValidProjectName("&foo"))
	assert.Equal(t, "foo", getValidProjectName("*foo"))
	assert.Equal(t, "foo", getValidProjectName("(foo"))
	assert.Equal(t, "foo", getValidProjectName(")foo"))

	// Invalid names are replaced with a fallback name.
	assert.Equal(t, "project", getValidProjectName("!"))
	assert.Equal(t, "project", getValidProjectName("@"))
	assert.Equal(t, "project", getValidProjectName("#"))
	assert.Equal(t, "project", getValidProjectName("$"))
	assert.Equal(t, "project", getValidProjectName("%"))
	assert.Equal(t, "project", getValidProjectName("^"))
	assert.Equal(t, "project", getValidProjectName("&"))
	assert.Equal(t, "project", getValidProjectName("*"))
	assert.Equal(t, "project", getValidProjectName("("))
	assert.Equal(t, "project", getValidProjectName(")"))
	assert.Equal(t, "project", getValidProjectName("!@#$%^&*()"))
}

func getValidProjectNamePrefixes() []string {
	var results []string
	for ch := 'A'; ch <= 'Z'; ch++ {
		results = append(results, string(ch))
	}
	for ch := 'a'; ch <= 'z'; ch++ {
		results = append(results, string(ch))
	}
	results = append(results, "_")
	results = append(results, ".")
	return results
}
