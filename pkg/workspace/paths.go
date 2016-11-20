// Copyright 2016 Marapongo, Inc. All rights reserved.

package workspace

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/errors"
	"github.com/marapongo/mu/pkg/util"
)

// Mufile is the base name of a Mufile.
const Mufile = "Mu"

// Muspace is a directory containing settings, modules, etc., delimiting a workspace.
const Muspace = ".mu"

// MuspaceStacks is the directory in which dependency modules exist, either local to a workspace, or globally.
const MuspaceStacks = "stacks"

// MuspaceWorkspace is the base name of a workspace settings file.
const MuspaceWorkspace = "workspace"

// Exts contains a list of all the valid Mufile and Mucluster extensions.
var Exts = []string{
	".json",
	".yaml",
	// Although ".yml" is not a sanctioned YAML extension, it is used quite broadly; so we will support it.
	".yml",
}

// DetectMufile locates the closest Mufile from the given path, searching "upwards" in the directory hierarchy.  If no
// Mufile is found, an empty path is returned.  If problems are detected, they are logged to the diag.Sink.
func DetectMufile(from string, d diag.Sink) string {
	abs, err := filepath.Abs(from)
	util.AssertMF(err == nil, "An IO error occurred while searching for a Mufile: %v", err)

	// It's possible the target is already the file we seek; if so, return right away.
	if IsMufile(abs, d) {
		return abs
	}

	curr := abs
	for {
		stop := false

		// If the target is a directory, enumerate its files, checking each to see if it's a Mufile.
		files, err := ioutil.ReadDir(curr)
		util.AssertMF(err == nil, "An IO error occurred while searching for a Mufile: %v", err)
		for _, file := range files {
			name := file.Name()
			path := filepath.Join(curr, name)
			if IsMufile(path, d) {
				return path
			} else if name == Muspace {
				// If we hit a .muspace file, stop looking.
				stop = true
			}
		}

		// If we encountered a stop condition, break out of the loop.
		if stop {
			break
		}

		// If neither succeeded, keep looking in our parent directory.
		curr = filepath.Dir(curr)
		if os.IsPathSeparator(curr[len(curr)-1]) {
			break
		}
	}

	return ""
}

// IsMufile returns true if the path references what appears to be a valid Mufile.  If problems are detected -- like
// an incorrect extension -- they are logged to the provided diag.Sink (if non-nil).
func IsMufile(path string, d diag.Sink) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	// Directories can't be Mufiles.
	if info.IsDir() {
		return false
	}

	// Ensure the base name is expected.
	name := info.Name()
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	if base != Mufile {
		if d != nil && strings.EqualFold(base, Mufile) {
			// If the strings aren't equal, but case-insensitively match, issue a warning.
			d.Warningf(errors.WarnIllegalMufileCasing.WithFile(name))
		}
		return false
	}

	// Check all supported extensions.
	for _, mufileExt := range Exts {
		if name == Mufile+mufileExt {
			return true
		}
	}

	// If we got here, it means the base name matched, but not the extension.  Warn and return.
	if d != nil {
		d.Warningf(errors.WarnIllegalMufileExt.WithFile(name), ext)
	}
	return false
}
