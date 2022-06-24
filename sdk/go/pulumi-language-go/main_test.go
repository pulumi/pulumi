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

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
)

func TestGetPlugin(t *testing.T) {
	t.Parallel()

	cases := []struct {
		Name      string
		Mod       *modInfo
		Expected  *pulumirpc.PluginDependency
		ShouldErr bool
		JSON      *plugin.PulumiPluginJSON
		JSONPath  string
	}{
		{
			Name: "valid-pulumi-mod",
			Mod: &modInfo{
				Path:    "github.com/pulumi/pulumi-aws/sdk",
				Version: "v1.29.0",
			},
			Expected: &pulumirpc.PluginDependency{
				Name:    "aws",
				Version: "v1.29.0",
			},
		},
		{
			Name: "pulumi-pseduo-version-plugin",
			Mod: &modInfo{
				Path:    "github.com/pulumi/pulumi-aws/sdk",
				Version: "v1.29.1-0.20200403140640-efb5e2a48a86",
			},
			Expected: &pulumirpc.PluginDependency{
				Name:    "aws",
				Version: "v1.29.0",
			},
		},
		{
			Name: "non-pulumi-mod",
			Mod: &modInfo{
				Path:    "github.com/moolumi/pulumi-aws/sdk",
				Version: "v1.29.0",
			},
			ShouldErr: true,
		},
		{
			Name: "invalid-version-module",
			Mod: &modInfo{
				Path:    "github.com/pulumi/pulumi-aws/sdk",
				Version: "42-42-42",
			},
			ShouldErr: true,
		},
		{
			Name: "pulumi-pulumi-mod",
			Mod: &modInfo{
				Path:    "github.com/pulumi/pulumi/sdk",
				Version: "v1.14.0",
			},
			ShouldErr: true,
		},
		{
			Name: "beta-pulumi-module",
			Mod: &modInfo{
				Path:    "github.com/pulumi/pulumi-aws/sdk",
				Version: "v2.0.0-beta.1",
			},
			Expected: &pulumirpc.PluginDependency{
				Name:    "aws",
				Version: "v2.0.0-beta.1",
			},
		},
		{
			Name: "non-zero-patch-module", Mod: &modInfo{
				Path:    "github.com/pulumi/pulumi-kubernetes/sdk",
				Version: "v1.5.8",
			},
			Expected: &pulumirpc.PluginDependency{
				Name:    "kubernetes",
				Version: "v1.5.8",
			},
		},
		{
			Name: "pulumiplugin",
			Mod: &modInfo{
				Path:    "github.com/me/myself/i",
				Version: "invalid-Version",
			},
			Expected: &pulumirpc.PluginDependency{
				Name:    "thing1",
				Version: "v1.2.3",
				Server:  "myserver.com",
			},
			JSON: &plugin.PulumiPluginJSON{
				Resource: true,
				Name:     "thing1",
				Version:  "v1.2.3",
				Server:   "myserver.com",
			},
		},
		{
			Name:      "non-resource",
			Mod:       &modInfo{},
			ShouldErr: true,
			JSON: &plugin.PulumiPluginJSON{
				Resource: false,
			},
		},
		{
			Name: "missing-pulumiplugin",
			Mod: &modInfo{
				Dir: "/not/real",
			},
			ShouldErr: true,
			JSON: &plugin.PulumiPluginJSON{
				Name:    "thing2",
				Version: "v1.2.3",
			},
		},
		{
			Name: "pulumiplugin-go-lookup",
			Mod: &modInfo{
				Path:    "github.com/me/myself",
				Version: "v1.2.3",
			},
			JSON: &plugin.PulumiPluginJSON{
				Name:     "name",
				Resource: true,
			},
			JSONPath: "go",
			Expected: &pulumirpc.PluginDependency{
				Name:    "name",
				Version: "v1.2.3",
			},
		},
		{
			Name: "pulumiplugin-go-name-lookup",
			Mod: &modInfo{
				Path:    "github.com/me/myself",
				Version: "v1.2.3",
			},
			JSON: &plugin.PulumiPluginJSON{
				Name:     "name",
				Resource: true,
			},
			JSONPath: filepath.Join("go", "name"),
			Expected: &pulumirpc.PluginDependency{
				Name:    "name",
				Version: "v1.2.3",
			},
		},
		{
			Name: "pulumiplugin-nested-too-deep",
			Mod: &modInfo{
				Path:    "path.com/here",
				Version: "v0.0",
			},
			JSONPath: filepath.Join("go", "valid", "invalid"),
			JSON: &plugin.PulumiPluginJSON{
				Name:     "name",
				Resource: true,
			},
			ShouldErr: true,
		},
		{
			Name: "nested-wrong-folder",
			Mod: &modInfo{
				Path:    "path.com/here",
				Version: "v0.0",
			},
			JSONPath: filepath.Join("invalid", "valid"),
			JSON: &plugin.PulumiPluginJSON{
				Name:     "name",
				Resource: true,
			},
			ShouldErr: true,
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()

			cwd := t.TempDir()
			if c.JSON != nil {
				if c.Mod.Dir == "" {
					c.Mod.Dir = cwd
				}
				path := filepath.Join(cwd, c.JSONPath)
				err := os.MkdirAll(path, 0700)
				assert.NoErrorf(t, err, "Failed to setup test folder", path)
				bytes, err := c.JSON.JSON()
				assert.NoError(t, err, "Failed to setup test pulumi-plugin.json")
				err = os.WriteFile(filepath.Join(path, "pulumi-plugin.json"), bytes, 0600)
				assert.NoError(t, err, "Failed to write pulumi-plugin.json")
			}
			actual, err := c.Mod.getPlugin()
			if c.ShouldErr {
				assert.Error(t, err)
			} else {
				// Kind must be resource. We can thus exclude it from the test.
				if c.Expected.Kind == "" {
					c.Expected.Kind = "resource"
				}
				assert.NoError(t, err)
				assert.Equal(t, c.Expected, actual)
			}
		})
	}
}
