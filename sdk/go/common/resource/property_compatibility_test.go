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

package resource_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	pTest "github.com/pulumi/pulumi/sdk/v3/go/property/testing"
)

// Test that we can round trip through
// github.com/pulumi/pulumi/sdk/v3/go/common/resource.PropertyValue without loosing
// information.
//
// Note: This is not possible in the other direction, since
// github.com/pulumi/pulumi/sdk/v3/go/common/resource.PropertyValue can represent invalid
// states, such as Computed(Computed(Null)).
func TestRoundTripConvert(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		source := pTest.Value(10).Draw(t, "round-trip value")
		propertyValue := resource.ToResourcePropertyValue(source)
		roundTripped := resource.FromResourcePropertyValue(propertyValue)

		assert.True(t, source.Equals(roundTripped))
	})
}
