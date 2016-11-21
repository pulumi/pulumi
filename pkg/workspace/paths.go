// Copyright 2016 Marapongo, Inc. All rights reserved.

package workspace

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/errors"
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

// InstallRootEnvvar is the envvar describing where Mu has been installed.
const InstallRootEnvvar = "MUROOT"

// DefaultInstallRoot is where Mu is installed by default, if the envvar is missing.
// TODO: support Windows.
const DefaultInstallRoot = "/usr/lib/mu"

// InstallRoot returns Mu's installation location.  This is controlled my the MUROOT envvar.
func InstallRoot() string {
	root := os.Getenv(InstallRootEnvvar)
	if root == "" {
		return DefaultInstallRoot
	}
	return root
}

// Workspace offers functionality for interacting with Mu workspaces.
type Workspace interface {
	// Root returns the base path of the current workspace.
	Root() string
	// DetectMufile locates the closest Mufile from the given path, searching "upwards" in the directory hierarchy.  If no
	// Mufile is found, an empty path is returned.  If problems are detected, they are logged to the diag.Sink.
	DetectMufile() (string, error)
}

// New creates a new workspace from the given starting path.
func New(path string, d diag.Sink) (Workspace, error) {
	// First normalize the path to an absolute one.
	var err error
	path, err = filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	ws := workspace{
		path: path,
		d:    d,
	}
	if _, err := ws.detectRoot(); err != nil {
		return nil, err
	}
	return &ws, nil
}

type workspace struct {
	path string    // the path at which the workspace was constructed.
	root string    // the root of the workspace.
	d    diag.Sink // a diagnostics sink to use for workspace operations.
}

// isTop returns true if the path represents the top of the filesystem.
func isTop(path string) bool {
	return os.IsPathSeparator(path[len(path)-1])
}

// pathDir returns the nearest directory to the workspace's path.
func (w *workspace) pathDir() (string, error) {
	// It's possible that the path is a file (e.g., a Mu.yaml file); if so, we want the directory.
	info, err := os.Stat(w.path)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return w.path, nil
	} else {
		return filepath.Dir(w.path), nil
	}
}

// detectRoot finds the root of the workspace and caches it for fast lookups.
func (w *workspace) detectRoot() (string, error) {
	if w.root == "" {
		root, err := w.pathDir()
		if err != nil {
			return "", err
		}

		// Now search for the root of the workspace so we can cache it.
	Search:
		for {
			files, err := ioutil.ReadDir(root)
			if err != nil {
				return "", err
			}
			for _, file := range files {
				// A muspace file delimits the root of the workspace.
				if file.Name() == Muspace {
					break Search
				}
			}

			// If neither succeeded, keep looking in our parent directory.
			root = filepath.Dir(root)
			if isTop(root) {
				// We reached the top of the filesystem.  Just set root back to the path and stop.
				root = w.path
				break
			}
		}

		w.root = root
	}

	return w.root, nil
}

func (w *workspace) DetectMufile() (string, error) {
	// It's possible the target is already the file we seek; if so, return right away.
	if IsMufile(w.path, w.d) {
		return w.path, nil
	}

	curr, err := w.pathDir()
	if err != nil {
		return "", err
	}
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
			if IsMufile(path, w.d) {
				return path, nil
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
		if isTop(curr) {
			break
		}
	}

	return "", nil
}

// Root returns the current workspace's root.
func (w *workspace) Root() string {
	return w.root
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
