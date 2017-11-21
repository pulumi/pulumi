// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package workspace

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/pulumi/pulumi/pkg/util/fsutil"

	"github.com/pulumi/pulumi/pkg/encoding"
)

const ProjectFile = "Pulumi"           // the base name of a project file.
const GitDir = ".git"                  // the name of the folder git uses to store information.
const BookkeepingDir = ".pulumi"       // the name of our bookeeping folder, we store state here (like .git for git).
const StackDir = "stacks"              // the name of the directory that holds stack information for projects.
const WorkspaceDir = "workspaces"      // the name of the directory that holds workspace information for projects.
const RepoFile = "settings.json"       // the name of the file that holds information specific to the entire repository.
const ConfigDir = "config"             // the name of the folder that holds local configuration information.
const WorkspaceFile = "workspace.json" // the name of the file that holds workspace information.

// DetectPackage locates the closest package from the given path, searching "upwards" in the directory hierarchy.  If no
// Project is found, an empty path is returned.  If problems are detected, they are logged to the diag.Sink.
func DetectPackage(path string) (string, error) {
	return fsutil.WalkUp(path, isProject, func(s string) bool {
		return !isRepositoryFolder(filepath.Join(s, BookkeepingDir))
	})
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
