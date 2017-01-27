// Copyright 2016 Marapongo, Inc. All rights reserved.

package workspace

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/marapongo/mu/pkg/compiler/errors"
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/encoding"
)

// Mufile is the base name of a Mufile.
const Mufile = "Mu"

// Muspace is the base name of a markup file containing settings shared amongst a workspace.
const Muspace = "Muspace"

// Mudeps is the directory in which dependency modules exist, either local to a workspace, or globally.
const Mudeps = ".Mudeps"

// InstallRootEnvvar is the envvar describing where Mu has been installed.
const InstallRootEnvvar = "MUROOT"

// InstallRootLibdir is the directory in which the standard Mu library exists.
const InstallRootLibdir = "lib"

// DefaultInstallRoot is where Mu is installed by default, if the envvar is missing.
// TODO: support Windows.
const DefaultInstallRoot = "/usr/local/mu"

// InstallRoot returns Mu's installation location.  This is controlled my the MUROOT envvar.
func InstallRoot() string {
	root := os.Getenv(InstallRootEnvvar)
	if root == "" {
		return DefaultInstallRoot
	}
	return root
}

// isTop returns true if the path represents the top of the filesystem.
func isTop(path string) bool {
	return os.IsPathSeparator(path[len(path)-1])
}

// pathDir returns the nearest directory to the given path (identity if a directory; parent otherwise).
func pathDir(path string) string {
	// It's possible that the path is a file (e.g., a Mu.yaml file); if so, we want the directory.
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return path
	}
	return filepath.Dir(path)
}

// DetectPackage locates the closest package from the given path, searching "upwards" in the directory hierarchy.  If no
// Mufile is found, an empty path is returned.  If problems are detected, they are logged to the diag.Sink.
func DetectPackage(path string, d diag.Sink) (string, error) {
	// It's possible the target is already the file we seek; if so, return right away.
	if IsMufile(path, d) {
		return path, nil
	}

	curr := pathDir(path)
	for {
		stop := false

		// Enumerate the current path's files, checking each to see if it's a Mufile.
		files, err := ioutil.ReadDir(curr)
		if err != nil {
			return "", err
		}
		for _, file := range files {
			name := file.Name()
			path := filepath.Join(curr, name)
			if IsMufile(path, d) {
				return path, nil
			} else if IsMuspace(path, d) {
				// If we hit a Muspace file, stop looking.
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

// IsMufile returns true if the path references what appears to be a valid Mufile.  If problems are detected -- like
// an incorrect extension -- they are logged to the provided diag.Sink (if non-nil).
func IsMufile(path string, d diag.Sink) bool {
	return isMarkupFile(path, Mufile, d)
}

// IsMuspace returns true if the path references what appears to be a valid Muspace file.  If problems are detected --
// like an incorrect extension -- they are logged to the provided diag.Sink (if non-nil).
func IsMuspace(path string, d diag.Sink) bool {
	return isMarkupFile(path, Muspace, d)
}

func isMarkupFile(path string, expect string, d diag.Sink) bool {
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
		if d != nil && strings.EqualFold(base, expect) {
			// If the strings aren't equal, but case-insensitively match, issue a warning.
			d.Warningf(errors.WarningIllegalMarkupFileCasing.AtFile(name), expect)
		}
		return false
	}

	// Check all supported extensions.
	for _, mext := range encoding.Exts {
		if name == expect+mext {
			return true
		}
	}

	// If we got here, it means the base name matched, but not the extension.  Warn and return.
	if d != nil {
		d.Warningf(errors.WarningIllegalMarkupFileExt.AtFile(name), expect, ext)
	}
	return false
}
