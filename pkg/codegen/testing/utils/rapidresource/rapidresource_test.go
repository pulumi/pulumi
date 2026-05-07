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

package rapidresource

import (
	"testing"

	"pgregory.net/rapid"

	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/utils/rapidschema"
	"github.com/stretchr/testify/require"
)

// TestResourceGeneratorsDraw smokes the three exported generators against
// schemas drawn by rapidschema.Package(). It just verifies they don't panic
// and that ResourceState's nilness tracks r.StateInputs. Full structural
// typing is asserted by TestRapidResourceValuesAreStructurallyTyped in
// pkg/importer (which has access to valueStructurallyTypedAs).
func TestResourceGeneratorsDraw(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		pkg := rapidschema.Package().Draw(t, "pkg")
		for _, r := range pkg.Resources {
			_ = ResourceInputs(r).Draw(t, "inputs:"+r.Token)
			_ = ResourceProperties(r).Draw(t, "outputs:"+r.Token)
			state := ResourceState(r).Draw(t, "state:"+r.Token)
			if r.StateInputs == nil {
				require.Nil(t, state)
			} else {
				require.NotNil(t, state)
			}
		}
	})
}
