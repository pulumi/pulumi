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

package importer

import (
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/utils/rapidresource"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/utils/rapidschema"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

// TestRapidResourceValuesAreStructurallyTyped draws a schema, then for each
// resource in the schema draws inputs/outputs/state and asserts that the
// resulting property maps are structurally typed against the resource's
// declared property lists.
func TestRapidResourceValuesAreStructurallyTyped(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		pkg := rapidschema.Package().Draw(t, "pkg")
		for _, r := range pkg.Resources {
			inputs := rapidresource.ResourceInputs(r).Draw(t, "inputs:"+r.Token)
			assertMapStructurallyTyped(t, inputs, r.InputProperties)

			outputs := rapidresource.ResourceProperties(r).Draw(t, "outputs:"+r.Token)
			assertMapStructurallyTyped(t, outputs, r.Properties)

			state := rapidresource.ResourceState(r).Draw(t, "state:"+r.Token)
			if r.StateInputs == nil {
				require.Nil(t, state, "state should be nil when StateInputs is nil")
			} else {
				require.NotNil(t, state)
				assertMapStructurallyTyped(t, *state, r.StateInputs.Properties)
			}
		}
	})
}

func assertMapStructurallyTyped(t require.TestingT, m property.Map, props []*schema.Property) {
	require.Truef(t, rapidresource.MapStructurallyTyped(m, props),
		"map %v does not match property list", m)
}
