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
)

var ErrNotInWorkspace = errors.New("not in a workspace")

// FindWorkspaceRoot determines if we are in a yarn/npm workspace setup and
// returns the root directory of the workspace.  If the startingPath is
// not in a workspace, it returns ErrNotInWorkspace.
func FindWorkspaceRoot(startingPath string) (string, error) {
	stat, err := os.Stat(startingPath)
	if err != nil {
		return "", err
	}
	if !stat.IsDir() {
		startingPath = filepath.Dir(startingPath)
	}
	// We start at the location of the first `package.json` we find.
	packageJSONDir, err := searchup(startingPath, "package.json")
	if err != nil {
		return "", fmt.Errorf("did not find package.json in %s: %w", startingPath, err)
	}
	currentDir := packageJSONDir
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
		workspaces, err := parseWorkspaces(p)
		if err != nil {
			return "", fmt.Errorf("failed to parse workspaces from %s: %w", p, err)
		}
		for _, workspace := range workspaces {
			// See if any of the workspace glob results is the programDirectory.
			paths, err := filepath.Glob(filepath.Join(currentDir, workspace, "package.json"))
			if err != nil {
				return "", err
			}
			if paths != nil && slices.Contains(paths, filepath.Join(packageJSONDir, "package.json")) {
				return currentDir, nil
			}
		}
		currentDir = nextDir
		nextDir = filepath.Dir(currentDir)
	}
	return "", ErrNotInWorkspace
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

func searchup(currentDir, fileToFind string) (string, error) {
	if _, err := os.Stat(filepath.Join(currentDir, fileToFind)); err == nil {
		return currentDir, nil
	}
	parentDir := filepath.Dir(currentDir)
	if currentDir == parentDir {
		return "", nil
	}
	return searchup(parentDir, fileToFind)
}
