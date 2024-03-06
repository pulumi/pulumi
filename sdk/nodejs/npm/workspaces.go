// Copyright 2016-2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package npm

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"gopkg.in/yaml.v3"
)

var ErrNotInWorkspace = errors.New("not in a workspace")

// FindWorkspaceRoot determines if we are in a yarn/npm workspace setup and
// returns the root directory of the workspace.  If the programDirectory is
// not in a workspace, it returns ErrNotInWorkspace.
func FindWorkspaceRoot(programDirectory string) (string, error) {
	currentDir := filepath.Dir(programDirectory)
	nextDir := filepath.Dir(currentDir)
	for currentDir != nextDir { // We're at the root when the nextDir is the same as the currentDir.
		p := filepath.Join(currentDir, "package.json")
		_, err := os.Stat(p)
		if err != nil {
			if os.IsNotExist(err) {
				// No package.json in this directory, continue the search in the next directory up.
				currentDir = nextDir
				nextDir = filepath.Dir(currentDir)
				continue
			}
			return "", err
		}

		// First look for pnpm workspace configuration
		pnpmWorkspace := filepath.Join(currentDir, "pnpm-workspace.yaml")
		_, err = os.Stat(pnpmWorkspace)
		if err == nil {
			// We have a pnpm-workspace.yaml
			workspaces, err := parsePnpmWorkspace(pnpmWorkspace)
			if err != nil {
				return "", err
			}
			matches, err := matchesWorkspaceGlobs(workspaces, currentDir, programDirectory)
			if err != nil {
				return "", err
			}
			if matches {
				return currentDir, nil
			}
		}

		// No pnpm-workspace.yaml, check for npm/yarn workspaces in the package.json
		workspaces, err := parseWorkspaces(p)
		if err != nil {
			return "", fmt.Errorf("failed to parse workspaces from %s: %w", p, err)
		}
		matches, err := matchesWorkspaceGlobs(workspaces, currentDir, programDirectory)
		if err != nil {
			return "", err
		}
		if matches {
			return currentDir, nil
		}

		// None of the workspace globs matched the program directory, so we're
		// in the slightly weird situation where a parent directory has a
		// package.json with workspaces set up, but the program directory is
		// not part of this.
		return "", ErrNotInWorkspace
	}

	// We walked all the way to the root and didn't find a workspace configuration.
	return "", ErrNotInWorkspace
}

// matchesWorkspaceGlobs checks if the program directory matches any of the workspaces.
func matchesWorkspaceGlobs(workspaces []string, currentDir string, programDirectory string) (bool, error) {
	for _, workspace := range workspaces {
		paths, err := filepath.Glob(filepath.Join(currentDir, workspace, "package.json"))
		if err != nil {
			return false, err
		}
		if paths != nil && slices.Contains(paths, filepath.Join(programDirectory, "package.json")) {
			return true, nil
		}
	}
	return false, ErrNotInWorkspace
}

// parseWorkspaces reads a package.json file and returns the list of workspaces.
// This supports the simple format for npm and yarn:
//
//	{
//	  "workspaces": ["workspace-a", "workspace-b"]
//	}
//
// As well as the extended format for yarn:
//
//	{
//		"workspaces": {
//			"packages": ["packages/*"],
//			"nohoist": ["**/react-native", "**/react-native/**"]
//		}
//	}
func parseWorkspaces(p string) ([]string, error) {
	pkgContents, err := os.ReadFile(p)
	if err != nil {
		return []string{}, err
	}
	pkg := struct {
		Workspaces []string `json:"workspaces"`
	}{}
	err = json.Unmarshal(pkgContents, &pkg)
	if err == nil {
		return pkg.Workspaces, nil
	}
	// Failed to parse the simple format, try to parse extended yarn workspaces format
	pkgExtended := struct {
		Workspaces struct {
			Packages []string `json:"packages"`
		} `json:"workspaces"`
	}{}
	err = json.Unmarshal(pkgContents, &pkgExtended)
	if err != nil {
		return []string{}, err
	}
	return pkgExtended.Workspaces.Packages, nil
}

func parsePnpmWorkspace(p string) ([]string, error) {
	workspaceContent, err := os.ReadFile(p)
	if err != nil {
		return []string{}, err
	}
	workspace := struct {
		Packages []string `yaml:"packages"`
	}{}
	err = yaml.Unmarshal(workspaceContent, &workspace)
	if err != nil {
		return []string{}, err
	}
	return workspace.Packages, nil
}
