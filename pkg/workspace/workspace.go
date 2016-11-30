// Copyright 2016 Marapongo, Inc. All rights reserved.

package workspace

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/glog"
	homedir "github.com/mitchellh/go-homedir"

	"github.com/marapongo/mu/pkg/ast"
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/encoding"
)

// W offers functionality for interacting with Mu workspaces.
type W interface {
	// Root returns the base path of the current workspace.
	Root() string
	// Settings returns a mutable pointer to the optional workspace settings info.
	Settings() *ast.Workspace
	// ReadSettings reads in the settings file and returns it, returning nil if there is none.
	ReadSettings() (*diag.Document, error)

	// DetectMufile locates the closest Mufile from the given path, searching "upwards" in the directory hierarchy.
	DetectMufile() (string, error)
	// DepCandidates fetches all candidate locations for resolving a dependency name to its installed artifacts.
	DepCandidates(dep ast.RefParts) []string
}

// New creates a new workspace from the given starting path.
func New(path string, d diag.Sink) (W, error) {
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
		path: path,
		home: home,
		d:    d,
	}

	// Memoize the root directory before returning.
	if _, err := ws.initRootInfo(); err != nil {
		return nil, err
	}

	return &ws, nil
}

type workspace struct {
	path     string        // the path at which the workspace was constructed.
	home     string        // the home directory to use for this workspace.
	root     string        // the root of the workspace.
	muspace  string        // a path to the Muspace file, if any.
	settings ast.Workspace // an optional bag of workspace-wide settings.
	d        diag.Sink     // a diagnostics sink to use for workspace operations.
}

// initRootInfo finds the root of the workspace, caches it for fast lookups, and loads up any workspace settings.
func (w *workspace) initRootInfo() (string, error) {
	if w.root == "" {
		// Detect the root of the workspace and cache it.
		root := pathDir(w.path)
	Search:
		for {
			files, err := ioutil.ReadDir(root)
			if err != nil {
				return "", err
			}
			for _, file := range files {
				// A muspace file delimits the root of the workspace.
				muspace := filepath.Join(root, file.Name())
				if IsMuspace(muspace, w.d) {
					glog.V(3).Infof("Mu workspace detected; setting root to %v", w.root)
					w.root = root
					w.muspace = muspace
					break Search
				}
			}

			// If neither succeeded, keep looking in our parent directory.
			root = filepath.Dir(root)
			if isTop(root) {
				// We reached the top of the filesystem.  Just set root back to the path and stop.
				glog.V(3).Infof("No Mu workspace found; defaulting to current path %v", w.root)
				w.root = w.path
				break
			}
		}
	}

	return w.root, nil
}

func (w *workspace) Root() string {
	return w.root
}

func (w *workspace) Settings() *ast.Workspace {
	return &w.settings
}

func (w *workspace) ReadSettings() (*diag.Document, error) {
	if w.muspace == "" {
		return nil, nil
	}

	// If there is a workspace settings file in here, load it up before returning.
	return diag.ReadDocument(w.muspace)
}

func (w *workspace) DetectMufile() (string, error) {
	return DetectMufile(w.path, w.d)
}

func (w *workspace) DepCandidates(dep ast.RefParts) []string {
	// The search order for dependencies is specified in https://github.com/marapongo/mu/blob/master/docs/deps.md.
	//
	// Roughly speaking, these locations are are searched, in order:
	//
	// 		1. The current Workspace, for intra-Workspace but inter-Stack dependencies.
	// 		2. The current Workspace's .mu/stacks/ directory.
	// 		3. The global Workspace's .mu/stacks/ directory.
	// 		4. The Mu installation location's $MUROOT/lib/ directory (default /usr/local/mu/lib).
	//
	// In each location, we prefer a fully qualified hit if it exists -- containing both the base of the reference plus
	// the name -- however, we also accept name-only hits.  This allows developers to organize their workspace without
	// worrying about where their Mu Stacks are hosted.  Most of the Mu tools, however, prefer fully qualified paths.
	//
	// To be more precise, given a StackRef r and a workspace root w, we look in these locations, in order:
	//
	//		1. w/base(r)/name(r)
	//		2. w/name(r)
	//		3. w/.Mudeps/base(r)/name(r)
	//		4. w/.Mudeps/name(r)
	//		5. ~/.Mudeps/base(r)/name(r)
	//		6. ~/.Mudeps/name(r)
	//		7. $MUROOT/lib/base(r)/name(r)
	//		8. $MUROOT/lib/name(r)
	//
	// The following code simply produces an array of these candidate locations, in order.

	base := stringNamePath(dep.Base)
	name := namePath(dep.Name)

	// For each extension we support, add the same set of search locations.
	cands := make([]string, 0, 4*len(encoding.Exts))
	for _, ext := range encoding.Exts {
		cands = append(cands, filepath.Join(w.root, base, name, Mufile+ext))
		cands = append(cands, filepath.Join(w.root, name, Mufile+ext))
		cands = append(cands, filepath.Join(w.root, Mudeps, base, name, Mufile+ext))
		cands = append(cands, filepath.Join(w.root, Mudeps, name, Mufile+ext))
		cands = append(cands, filepath.Join(w.home, Mudeps, base, name, Mufile+ext))
		cands = append(cands, filepath.Join(w.home, Mudeps, name, Mufile+ext))
		cands = append(cands, filepath.Join(InstallRoot(), InstallRootLibdir, base, name, Mufile+ext))
		cands = append(cands, filepath.Join(InstallRoot(), InstallRootLibdir, name, Mufile+ext))
	}
	return cands
}

// namePath just cleans a name and makes sure it's appropriate to use as a path.
func namePath(nm ast.Name) string {
	return stringNamePath(string(nm))
}

// stringNamePart cleans a string component of a name and makes sure it's appropriate to use as a path.
func stringNamePath(nm string) string {
	return strings.Replace(nm, ast.NameDelimiter, string(os.PathSeparator), -1)
}
