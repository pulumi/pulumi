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

package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestMarshalDefinitionKeepsDocumentComments(t *testing.T) {
	t.Parallel()

	const def = "# leading comment\n\nvalues:\n  x: 1\n\n# trailing comment\n"

	var doc yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(def), &doc))

	out, err := marshalDefinition(&doc)
	require.NoError(t, err)
	assert.Equal(t, def, string(out))
}
