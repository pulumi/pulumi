// Copyright 2016-2023, Pulumi Corporation.
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

package deploy

import (
	"testing"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
)

func TestManifest(t *testing.T) {
	t.Parallel()
	t.Run("Serialize", func(t *testing.T) {
		t.Parallel()
		ver := semver.MustParse("1.0.0")
		m := Manifest{
			Plugins: []workspace.PluginInfo{
				{
					Name:    "plug-1",
					Path:    "/foo",
					Kind:    apitype.LanguagePlugin,
					Version: &ver,
				},
				{
					Name:    "plug-2",
					Path:    "/bar",
					Kind:    apitype.ResourcePlugin,
					Version: &ver,
				},
			},
		}
		assert.Equal(t, apitype.ManifestV1{
			Plugins: []apitype.PluginInfoV1{
				{Name: "plug-1", Path: "/foo", Type: "language", Version: "1.0.0"},
				{Name: "plug-2", Path: "/bar", Type: "resource", Version: "1.0.0"},
			},
		}, m.Serialize())
	})
	t.Run("DeserializeManifest", func(t *testing.T) {
		t.Parallel()
		t.Run("bad version", func(t *testing.T) {
			_, err := DeserializeManifest(apitype.ManifestV1{
				Plugins: []apitype.PluginInfoV1{
					{
						Version: "?????????????????",
					},
				},
			})
			assert.ErrorContains(t, err, "Invalid character(s) found in major number")
		})
		t.Run("ok", func(t *testing.T) {
			t.Run("has plugins", func(t *testing.T) {
				m, err := DeserializeManifest(apitype.ManifestV1{
					Plugins: []apitype.PluginInfoV1{
						{
							Name:    "plugin",
							Version: "1.0.0",
						},
						{
							Name: "plugin-no-version",
						},
					},
				})
				assert.NoError(t, err)
				assert.Equal(t, apitype.ManifestV1{
					Plugins: []apitype.PluginInfoV1{
						{Name: "plugin", Path: "", Type: "", Version: "1.0.0"},
						{Name: "plugin-no-version", Path: "", Type: "", Version: ""},
					},
				}, m.Serialize())
			})
			t.Run("no plugins", func(t *testing.T) {
				m, err := DeserializeManifest(apitype.ManifestV1{
					Plugins: []apitype.PluginInfoV1{},
				})
				assert.NoError(t, err)
				assert.Equal(t, apitype.ManifestV1{
					Plugins: nil,
				}, m.Serialize())
			})
		})
	})
}
