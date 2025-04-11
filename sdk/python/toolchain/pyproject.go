// Copyright 2025, Pulumi Corporation.
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

package toolchain

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type BuildSystem struct {
	Requires     []string `toml:"requires,omitempty" json:"requires,omitempty"`
	BuildBackend string   `toml:"build-backend,omitempty" json:"build-backend,omitempty"`
}

type Project struct {
	Name string
}

type Pyproject struct {
	Project     Project        `toml:"project" json:"project"`
	BuildSystem *BuildSystem   `toml:"build-system,omitempty" json:"build-system,omitempty"`
	Tool        map[string]any `toml:"tool,omitempty" json:"tool,omitempty"`
}

// IsBuildablePackage checks if a directory is a buildable Python package. A
// directory is considered a buildable package if it contains a pyproject.toml
// file with a build-system section.
func IsBuildablePackage(dir string) (bool, error) {
	pyproject, err := LoadPyproject(dir)
	if err != nil {
		return false, fmt.Errorf("parsing pyproject.toml: %w", err)
	}
	return pyproject.BuildSystem != nil && pyproject.BuildSystem.BuildBackend != "", nil
}

func LoadPyproject(dir string) (Pyproject, error) {
	pyprojectToml := filepath.Join(dir, "pyproject.toml")
	_, err := os.Stat(pyprojectToml)
	if err != nil {
		if os.IsNotExist(err) {
			return Pyproject{}, nil
		}
		return Pyproject{}, fmt.Errorf("checking pyproject.toml: %w", err)
	}

	b, err := os.ReadFile(pyprojectToml)
	if err != nil {
		return Pyproject{}, fmt.Errorf("reading %s: %w", pyprojectToml, err)
	}
	var pyproject Pyproject
	if err := toml.Unmarshal(b, &pyproject); err != nil {
		return Pyproject{}, fmt.Errorf("unmarshaling pyproject.toml: %w", err)
	}
	return pyproject, nil
}
