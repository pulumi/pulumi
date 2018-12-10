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
// write this into a .pulumi folder next to the Pulumi.yaml file for the project, and for stacks managed by the
// pulumi service, where there is the concept of an "owner" we further segment these files into directories named after
// the owner.
//
// The ".pulumi" folder may be configured by setting the `config` property in the Project's Pulumi.yaml file.
//
// For compatibility, we have a few other paths that we prefer (when an actual file exists on disk), but when no
// existing configuration file is present, we use the above path.
func DetectProjectStackPath(owner string, stackName tokens.QName) (string, error) {
	proj, projPath, err := DetectProjectAndPath()
	if err != nil {
		return "", err
	}

	projectRoot := filepath.Dir(projPath)
	configRoot := proj.Config
	stackFileName := qnameFileName(stackName)
	stackFileExt := filepath.Ext(projPath)

	// As our configuration system has evolved, we've made some changes to where we store stack specific configuration
	// on disk. `candidates` is slice of possible paths to look. We start at the first element of the slice and return
	// the first path that exists (so these are ordered from older formats to newer formats). If none of these files
	// exist, we say the path is the last element in this slice (and so that should be the most preferred format).
	candidates := []string{
		filepath.Join(projectRoot, configRoot, fmt.Sprintf("%s.%s%s", ProjectFile, stackFileName, stackFileExt)),
		filepath.Join(projectRoot, configRoot, owner, fmt.Sprintf("%s.%s%s", ProjectFile, stackFileName, stackFileExt)),
	}

	// When configRoot is unset, we also include ".pulumi" at the end of our candiates lists. We do this because we'd
	// like an unset configuration root to mean ".pulumi", a change in behavior from the old default where we would
	// just write them into next to Pulumi.yaml.
	if configRoot == "" {
		candidates = append(candidates, filepath.Join(projectRoot, ".pulumi",
			fmt.Sprintf("%s.%s%s", ProjectFile, stackFileName, stackFileExt)))
		candidates = append(candidates, filepath.Join(projectRoot, ".pulumi", owner,
			fmt.Sprintf("%s.%s%s", ProjectFile, stackFileName, stackFileExt)))
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	// In the case where none of the candidate file exists, the last one in the candidate list is the canonical path
	// we'd prefer to use.
	return candidates[len(candidates)-1], nil
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

func DetectProjectStack(owner string, stackName tokens.QName) (*ProjectStack, error) {
	path, err := DetectProjectStackPath(owner, stackName)
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

func SaveProjectStack(owner string, stackName tokens.QName, stack *ProjectStack) error {
	path, err := DetectProjectStackPath(owner, stackName)
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
