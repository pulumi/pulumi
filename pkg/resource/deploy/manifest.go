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

package deploy

import (
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// Manifest captures versions for all binaries used to construct this snapshot.
type Manifest struct {
	Time    time.Time              // the time this snapshot was taken.
	Magic   string                 // a magic cookie.
	Version string                 // the pulumi command version.
	Plugins []workspace.PluginInfo // the plugin versions also loaded.
}

// Serialize turns a manifest into a data structure suitable for serialization.
func (m Manifest) Serialize() apitype.ManifestV1 {
	manifest := apitype.ManifestV1{
		Time:    m.Time,
		Magic:   m.Magic,
		Version: m.Version,
	}
	for _, plug := range m.Plugins {
		var version string
		if plug.Version != nil {
			version = plug.Version.String()
		}
		manifest.Plugins = append(manifest.Plugins, apitype.PluginInfoV1{
			Name:    plug.Name,
			Path:    plug.Path,
			Type:    plug.Kind,
			Version: version,
		})
	}
	return manifest
}

// DeserializeManifest deserializes a typed ManifestV1 into a `deploy.Manifest`.
func DeserializeManifest(m apitype.ManifestV1) (*Manifest, error) {
	manifest := Manifest{
		Time:    m.Time,
		Magic:   m.Magic,
		Version: m.Version,
	}
	for _, plug := range m.Plugins {
		var version *semver.Version
		if v := plug.Version; v != "" {
			sv, err := semver.ParseTolerant(v)
			if err != nil {
				return nil, err
			}
			version = &sv
		}
		manifest.Plugins = append(manifest.Plugins, workspace.PluginInfo{
			Name:    plug.Name,
			Kind:    plug.Type,
			Version: version,
		})
	}
	return &manifest, nil
}

// NewMagic creates a magic cookie out of a manifest; this can be used to check for tampering.  This ignores
// any existing magic value already stored on the manifest.
func (m Manifest) NewMagic() string {
	if m.Version == "" {
		return ""
	}
	return fmt.Sprintf("%x", sha256.Sum256([]byte(m.Version)))
}
