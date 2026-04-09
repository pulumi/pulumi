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

package npm

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
)

// PackageManifestNames are the filenames recognized as Node.js package manifests, in priority order. pnpm allows
// package.yaml as an alternative to package.json (see https://pnpm.io/package_json), and prefers package.json if both
// exist.
var PackageManifestNames = []string{"package.json", "package.yaml"}

// ReadPackageManifest reads the package manifest (package.json or package.yaml) from dir and returns the parsed
// contents along with the path of the file that was read. If both files exist, package.json is preferred. Returns an
// error wrapping os.ErrNotExist if neither file exists in dir.
func ReadPackageManifest(dir string) (map[string]any, string, error) {
	for _, name := range PackageManifestNames {
		path := filepath.Join(dir, name)
		content, err := os.ReadFile(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, path, fmt.Errorf("could not read %s: %w", path, err)
		}
		data := map[string]any{}
		if err := unmarshalManifestBytes(name, content, &data); err != nil {
			return nil, path, fmt.Errorf("could not parse %s: %w", path, err)
		}
		return data, path, nil
	}
	return nil, "", fmt.Errorf("no package.json or package.yaml in %s: %w", dir, os.ErrNotExist)
}

func unmarshalManifestBytes(name string, content []byte, target any) error {
	m, _ := encoding.Detect(name)
	if m == nil {
		return fmt.Errorf("unsupported package manifest extension: %s", filepath.Ext(name))
	}
	return m.Unmarshal(content, target)
}

func findManifestInDir(dir string) (string, error) {
	for _, name := range PackageManifestNames {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
	}
	return "", nil
}

// SearchupPackageManifest walks up from startDir looking for a directory that contains a package manifest (package.json
// or package.yaml) and returns the path of the manifest file. If both files exist in the same directory, package.json
// is preferred. Returns an error wrapping os.ErrNotExist if no manifest is found anywhere up the tree.
func SearchupPackageManifest(startDir string) (string, error) {
	dir := startDir
	for {
		path, err := findManifestInDir(dir)
		if err != nil {
			return "", err
		}
		if path != "" {
			return path, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no package.json or package.yaml found searching up from %s: %w",
				startDir, os.ErrNotExist)
		}
		dir = parent
	}
}
