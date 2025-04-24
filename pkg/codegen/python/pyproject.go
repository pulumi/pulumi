// Copyright 2023-2024, Pulumi Corporation.
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

package python

// The specification for the pyproject.toml file can be found here.
// https://packaging.python.org/en/latest/specifications/declaring-project-metadata/
type PyprojectSchema struct {
	Project     *Project               `toml:"project,omitempty" json:"project,omitempty"`
	BuildSystem *BuildSystem           `toml:"build-system,omitempty" json:"build-system,omitempty"`
	Tool        map[string]interface{} `toml:"tool,omitempty" json:"tool,omitempty"`
}

// Project is a view layer for a pyproject.toml file.
type Project struct {
	Name         *string     `toml:"name,omitempty" json:"name,omitempty"`
	Authors      []Contact   `toml:"authors,omitempty" json:"authors,omitempty"`
	Classifiers  []string    `toml:"classifiers,omitempty" json:"classifiers,omitempty"`
	Description  *string     `toml:"description,omitempty" json:"description,omitempty"`
	Dependencies []string    `toml:"dependencies,omitempty" json:"dependencies,omitempty"`
	Dynamic      []string    `toml:"dynamic,omitempty" json:"dynamic,omitempty"`
	EntryPoints  Entrypoints `toml:"entry-points,omitempty" json:"entry-points,omitempty"`
	GUIScripts   Entrypoints `toml:"gui-scripts,omitempty" json:"gui-scripts,omitempty"`
	// These are keywords used in package search.
	Keywords    []string  `toml:"keywords,omitempty" json:"keywords,omitempty"`
	License     *License  `toml:"license,omitempty" json:"license,omitempty"`
	Maintainers []Contact `toml:"maintainers,omitempty" json:"maintainers,omitempty"`
	//nolint:lll
	OptionalDependencies OptionalDependencies `toml:"optional-dependencies,omitempty" json:"optional-dependencies,omitempty"`
	// README is a path to a .md file or a .rst file
	README *string `toml:"readme,omitempty" json:"readme,omitempty"`
	// The version constraint e.g. ">=3.8"
	RequiresPython *string     `toml:"requires-python,omitempty" json:"requires-python,omitempty"`
	Scripts        Entrypoints `toml:"scripts,omitempty" json:"scripts,omitempty"`
	// URLs provides core metadata about this project's website, a link
	// to the repo, project documentation, and the project homepage.
	URLs map[string]string `toml:"urls,omitempty" json:"urls,omitempty"`
	// Version is the package version.
	Version *string `toml:"version,omitempty" json:"version,omitempty"`
}

type BuildSystem struct {
	Requires     []string `toml:"requires,omitempty" json:"requires,omitempty"`
	BuildBackend string   `toml:"build-backend,omitempty" json:"build-backend,omitempty"`
}

// Contact references someone associated with the project, including
// their contact information. Contacts are used for both Authors and
// Maintainers, since both fields have the same schema and specification.
// It is often easier to specify both fields,
// but the precise rules for specifying either one or the other field
// can be found here:
// https://packaging.python.org/en/latest/specifications/declaring-project-metadata/#authors-maintainers
type Contact struct {
	Name  string `toml:"name,omitempty" json:"name,omitempty"`
	Email string `toml:"email,omitempty" json:"email,omitempty"`
}

// An Entrypoint is an object reference for an executable Python script. These
// scripts can be applications, plugins, or build-time metadata. Since Pulumi
// distributes libraries, we largely don't use this field, though we include it
// for completeness and consistency with the spec.
type Entrypoints map[string]string

// The license instance must populate either
// file or text, but not both. File is a path
// to a license file, while text is either the
// name of the license, or the text of the license.
type License struct {
	File string `toml:"file,omitempty" json:"file,omitempty"`
	Text string `toml:"text,omitempty" json:"text,omitempty"`
}

// OptionalDependencies provides a map from "Extras" (parlance specific to Python)
// to their dependencies. Each value in the array becomes a required dependency
// if the Extra is enabled.
type OptionalDependencies map[string][]string
