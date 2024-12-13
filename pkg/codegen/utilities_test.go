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

package codegen

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func readSchemaFile(file string) (pkgSpec schema.PackageSpec) {
	// Read in, decode, and import the schema.
	schemaBytes, err := os.ReadFile(filepath.Join("testing", "test", "testdata", file))
	if err != nil {
		panic(err)
	}

	if strings.HasSuffix(file, ".json") {
		if err = json.Unmarshal(schemaBytes, &pkgSpec); err != nil {
			panic(err)
		}
	} else if strings.HasSuffix(file, ".yaml") || strings.HasSuffix(file, ".yml") {
		if err = yaml.Unmarshal(schemaBytes, &pkgSpec); err != nil {
			panic(err)
		}
	} else {
		panic("unknown schema file extension while parsing " + file)
	}

	return pkgSpec
}

func TestResolvingPackageReferences(t *testing.T) {
	t.Parallel()

	testdataPath := filepath.Join("testing", "test", "testdata")
	loader := schema.NewPluginLoader(utils.NewHost(testdataPath))
	pkgSpec := readSchemaFile("awsx-1.0.0-beta.5.json")
	pkg, diags, err := schema.BindSpec(pkgSpec, loader)
	require.NotNil(t, pkg)
	require.NoError(t, err)
	require.Empty(t, diags)
	// ensure that package references return aws because awsx depends on aws
	references := PackageReferences(pkg)
	require.Equal(t, 1, len(references))
	assert.Equal(t, "aws", references[0])
}

func TestStringSetContains(t *testing.T) {
	t.Parallel()

	set123 := NewStringSet("1", "2", "3")
	set12 := NewStringSet("1", "2")
	set14 := NewStringSet("1", "4")
	setEmpty := NewStringSet()

	assert.True(t, set123.Contains(set123))
	assert.True(t, set123.Contains(set12))
	assert.False(t, set12.Contains(set123))
	assert.False(t, set123.Contains(set14))
	assert.True(t, set123.Contains(setEmpty))
}

func TestStringSetSubtract(t *testing.T) {
	t.Parallel()

	set1234 := NewStringSet("1", "2", "3", "4")
	set125 := NewStringSet("1", "2", "5")
	set34 := NewStringSet("3", "4")
	setEmpty := NewStringSet()

	assert.Equal(t, set34, set1234.Subtract(set125))
	assert.Equal(t, setEmpty, set1234.Subtract(set1234))
	assert.Equal(t, set1234, set1234.Subtract(setEmpty))
}

func TestSimplifyInputUnion(t *testing.T) {
	t.Parallel()

	u1 := &schema.UnionType{
		ElementTypes: []schema.Type{
			&schema.InputType{ElementType: schema.StringType},
			schema.NumberType,
		},
	}

	u2 := SimplifyInputUnion(u1)
	assert.Equal(t, &schema.UnionType{
		ElementTypes: []schema.Type{
			schema.StringType,
			schema.NumberType,
		},
	}, u2)
}
