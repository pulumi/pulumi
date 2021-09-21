// Copyright 2016-2021, Pulumi Corporation.
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

package plugin

import (
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	resource_testing "github.com/pulumi/pulumi/sdk/v3/go/common/resource/testing"
)

var marshalOpts = MarshalOptions{
	KeepUnknowns:     true,
	KeepSecrets:      true,
	KeepResources:    true,
	KeepOutputValues: true,
}

func TestOutputValueTurnaround(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		v := resource_testing.OutputPropertyGenerator(1).Draw(t, "output").(resource.PropertyValue)
		pb, err := MarshalPropertyValue("", v, marshalOpts)
		require.NoError(t, err)

		v2, err := UnmarshalPropertyValue("", pb, marshalOpts)
		require.NoError(t, err)
		require.NotNil(t, v2)

		resource_testing.AssertEqualPropertyValues(t, v, *v2)
	})
}

func TestSerializePropertyValues(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		v := resource_testing.PropertyValueGenerator(6).Draw(t, "property value").(resource.PropertyValue)
		_, err := MarshalPropertyValue("", v, marshalOpts)
		require.NoError(t, err)
	})
}
