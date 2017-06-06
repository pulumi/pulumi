// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
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
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/glog"
	homedir "github.com/mitchellh/go-homedir"

	"github.com/pulumi/lumi/pkg/compiler/core"
	"github.com/pulumi/lumi/pkg/encoding"
	"github.com/pulumi/lumi/pkg/pack"
	"github.com/pulumi/lumi/pkg/tokens"
	"github.com/pulumi/lumi/pkg/util/contract"
)

// W offers functionality for interacting with Lumi workspaces.  A workspace influences compilation; for example, it
// can specify default versions of dependencies, easing the process of working with multiple projects.
type W interface {
	Path() string        // the base path of the current workspace.
	Root() string        // the root path of the current workspace.
	Settings() *Settings // returns a mutable pointer to the optional workspace settings info.

	DetectPackage() (string, error)             // locates the nearest project file in the directory hierarchy.
	DepCandidates(dep pack.PackageURL) []string // fetches all candidate locations for a dependency's artifacts.
	Save() error                                // saves any modifications to the workspace.
}

// New creates a new workspace from the given starting path.
func New(ctx *core.Context) (W, error) {
	contract.Requiref(ctx != nil, "ctx", "!= nil")

	// First normalize the path to an absolute one.
	path, err := filepath.Abs(ctx.Path)
	if err != nil {
		return nil, err
	}

	home, err := homedir.Dir()
	if err != nil {
		return nil, err
	}

	ws := workspace{
		ctx:  ctx,
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
	ctx      *core.Context // the shared compiler context object.
	path     string        // the path at which the workspace was constructed.
	home     string        // the home directory to use for this workspace.
	root     string        // the root of the workspace.
	settings Settings      // an optional bag of workspace-wide settings.
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
	return DetectPackage(w.path, w.ctx.Diag)
}

func (w *workspace) DepCandidates(dep pack.PackageURL) []string {
	// The search order for dependencies is specified in https://github.com/pulumi/lumi/blob/master/docs/deps.md.
	//
	// Roughly speaking, these locations are are searched, in order:
	//
	// 		1. The current workspace, for intra-workspace but inter-package dependencies.
	// 		2. The current workspace's .lumi/packs/ directory.
	// 		3. The global workspace's .lumi/packs/ directory.
	// 		4. The Lumi installation location's $LUMIROOT/lib/ directory (default /usr/local/lumi/lib).
	//
	// In each location, we prefer a fully qualified hit if it exists -- containing both the base of the reference plus
	// the name -- however, we also accept name-only hits.  This allows developers to organize their workspace without
	// worrying about where packages are hosted.  Most of the Lumi tools, however, prefer fully qualified paths.
	//
	// To be more precise, given a PackageRef r and a workspace root w, we look in these locations, in order:
	//
	//		1. w/base(r)/name(r)
	//		2. w/name(r)
	//		3. w/.lumi/packs/base(r)/name(r)
	//		4. w/.lumi/packs/name(r)
	//		5. ~/.lumi/packs/base(r)/name(r)
	//		6. ~/.lumi/packs/name(r)
	//		7. $LUMIROOT/lib/base(r)/name(r)
	//		8. $LUMIROOT/lib/name(r)
	//
	// A workspace may optionally have a namespace, in which case, we will also look for stacks in the workspace whose
	// name is simplified to omit that namespace part.  For example, if a stack is named `mu/project/stack`, and the
	// workspace namespace is `mu/`, then we will search `w/project/stack`; if the workspace is `mu/project/`, then we
	// will search `w/stack`; and so on.  This helps to avoid needing to deeply nest workspaces needlessly.
	//
	// The following code simply produces an array of these candidate locations, in order.

	dep = dep.Defaults() // ensure we use defaults in the pathing.
	base := stringNamePath(dep.Base)
	name := packageNamePath(dep.Name)
	wsname := workspacePath(w, dep.Name)

	// For each extension we support, add the same set of search locations.
	cands := make([]string, 0, 4*len(encoding.Exts))
	for _, ext := range encoding.Exts {
		cands = append(cands, filepath.Join(w.root, base, name, PackFile+ext))
		cands = append(cands, filepath.Join(w.root, wsname, PackFile+ext))
		cands = append(cands, filepath.Join(w.root, Dir, DepDir, base, name, PackFile+ext))
		cands = append(cands, filepath.Join(w.root, Dir, DepDir, name, PackFile+ext))
		cands = append(cands, filepath.Join(w.home, Dir, DepDir, base, name, PackFile+ext))
		cands = append(cands, filepath.Join(w.home, Dir, DepDir, name, PackFile+ext))
		cands = append(cands, filepath.Join(InstallRoot(), InstallRootLibdir, base, name, PackFile+ext))
		cands = append(cands, filepath.Join(InstallRoot(), InstallRootLibdir, name, PackFile+ext))
	}
	return cands
}

// namePath just cleans a name and makes sure it's appropriate to use as a path.
func namePath(nm tokens.Name) string {
	return stringNamePath(string(nm))
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
