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

package oci

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
)

// RequiredPackagesPath is the well-known path, inside a program or plugin image,
// where the build bakes a manifest of the Pulumi provider packages the program
// requires. A template's build step generates it (from the installed deps' Pulumi
// plugin metadata) after installing dependencies; the OCI host reads it back with
// ReadRequiredPackagesFromImage.
//
// It lives outside the workspace mount (/workspace), which the pod shadows with
// the shared workspace volume at runtime — so the manifest is always readable from
// the image's own layers, not masked by the volume.
const RequiredPackagesPath = "/pulumi/required-packages.json"

// RequiredPackagesManifest is the baked, best-effort list of provider packages a
// program declares, as read from RequiredPackagesPath.
//
// Each entry is a plugin.PulumiPluginJSON — the SAME shape Pulumi's per-package
// plugin metadata already uses (a dependency's package.json "pulumi" field for
// Node, a pulumi-plugin.json file for Go/Python/.NET). Conforming to that format
// rather than a bespoke one means: the generator aggregates the metadata verbatim
// (no lossy re-encoding), the host parses it into the existing type for free, and
// parameterization (needed for parameterized providers) rides along.
//
// Note on Server: it is the plugin's binary DOWNLOAD host (a get.pulumi.com-style
// URL), which the OCI model does not use — a provider is resolved to an IMAGE by
// name+version against the configured registry (see providerImageRef), not fetched
// from Server. It is carried here verbatim for fidelity but ignored by the OCI
// resolver; it would only matter if a package's declaration ever pointed at a
// container registry, which today it does not.
type RequiredPackagesManifest struct {
	Plugins []plugin.PulumiPluginJSON `json:"plugins"`
}

// ParseRequiredPackages parses a required-packages manifest. Empty or whitespace-only
// input yields an empty manifest and no error (an absent manifest is normal for a
// best-effort hint); malformed JSON is an error.
func ParseRequiredPackages(data []byte) (RequiredPackagesManifest, error) {
	var m RequiredPackagesManifest
	if len(bytes.TrimSpace(data)) == 0 {
		return m, nil
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return RequiredPackagesManifest{}, fmt.Errorf("parsing required-packages manifest: %w", err)
	}
	return m, nil
}

// ReadRequiredPackagesFromImage reads and parses the required-packages manifest
// baked at RequiredPackagesPath in image. A missing manifest yields an empty result
// and no error: the manifest is a best-effort optimization, and its absence (an
// older or hand-authored image, or a template without the build step) simply means
// there is nothing to pre-fetch.
func ReadRequiredPackagesFromImage(
	ctx context.Context, pod PodManager, image string,
) (RequiredPackagesManifest, error) {
	data, err := pod.ReadImageFile(ctx, image, RequiredPackagesPath)
	if err != nil {
		return RequiredPackagesManifest{}, err
	}
	return ParseRequiredPackages(data)
}

// Summary renders the manifest as "name@version, ..." for logging. Plugins without
// a version render as just the name.
func (m RequiredPackagesManifest) Summary() string {
	parts := make([]string, 0, len(m.Plugins))
	for _, p := range m.Plugins {
		if p.Version != "" {
			parts = append(parts, p.Name+"@"+p.Version)
		} else {
			parts = append(parts, p.Name)
		}
	}
	return strings.Join(parts, ", ")
}
