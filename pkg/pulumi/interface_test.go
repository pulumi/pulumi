// Copyright 2016-2023, Pulumi Corporation.
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

package pulumi

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Ensure that every language test starts with a standard prefix.
func TestTestNames(t *testing.T) {
	t.Parallel()

	for name := range languageTests {
		isl1 := strings.HasPrefix(name, "l1-")
		isl2 := strings.HasPrefix(name, "l2-")
		isl3 := strings.HasPrefix(name, "l3-")
		assert.True(t, isl1 || isl2 || isl3, "test name %s must start with l1-, l2-, or l3-", name)
	}
}

// Ensure l1 tests don't use providers.
func TestL1NoProviders(t *testing.T) {
	t.Parallel()

	for name, test := range languageTests {
		if strings.HasPrefix(name, "l1-") {
			assert.Empty(t, test.providers, "test name %s must not use providers", name)
		}
	}
}
