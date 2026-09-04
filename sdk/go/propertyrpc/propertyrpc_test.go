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

package propertyrpc_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	pTest "github.com/pulumi/pulumi/sdk/v3/go/property/testing"
	"github.com/pulumi/pulumi/sdk/v3/go/propertyrpc"
)

// TestMarshalThroughGRPC checks that a [property.Map] survives a round trip through [propertyrpc.Marshal] and
// [plugin.UnmarshalProperties].
func TestMarshalThroughGRPC(t *testing.T) {
	t.Parallel()

	opts := plugin.MarshalOptions{
		KeepUnknowns:     true,
		KeepSecrets:      true,
		KeepResources:    true,
		KeepOutputValues: true,
	}

	m := pTest.Map(10)
	rapid.Check(t, func(t *rapid.T) {
		source := m.Draw(t, "round-trip map")

		unmarshalled, err := plugin.UnmarshalProperties(propertyrpc.Marshal(source), opts)
		require.NoError(t, err)

		assert.Equal(t, source, resource.FromResourcePropertyMap(unmarshalled))
	})
}
