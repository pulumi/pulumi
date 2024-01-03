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

package docs

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

func TestGeneratePackageTree(t *testing.T) {
	t.Parallel()

	dctx := newDocGenContext()
	testPackageSpec := newTestPackageSpec()

	schemaPkg, err := schema.ImportSpec(testPackageSpec, nil)
	assert.NoError(t, err, "importing spec")

	dctx.initialize(unitTestTool, schemaPkg)
	pkgTree, err := dctx.generatePackageTree()
	if err != nil {
		t.Errorf("Error generating the package tree for package %s: %v", schemaPkg.Name, err)
	}

	assert.NotEmpty(t, pkgTree, "Package tree was empty")

	t.Run("ValidatePackageTreeTopLevelItems", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, entryTypeModule, pkgTree[0].Type)
		assert.Equal(t, entryTypeModule, pkgTree[1].Type)
		assert.Equal(t, entryTypeResource, pkgTree[2].Type)
		assert.Equal(t, entryTypeResource, pkgTree[3].Type)
		assert.Equal(t, entryTypeFunction, pkgTree[4].Type)
	})

	t.Run("ValidateSortOrder", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "module", pkgTree[0].Name)
		assert.Equal(t, "module2", pkgTree[1].Name)
		assert.Equal(t, "PackageLevelResource", pkgTree[2].Name)
		assert.Equal(t, "Provider", pkgTree[3].Name)
		assert.Equal(t, "getPackageResource", pkgTree[4].Name)
	})

	t.Run("ValidatePackageTreeModuleChildren", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, 2, len(pkgTree[0].Children))
		children := pkgTree[0].Children
		assert.Equal(t, entryTypeResource, children[0].Type)
		assert.Equal(t, entryTypeFunction, children[1].Type)
	})
}

// Original issues: pulumi/pulumi#14821, pulumi/pulumi#14820.
func TestGeneratePackageTreeNested(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name   string
		spec   schema.PackageSpec
		expect string
	}

	testCases := []testCase{
		{
			"14820",
			schema.PackageSpec{
				Name:    "fortios",
				Version: "0.0.1",
				Meta: &schema.MetadataSpec{
					ModuleFormat: "(.*)(?:/[^/]*)",
				},
				Resources: map[string]schema.ResourceSpec{
					"fortios:router/bgp:Bgp":             {},
					"fortios:router/bgp/network:Network": {},
				},
			},
			`
			[
			  {
			    "name": "router",
			    "type": "module",
			    "link": "router/",
			    "children": [
			      {
				"name": "bgp",
				"type": "module",
				"link": "bgp/",
				"children": [
				  {
				    "name": "Network",
				    "type": "resource",
				    "link": "network"
				  }
				]
			      },
			      {
				"name": "Bgp",
				"type": "resource",
				"link": "bgp"
			      }
			    ]
			  },
			  {
			    "name": "Provider",
			    "type": "resource",
			    "link": "provider"
			  }
			]`,
		},
		{
			"14821",
			schema.PackageSpec{
				Name:    "fortios",
				Version: "0.0.1",
				Meta: &schema.MetadataSpec{
					ModuleFormat: "(.*)(?:/[^/]*)",
				},
				Resources: map[string]schema.ResourceSpec{
					"fortios:log/syslogd/v2/filter:Filter":                   {},
					"fortios:log/syslogd/v2/overridefilter:Overridefilter":   {},
					"fortios:log/syslogd/v2/overridesetting:Overridesetting": {},
					"fortios:log/syslogd/v2/setting:Setting":                 {},
				},
			},
			`
			[
			  {
			    "name": "log",
			    "type": "module",
			    "link": "log/",
			    "children": [
			      {
				"name": "syslogd",
				"type": "module",
				"link": "syslogd/",
				"children": [
				  {
				    "name": "v2",
				    "type": "module",
				    "link": "v2/",
				    "children": [
				      {
					"name": "Filter",
					"type": "resource",
					"link": "filter"
				      },
				      {
					"name": "Overridefilter",
					"type": "resource",
					"link": "overridefilter"
				      },
				      {
					"name": "Overridesetting",
					"type": "resource",
					"link": "overridesetting"
				      },
				      {
					"name": "Setting",
					"type": "resource",
					"link": "setting"
				      }
				    ]
				  }
				]
			      }
			    ]
			  },
			  {
			    "name": "Provider",
			    "type": "resource",
			    "link": "provider"
			  }
			]`,
		},
	}

	prep := func(t *testing.T, tc testCase) (*docGenContext, *schema.Package) {
		t.Helper()

		schemaPkg, err := schema.ImportSpec(tc.spec, nil)
		assert.NoError(t, err, "importing spec")

		c := newDocGenContext()
		c.initialize("test", schemaPkg)

		return c, schemaPkg
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			c, _ := prep(t, tc)

			items, err := c.generatePackageTree()
			require.NoError(t, err)

			data, err := json.MarshalIndent(items, "", "  ")
			require.NoError(t, err)

			t.Logf("%s", string(data))

			require.JSONEq(t, tc.expect, string(data))
		})

		t.Run(tc.name+"/generatePackage", func(t *testing.T) {
			t.Parallel()

			c, schemaPkg := prep(t, tc)

			files, err := c.generatePackage("test", schemaPkg)
			require.NoError(t, err)

			for f := range files {
				t.Logf("+ %v", f)
			}
		})
	}
}
