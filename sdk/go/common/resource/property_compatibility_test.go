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
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/urn"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
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

func testRoundTripThroughGRPC(t require.TestingT, v property.Value) {
	rm := resource.ToResourcePropertyValue(v)

	marshalOpts := plugin.MarshalOptions{
		KeepUnknowns:     true,
		KeepSecrets:      true,
		KeepOutputValues: true,
	}

	mm, err := plugin.MarshalPropertyValue("", rm, marshalOpts)
	require.NoError(t, err)

	nrm, err := plugin.UnmarshalPropertyValue("", mm, marshalOpts)
	require.NoError(t, err)

	// Inexplicably, some [resource.PropertyValue]s do not survive round-tripping. We
	// see the computed empty string in rm turn into a computed nil value in
	// nrm. These are semantically equivalent (since they are behind a
	// [resource.Computed]), but they should round trip correctly.
	//
	// You can check if this comment still applies by adding an:
	//
	//	assert.NotEqual(t, rm, nrm)
	//
	// If that check fails, this comment no longer applies and can be removed.

	nm := resource.FromResourcePropertyValue(*nrm)

	assert.Equal(t, v, nm, "Assert that m survived a full round trip through gRPC's representation")
}

func TestConversionThroughGRPCRapid(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		source := pTest.Value(10).Draw(t, "round-trip value")
		testRoundTripThroughGRPC(t, source)
	})
}

func TestConversionThroughGRPC(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value property.Value
	}{
		{"known", property.New("v1")},
		{"unknown-output", property.New(property.Computed).WithSecret(true)},
		{"known-output", property.New(1.2).WithDependencies([]urn.URN{"urn1", "urn2"})},
		{"unknown", property.New(property.Computed)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			testRoundTripThroughGRPC(t, tt.value)
		})
	}
}
