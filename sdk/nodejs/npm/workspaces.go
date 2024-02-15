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
		workspaces, err := parseWorkspaces(p)
		if err != nil {
			return "", fmt.Errorf("failed to parse workspaces from %s: %w", p, err)
		}
		for _, workspace := range workspaces {
			// See if any of the workspace glob results is the programDirectory.
			paths, err := filepath.Glob(filepath.Join(currentDir, workspace))
			if err != nil {
				return "", err
			}
			if paths != nil && slices.Contains(paths, programDirectory) {
				return currentDir, nil
			}
		}
		// None of the workspace globs matched the program directory, so we're
		// in the slightly weird situation where a parent directory has a
		// package.json with workspaces set up, but the program directory is
		// not part of this.
		return "", ErrNotInWorkspace
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
