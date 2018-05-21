// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/encoding"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/fsutil"
)

const (
	BackupDir      = "backups"    // the name of the folder where backup stack information is stored.
	BookkeepingDir = ".pulumi"    // the name of our bookeeping folder, we store state here (like .git for git).
	ConfigDir      = "config"     // the name of the folder that holds local configuration information.
	GitDir         = ".git"       // the name of the folder git uses to store information.
	HistoryDir     = "history"    // the name of the directory that holds historical information for projects.
	PluginDir      = "plugins"    // the name of the directory containing plugins.
	StackDir       = "stacks"     // the name of the directory that holds stack information for projects.
	TemplateDir    = "templates"  // the name of the directory containing templates.
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

// DetectProjectStackPath returns the name of the file to store stack specific project settings in. We place stack
// specific settings next to the Pulumi.yaml file, named like: Pulumi.<stack-name>.yaml
func DetectProjectStackPath(stackName tokens.QName) (string, error) {
	projPath, err := DetectProjectPath()
	if err != nil {
		return "", err
	}

	return filepath.Join(filepath.Dir(projPath), fmt.Sprintf("%s.%s%s", ProjectFile, qnameFileName(stackName),
		filepath.Ext(projPath))), nil
}

// DetectProjectPathFrom locates the closest project from the given path, searching "upwards" in the directory
// hierarchy.  If no project is found, an empty path is returned.
func DetectProjectPathFrom(path string) (string, error) {
	return fsutil.WalkUp(path, isProject, func(s string) bool {
		return true
	})
}

// DetectProject loads the closest project from the current working directory, or an error if not found.
func DetectProject() (*Project, error) {
	proj, _, err := DetectProjectAndPath()
	return proj, err
}

func DetectProjectStack(stackName tokens.QName) (*ProjectStack, error) {
	path, err := DetectProjectStackPath(stackName)
	if err != nil {
		return nil, err
	}

	return LoadProjectStack(path)
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

func SaveProjectStack(stackName tokens.QName, stack *ProjectStack) error {
	path, err := DetectProjectStackPath(stackName)
	if err != nil {
		return err
	}

	return stack.Save(path)
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
