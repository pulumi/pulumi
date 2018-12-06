// Copyright 2016-2018, Pulumi Corporation.
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

package workspace

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/encoding"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/fsutil"
)

const (
	// BackupDir is the name of the folder where backup stack information is stored.
	BackupDir = "backups"
	// BookkeepingDir is the name of our bookeeping folder, we store state here (like .git for git).
	BookkeepingDir = ".pulumi"
	// ConfigDir is the name of the folder that holds local configuration information.
	ConfigDir = "config"
	// GitDir is the name of the folder git uses to store information.
	GitDir = ".git"
	// HistoryDir is the name of the directory that holds historical information for projects.
	HistoryDir = "history"
	// PluginDir is the name of the directory containing plugins.
	PluginDir = "plugins"
	// StackDir is the name of the directory that holds stack information for projects.
	StackDir = "stacks"
	// TemplateDir is the name of the directory containing templates.
	TemplateDir = "templates"
	// WorkspaceDir is the name of the directory that holds workspace information for projects.
	WorkspaceDir = "workspaces"

	// ProjectFile is the base name of a project file.
	ProjectFile = "Pulumi"
	// RepoFile is the name of the file that holds information specific to the entire repository.
	RepoFile = "settings.json"
	// WorkspaceFile is the name of the file that holds workspace information.
	WorkspaceFile = "workspace.json"
	// CachedVersionFile is the name of the file we use to store when we last checked if the CLI was out of date
	CachedVersionFile = ".cachedVersionInfo"
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

// DetectProjectStackPath returns the name of the file to store stack specific project settings in. By default, we
// write this into a .pulumi folder next to the Pulumi.yaml file for the project, but we allow this behavior to be
// configured by a setting in the project file and also have some compatibility behavior to support our older model
// where we wrote the configuration YAML into a file next to Pulumi.yaml by default.
func DetectProjectStackPath(stackName tokens.QName) (string, error) {
	proj, projPath, err := DetectProjectAndPath()
	if err != nil {
		return "", err
	}

	configRoot := proj.Config

	// If the location has not been overridden and an existing configuration file doesn't exist next to Pulumi.yaml,
	// prefer storing in the .pulumi folder instead.
	if configRoot == "" {
		candidateFile := filepath.Join(filepath.Dir(projPath),
			fmt.Sprintf("%s.%s%s", ProjectFile, qnameFileName(stackName), filepath.Ext(projPath)))

		if _, err := os.Stat(candidateFile); os.IsNotExist(err) {
			configRoot = ".pulumi"
		}
	}

	return filepath.Join(filepath.Dir(projPath), configRoot, fmt.Sprintf("%s.%s%s", ProjectFile, qnameFileName(stackName),
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

// GetCachedVersionFilePath returns the location where the CLI caches information from pulumi.com on the newest
// available version of the CLI
func GetCachedVersionFilePath() (string, error) {
	user, err := user.Current()
	if err != nil {
		return "", err
	}

	return filepath.Join(user.HomeDir, BookkeepingDir, CachedVersionFile), nil
}
