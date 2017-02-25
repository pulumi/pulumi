// Copyright 2016 Pulumi, Inc. All rights reserved.

package workspace

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/pulumi/coconut/pkg/compiler/errors"
	"github.com/pulumi/coconut/pkg/diag"
	"github.com/pulumi/coconut/pkg/encoding"
	"github.com/pulumi/coconut/pkg/tokens"
)

const Nutfile = "Nut"           // the base name of a Nutfile.
const Nutpack = "Nutpack"       // the base name of a compiled NutPack.
const NutpackOutDir = "nutpack" // the default name of the NutPack output directory.
const NutpackBinDir = "bin"     // the default name of the NutPack binary output directory.
const NutpackHusksDir = "husks" // the default name of the NutPack husks directory.
const Nutspace = "Coconut"      // the base name of a markup file for shared settings in a workspace.
const Nutdeps = ".Nuts"         // the directory in which dependencies exist, either local or global.

const InstallRootEnvvar = "COCOROOT"            // the envvar describing where Coconut has been installed.
const InstallRootLibdir = "lib"                 // the directory in which the Coconut standard library exists.
const DefaultInstallRoot = "/usr/local/coconut" // where Coconut is installed by default.

// InstallRoot returns Coconut's installation location.  This is controlled my the COCOROOT envvar.
func InstallRoot() string {
	// TODO: support Windows.
	root := os.Getenv(InstallRootEnvvar)
	if root == "" {
		return DefaultInstallRoot
	}
	return root
}

// HuskPath returns a path to the given husk's default location.
func HuskPath(husk tokens.QName) string {
	return filepath.Join(NutpackOutDir, NutpackHusksDir, qnamePath(husk)+encoding.Exts[0])
}

// isTop returns true if the path represents the top of the filesystem.
func isTop(path string) bool {
	return os.IsPathSeparator(path[len(path)-1])
}

// pathDir returns the nearest directory to the given path (identity if a directory; parent otherwise).
func pathDir(path string) string {
	// It's possible that the path is a file (e.g., a Nut.yaml file); if so, we want the directory.
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return path
	}
	return filepath.Dir(path)
}

// DetectPackage locates the closest package from the given path, searching "upwards" in the directory hierarchy.  If no
// Nutfile is found, an empty path is returned.  If problems are detected, they are logged to the diag.Sink.
func DetectPackage(path string, d diag.Sink) (string, error) {
	// It's possible the target is already the file we seek; if so, return right away.
	if IsNutfile(path, d) {
		return path, nil
	}

	curr := pathDir(path)
	for {
		stop := false

		// Enumerate the current path's files, checking each to see if it's a Nutfile.
		files, err := ioutil.ReadDir(curr)
		if err != nil {
			return "", err
		}

		// See if there's a compiled Nutpack in the expected location.
		pack := filepath.Join(NutpackOutDir, NutpackBinDir, Nutpack)
		for _, ext := range encoding.Exts {
			packfile := pack + ext
			if IsNutpack(packfile, d) {
				return packfile, nil
			}
		}

		// Now look for individual Nutfiles.
		for _, file := range files {
			name := file.Name()
			path := filepath.Join(curr, name)
			if IsNutfile(path, d) {
				return path, nil
			} else if IsNutspace(path, d) {
				// If we hit a Nutspace file, stop looking.
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

// IsNutfile returns true if the path references what appears to be a valid Nutfile.  If problems are detected -- like
// an incorrect extension -- they are logged to the provided diag.Sink (if non-nil).
func IsNutfile(path string, d diag.Sink) bool {
	return isMarkupFile(path, Nutfile, d)
}

// IsNutpack returns true if the path references what appears to be a valid Nutpack.  If problems are detected -- like
// an incorrect extension -- they are logged to the provided diag.Sink (if non-nil).
func IsNutpack(path string, d diag.Sink) bool {
	return isMarkupFile(path, Nutpack, d)
}

// IsNutspace returns true if the path references what appears to be a valid Nutspace file.  If problems are detected --
// like an incorrect extension -- they are logged to the provided diag.Sink (if non-nil).
func IsNutspace(path string, d diag.Sink) bool {
	return isMarkupFile(path, Nutspace, d)
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
