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

func TestDeterminePluginDependency(t *testing.T) {
	cases := []struct {
		Name           string
		PackageName    string
		PackageVersion string
		VersionTxt     string
		PulumiPlugin   *plugin.PulumiPluginJSON
		ExpectError    bool
		Expected       *pulumirpc.PluginDependency
	}{}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			cwd := t.TempDir()
			if c.VersionTxt != "" {
				err := os.WriteFile(filepath.Join(cwd, "version.txt"), []byte(c.VersionTxt), 0600)
				assert.NoError(t, err)
			}
			if c.PulumiPlugin != nil {
				bytes, err := c.PulumiPlugin.JSON()
				assert.NoError(t, err)
				err = os.WriteFile(filepath.Join(cwd, "pulumiplugin.json"), bytes, 0600)
				assert.NoError(t, err)
			}
			actual, err := DeterminePluginDependency(cwd, c.PackageName, c.PackageVersion)
			if c.ExpectError {
				assert.Error(t, err)
			} else {
				if c.Expected.Kind == "" {
					c.Expected.Kind = "resource"
				}
				assert.NoError(t, err)
				assert.Equal(t, c.Expected, actual)
			}
		})
	}
}
