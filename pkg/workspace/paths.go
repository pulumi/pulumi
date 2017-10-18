// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package workspace

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/pulumi/pulumi/pkg/encoding"
	"github.com/pulumi/pulumi/pkg/tokens"
)

const ProjectFile = "Pulumi"     // the base name of a Project.
const Dir = ".pulumi"            // the default name of the LumiPack output directory.
const StackDir = "env"           // the default name of the LumiPack stack directory.
const DepDir = "packs"           // the directory in which dependencies exist, either local or global.
const SettingsFile = "workspace" // the base name of a markup file for shared settings in a workspace.

// StackPath returns a path to the given stack's default location.
func StackPath(stack tokens.QName) string {
	path := filepath.Join(Dir, StackDir)
	if stack != "" {
		path = filepath.Join(path, qnamePath(stack)+encoding.Exts[0])
	}
	return path
}

// isTop returns true if the path represents the top of the filesystem.
func isTop(path string) bool {
	return os.IsPathSeparator(path[len(path)-1])
}

// pathDir returns the nearest directory to the given path (identity if a directory; parent otherwise).
func pathDir(path string) string {
	// It's possible that the path is a file (e.g., a Lumi.yaml file); if so, we want the directory.
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return path
	}
	return filepath.Dir(path)
}

// DetectPackage locates the closest package from the given path, searching "upwards" in the directory hierarchy.  If no
// Project is found, an empty path is returned.  If problems are detected, they are logged to the diag.Sink.
func DetectPackage(path string) (string, error) {
	// It's possible the target is already the file we seek; if so, return right away.
	if IsProject(path) {
		return path, nil
	}

	curr := pathDir(path)
	for {
		stop := false

		// Enumerate the current path's files, checking each to see if it's a Project.
		files, err := ioutil.ReadDir(curr)
		if err != nil {
			return "", err
		}
		for _, file := range files {
			name := file.Name()
			path := filepath.Join(curr, name)
			if IsProject(path) {
				return path, nil
			} else if IsLumiDir(path) {
				// If we hit a workspace, stop looking.
				stop = true
			}
		}

		// If we encountered a stop condition, break out of the loop.
		if stop {
			break
		}

		// If neither succeeded, keep looking in our parent directory.
		curr = filepath.Dir(curr)
		if isTop(curr) {
			break
		}
	}

	return "", nil
}

// IsLumiDir returns true if the target is a Lumi directory.
func IsLumiDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir() && info.Name() == Dir
}

// IsProject returns true if the path references what appears to be a valid project.  If problems are detected -- like
// an incorrect extension -- they are logged to the provided diag.Sink (if non-nil).
func IsProject(path string) bool {
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
