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
	"github.com/marapongo/mu/pkg/options"
	"github.com/marapongo/mu/pkg/symbols"
	"github.com/marapongo/mu/pkg/util/contract"
)

// W offers functionality for interacting with Mu workspaces.  A workspace influences Mu compilation; for example, it
// can specify default versions of dependencies, easing the process of working with multiple projects.
type W interface {
	// Root returns the base path of the current workspace.
	Root() string
	// Options represents the current options governing the compilation.
	Options() *options.Options
	// Settings returns a mutable pointer to the optional workspace settings info.
	Settings() *ast.Workspace

	// ReadSettings reads in the settings file and returns it, returning nil if there is none.
	ReadSettings() (*diag.Document, error)

	// DetectMufile locates the closest Mufile from the given path, searching "upwards" in the directory hierarchy.
	DetectMufile() (string, error)
	// DepCandidates fetches all candidate locations for resolving a dependency name to its installed artifacts.
	DepCandidates(dep symbols.RefParts) []string
}

// New creates a new workspace from the given starting path.
func New(options *options.Options) (W, error) {
	contract.Requiref(options != nil, "options", "!= nil")

	// First normalize the path to an absolute one.
	path, err := filepath.Abs(options.Pwd)
	if err != nil {
		return nil, err
	}

	home, err := homedir.Dir()
	if err != nil {
		return nil, err
	}

	ws := workspace{
		path:    path,
		home:    home,
		options: options,
	}

	// Memoize the root directory before returning.
	if _, err := ws.initRootInfo(); err != nil {
		return nil, err
	}

	return &ws, nil
}

type workspace struct {
	path     string           // the path at which the workspace was constructed.
	home     string           // the home directory to use for this workspace.
	root     string           // the root of the workspace.
	muspace  string           // a path to the Muspace file, if any.
	options  *options.Options // the options governing the current compilation.
	settings ast.Workspace    // an optional bag of workspace-wide settings.
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
				if IsMuspace(muspace, w.options.Diag) {
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

func (w *workspace) Root() string              { return w.root }
func (w *workspace) Options() *options.Options { return w.options }
func (w *workspace) Settings() *ast.Workspace  { return &w.settings }

func (w *workspace) ReadSettings() (*diag.Document, error) {
	if w.muspace == "" {
		return nil, nil
	}

	// If there is a workspace settings file in here, load it up before returning.
	return diag.ReadDocument(w.muspace)
}

func (w *workspace) DetectMufile() (string, error) {
	return DetectMufile(w.path, w.options.Diag)
}

func (w *workspace) DepCandidates(dep symbols.RefParts) []string {
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
	// A workspace may optionally have a namespace, in which case, we will also look for stacks in the workspace whose
	// name is simplified to omit that namespace part.  For example, if a stack is named `mu/project/stack`, and the
	// workspace namespace is `mu/`, then we will search `w/project/stack`; if the workspace is `mu/project/`, then we
	// will search `w/stack`; and so on.  This helps to avoid needing to deeply nest workspaces needlessly.
	//
	// The following code simply produces an array of these candidate locations, in order.

	base := stringNamePath(dep.Base)
	name := namePath(dep.Name)
	wsname := workspacePath(w, dep.Name)

	// For each extension we support, add the same set of search locations.
	cands := make([]string, 0, 4*len(encoding.Exts))
	for _, ext := range encoding.Exts {
		cands = append(cands, filepath.Join(w.root, base, name, Mufile+ext))
		cands = append(cands, filepath.Join(w.root, wsname, Mufile+ext))
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
func namePath(nm symbols.Name) string {
	return stringNamePath(string(nm))
}

// stringNamePart cleans a string component of a name and makes sure it's appropriate to use as a path.
func stringNamePath(nm string) string {
	return strings.Replace(nm, symbols.NameDelimiter, string(os.PathSeparator), -1)
}

// workspacePath converts a name into the relevant name-part in the workspace to look for that dependency.
func workspacePath(w *workspace, nm symbols.Name) string {
	if ns := w.Settings().Namespace; ns != "" {
		// If the name starts with the namespace, trim the name part.
		orig := string(nm)
		if trim := strings.TrimPrefix(orig, ns+symbols.NameDelimiter); trim != orig {
			return stringNamePath(trim)
		}
	}
	return namePath(nm)
}
