// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package workspace

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/pulumi/pulumi/pkg/encoding"
	"github.com/pulumi/pulumi/pkg/pack"
	"github.com/pulumi/pulumi/pkg/util/fsutil"
)

const ProjectFile = "Pulumi"           // the base name of a project file.
const GitDir = ".git"                  // the name of the folder git uses to store information.
const BookkeepingDir = ".pulumi"       // the name of our bookeeping folder, we store state here (like .git for git).
const StackDir = "stacks"              // the name of the directory that holds stack information for projects.
const WorkspaceDir = "workspaces"      // the name of the directory that holds workspace information for projects.
const RepoFile = "settings.json"       // the name of the file that holds information specific to the entire repository.
const ConfigDir = "config"             // the name of the folder that holds local configuration information.
const WorkspaceFile = "workspace.json" // the name of the file that holds workspace information.
const IgnoreFile = ".pulumiignore"     // the name of the file that we use to control what to upload to the service.

// DetectPackage locates the closest package from the current working directory, or an error if not found.
func DetectPackage() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	pkgPath, err := DetectPackageFrom(dir)
	if err != nil {
		return "", err
	}

	return pkgPath, nil
}

// DetectPackageFrom locates the closest package from the given path, searching "upwards" in the directory hierarchy.
// If no Project is found, an empty path is returned.  If problems are detected, they are logged to the diag.Sink.
func DetectPackageFrom(path string) (string, error) {
	return fsutil.WalkUp(path, isProject, func(s string) bool {
		return !isRepositoryFolder(filepath.Join(s, BookkeepingDir))
	})
}

// GetPackage loads the closest package from the current working directory, or an error if not found.
func GetPackage() (*pack.Package, error) {
	pkgPath, err := DetectPackage()
	if err != nil {
		return nil, err
	}

	return pack.Load(pkgPath)
}

// GetPackagePath loads the closest package from the current working directory, or an error if not found.  It
// also returns the path where the package was found.
func GetPackagePath() (*pack.Package, string, error) {
	pkgPath, err := DetectPackage()
	if err != nil {
		return nil, "", err
	}

	pkg, err := pack.Load(pkgPath)
	if err != nil {
		return nil, "", err
	}

	return pkg, pkgPath, err
}

// SavePackage saves the package file on top of the existing one.
func SavePackage(pkg *pack.Package) error {
	pkgPath, err := DetectPackage()
	if err != nil {
		return err
	}

	return pack.Save(pkgPath, pkg)
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
