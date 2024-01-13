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

package test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"

	"github.com/pulumi/pulumi/sdk/v3/go/property"
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
		source := Value(10).Draw(t, "round-trip value")
		propertyValue := property.ToResourcePropertyValue(source)
		roundTripped := property.FromResourcePropertyValue(propertyValue)

		assert.True(t, source.Equals(roundTripped))
	})
}
