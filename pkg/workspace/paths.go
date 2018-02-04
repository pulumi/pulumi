// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package workspace

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/encoding"
	"github.com/pulumi/pulumi/pkg/util/fsutil"
)

const (
	BookkeepingDir = ".pulumi"    // the name of our bookeeping folder, we store state here (like .git for git).
	ConfigDir      = "config"     // the name of the folder that holds local configuration information.
	GitDir         = ".git"       // the name of the folder git uses to store information.
	HistoryDir     = "history"    // the name of the directory that holds historical information for projects.
	PluginDir      = "plugins"    // the name of the directory containing plugins.
	StackDir       = "stacks"     // the name of the directory that holds stack information for projects.
	WorkspaceDir   = "workspaces" // the name of the directory that holds workspace information for projects.

	IgnoreFile    = ".pulumiignore"  // the name of the file that we use to control what to upload to the service.
	ProjectFile   = "Pulumi"         // the base name of a project file.
	RepoFile      = "settings.json"  // the name of the file that holds information specific to the entire repository.
	WorkspaceFile = "workspace.json" // the name of the file that holds workspace information.
)

// DetectProjectPath locates the closest project from the current working directory, or an error if not found.
func DetectProjectPath() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	path, err := DetectProjectPathFrom(dir)
	if err != nil {
		return "", err
	}

	return path, nil
}

// DetectProjectPathFrom locates the closest project from the given path, searching "upwards" in the directory
// hierarchy.  If no project is found, an empty path is returned.  If problems are detected, they are logged to
// the diag.Sink.
func DetectProjectPathFrom(path string) (string, error) {
	return fsutil.WalkUp(path, isProject, func(s string) bool {
		return !isRepositoryFolder(filepath.Join(s, BookkeepingDir))
	})
}

// DetectProject loads the closest project from the current working directory, or an error if not found.
func DetectProject() (*Project, error) {
	proj, _, err := DetectProjectAndPath()
	return proj, err
}

// DetectProjectAndPath loads the closest package from the current working directory, or an error if not found.  It
// also returns the path where the package was found.
func DetectProjectAndPath() (*Project, string, error) {
	path, err := DetectProjectPath()
	if err != nil {
		return nil, "", err
	} else if path == "" {
		return nil, "", errors.Errorf("no Pulumi project found in the current working directory")
	}

	proj, err := LoadProject(path)
	return proj, path, err
}

// SaveProject saves the project file on top of the existing one, using the standard location.
func SaveProject(proj *Project) error {
	path, err := DetectProjectPath()
	if err != nil {
		return err
	}

	return proj.Save(path)
}

func isGitFolder(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir() && info.Name() == ".git"
}

func isRepositoryFolder(path string) bool {
	info, err := os.Stat(path)
	if err == nil && info.IsDir() && info.Name() == BookkeepingDir {
		// make sure it has a settings.json file in it
		info, err := os.Stat(filepath.Join(path, RepoFile))
		return err == nil && !info.IsDir()
	}

	return false
}

// isProject returns true if the path references what appears to be a valid project.  If problems are detected -- like
// an incorrect extension -- they are logged to the provided diag.Sink (if non-nil).
func isProject(path string) bool {
	return isMarkupFile(path, ProjectFile)
}

func isMarkupFile(path string, expect string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		// Missing files and directories can't be markup files.
		return false
	}

	// Ensure the base name is expected.
	name := info.Name()
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	if base != expect {
		return false
	}

	// Check all supported extensions.
	for _, mext := range encoding.Exts {
		if name == expect+mext {
			return true
		}
	}

	return false
}
