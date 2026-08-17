// Copyright 2016, Pulumi Corporation.
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

package codegen

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolvingPackageReferences(t *testing.T) {
	t.Parallel()

	testdataPath := filepath.Join("testing", "test", "testdata")
	loader := schema.NewPluginLoader(utils.NewContext(testdataPath))
	var pkgSpec schema.PackageSpec
	require.NoError(t, json.Unmarshal(utils.ReadSchema(t, "remoteref", "1.0.0"), &pkgSpec))
	pkg, diags, err := schema.BindSpec(pkgSpec, loader, schema.ValidationOptions{
		AllowDanglingReferences: true,
	})
	require.NotNil(t, pkg)
	require.NoError(t, err)
	require.Empty(t, diags)
	// ensure that package references return goalias because remoteref depends on goalias
	references := PackageReferences(pkg)
	require.Len(t, references, 1)
	assert.Equal(t, "goalias", references[0].Name())
}
