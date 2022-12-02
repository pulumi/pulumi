// Copyright 2016-2022, Pulumi Corporation.
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

package main

import (
	"fmt"
	"testing"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
)

type testLoader struct {
	packages map[string]map[string]*schema.Package
}

func (loader *testLoader) LoadPackage(pkg string, version *semver.Version) (*schema.Package, error) {
	versionStr := ""
	if version != nil {
		versionStr = version.String()
	}

	pack, ok := loader.packages[pkg][versionStr]
	if !ok {
		return nil, fmt.Errorf("could not load package %s %s", pkg, versionStr)
	}

	return pack, nil
}

// NewTestLoader creates a loader supporting multiple package versions in tests. This
// enables running tests offline.
func NewTestLoader(packages map[string]map[string]*schema.Package) schema.Loader {
	return &testLoader{
		packages: packages,
	}
}

func TestGenerateLanguageDefinitions_Simple(t *testing.T) {
	t.Parallel()

	spec := schema.PackageSpec{
		Name: "testprovider",
		Resources: map[string]schema.ResourceSpec{
			"testprovider:index:TestResource": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Properties: map[string]schema.PropertySpec{
						"out": {TypeSpec: schema.TypeSpec{Type: "string"}},
					},
					Required: []string{
						"out",
					},
				},
				InputProperties: map[string]schema.PropertySpec{
					"in": {TypeSpec: schema.TypeSpec{Type: "string"}},
				},
			},
		},
		Provider: schema.ResourceSpec{},
	}
	testSchema, err := schema.ImportSpec(spec, nil)
	assert.NoError(t, err)

	loader := NewTestLoader(map[string]map[string]*schema.Package{
		"testprovider": {
			"": testSchema,
		},
	})

	states := []*resource.State{
		{
			Type:   "testprovider:index:TestResource",
			URN:    resource.NewURN("teststack", "testproject", "", "testprovider:index:TestResource", "testResource"),
			Inputs: resource.NewPropertyMapFromMap(map[string]interface{}{"in": "input"}),
		},
	}

	program, err := generateLanguageDefinitions(loader, states, nil)
	assert.NoError(t, err)

	source := program.Source()
	expectedSource := map[string]string{
		"anonymous.pp": "resource testResource \"testprovider:index:TestResource\" {\n    in = \"input\"\n\n}\n",
	}
	assert.Equal(t, expectedSource, source)
}

func TestGenerateLanguageDefinitions_EscapeTemplates(t *testing.T) {
	t.Parallel()
	t.Skip("PCL binding doesn't handle literal occurrences of ${} in resource inputs")

	// Regression test for https://github.com/pulumi/pulumi/issues/11507

	spec := schema.PackageSpec{
		Name: "testprovider",
		Resources: map[string]schema.ResourceSpec{
			"testprovider:index:TestResource": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Properties: map[string]schema.PropertySpec{
						"out": {TypeSpec: schema.TypeSpec{Type: "string"}},
					},
					Required: []string{
						"out",
					},
				},
				InputProperties: map[string]schema.PropertySpec{
					"in": {TypeSpec: schema.TypeSpec{Type: "string"}},
				},
			},
		},
		Provider: schema.ResourceSpec{},
	}
	testSchema, err := schema.ImportSpec(spec, nil)
	assert.NoError(t, err)

	loader := NewTestLoader(map[string]map[string]*schema.Package{
		"testprovider": {
			"": testSchema,
		},
	})

	states := []*resource.State{
		{
			Type:   "testprovider:index:TestResource",
			URN:    resource.NewURN("teststack", "testproject", "", "testprovider:index:TestResource", "testResource"),
			Inputs: resource.NewPropertyMapFromMap(map[string]interface{}{"in": "${not_really_a_template}"}),
		},
	}

	program, err := generateLanguageDefinitions(loader, states, nil)
	assert.NoError(t, err)

	source := program.Source()
	expectedSource := map[string]string{
		"anonymous.pp": "resource testResource \"testprovider:index:TestResource\" {\n    in = \"$${not_really_a_template}\"\n\n}\n",
	}
	assert.Equal(t, expectedSource, source)
}
