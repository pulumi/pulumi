// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package workspace

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/glog"
	homedir "github.com/mitchellh/go-homedir"

	"github.com/pulumi/pulumi-fabric/pkg/diag"
	"github.com/pulumi/pulumi-fabric/pkg/encoding"
	"github.com/pulumi/pulumi-fabric/pkg/tokens"
	"github.com/pulumi/pulumi-fabric/pkg/util/contract"
)

// W offers functionality for interacting with Lumi workspaces.  A workspace influences compilation; for example, it
// can specify default versions of dependencies, easing the process of working with multiple projects.
type W interface {
	Path() string                   // the base path of the current workspace.
	Root() string                   // the root path of the current workspace.
	Settings() *Settings            // returns a mutable pointer to the optional workspace settings info.
	DetectPackage() (string, error) // locates the nearest project file in the directory hierarchy.
	Save() error                    // saves any modifications to the workspace.
}

// New creates a new workspace from the given starting path.
func New(path string, diag diag.Sink) (W, error) {
	contract.Requiref(diag != nil, "diag", "!= nil")

	// First normalize the path to an absolute one.
	var err error
	path, err = filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	home, err := homedir.Dir()
	if err != nil {
		return nil, err
	}

	ws := workspace{
		diag: diag,
		path: path,
		home: home,
	}

	// Perform our I/O: memoize the root directory and load up any settings before returning.
	if err := ws.init(); err != nil {
		return nil, err
	}

	return &ws, nil
}

type workspace struct {
	diag     diag.Sink // the diagnostics sink to use for messages.
	path     string    // the path at which the workspace was constructed.
	home     string    // the home directory to use for this workspace.
	root     string    // the root of the workspace.
	settings Settings  // an optional bag of workspace-wide settings.
}

// init finds the root of the workspace, caches it for fast lookups, and loads up any workspace settings.
func (w *workspace) init() error {
	if w.root == "" {
		// Detect the root of the workspace and cache it.
		root := pathDir(w.path)
	Search:
		for {
			files, err := ioutil.ReadDir(root)
			if err != nil {
				return err
			}
			for _, file := range files {
				// A lumi directory delimits the root of the workspace.
				lumidir := filepath.Join(root, file.Name())
				if IsLumiDir(lumidir) {
					glog.V(3).Infof("Lumi workspace detected; setting root to %v", w.root)
					w.root = root                      // remember the root.
					w.settings, err = w.readSettings() // load up optional settings.
					if err != nil {
						return err
					}
					break Search
				}
			}

			// If neither succeeded, keep looking in our parent directory.
			if root = filepath.Dir(root); isTop(root) {
				// We reached the top of the filesystem.  Just set root back to the path and stop.
				glog.V(3).Infof("No Lumi workspace found; defaulting to current path %v", w.root)
				w.root = w.path
				break
			}
		}
	}

	return nil
}

func (w *workspace) Path() string        { return w.path }
func (w *workspace) Root() string        { return w.root }
func (w *workspace) Settings() *Settings { return &w.settings }

func (w *workspace) DetectPackage() (string, error) {
	return DetectPackage(w.path, w.diag)
}

// qnamePath just cleans a name and makes sure it's appropriate to use as a path.
func qnamePath(nm tokens.QName) string {
	return stringNamePath(string(nm))
}

// packageNamePath just cleans a package name and makes sure it's appropriate to use as a path.
func packageNamePath(nm tokens.PackageName) string {
	return stringNamePath(string(nm))
}

// stringNamePart cleans a string component of a name and makes sure it's appropriate to use as a path.
func stringNamePath(nm string) string {
	return strings.Replace(nm, tokens.QNameDelimiter, string(os.PathSeparator), -1)
}

// workspacePath converts a name into the relevant name-part in the workspace to look for that dependency.
func workspacePath(w *workspace, nm tokens.PackageName) string {
	if ns := w.Settings().Namespace; ns != "" {
		// If the name starts with the namespace, trim the name part.
		orig := string(nm)
		if trim := strings.TrimPrefix(orig, ns+tokens.QNameDelimiter); trim != orig {
			return stringNamePath(trim)
		}
	}
	return packageNamePath(nm)
}

// Save persists any in-memory changes made to the workspace.
func (w *workspace) Save() error {
	// For now, the only changes to commit are the settings file changes.
	return w.saveSettings()
}

// settingsFile returns the settings file location for this workspace.
func (w *workspace) settingsFile(ext string) string {
	return filepath.Join(w.root, Dir, SettingsFile+ext)
}

// readSettings loads a settings file from the workspace, probing for all available extensions.
func (w *workspace) readSettings() (Settings, error) {
	// Attempt to load the raw bytes from all available extensions.
	var settings Settings
	for _, ext := range encoding.Exts {
		// See if the file exists.
		path := w.settingsFile(ext)
		b, err := ioutil.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue // try the next extension
			}
			return settings, err
		}

		// If it does, go ahead and decode it.
		m := encoding.Marshalers[ext]
		if err := m.Unmarshal(b, &settings); err != nil {
			return settings, err
		}
	}
	return settings, nil
}

// saveSettings saves the settings into a file for this workspace, committing any in-memory changes that have been made.
// IDEA: right now, we only support JSON.  It'd be ideal if we supported YAML too (and it would be quite easy).
func (w *workspace) saveSettings() error {
	m := encoding.Default()
	settings := w.Settings()
	b, err := m.Marshal(settings)
	if err != nil {
		return err
	}
	path := w.settingsFile(encoding.DefaultExt())
	return ioutil.WriteFile(path, b, 0644)
}
