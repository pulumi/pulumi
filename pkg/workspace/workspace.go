// Copyright 2016 Pulumi, Inc. All rights reserved.

package workspace

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/glog"
	homedir "github.com/mitchellh/go-homedir"

	"github.com/pulumi/coconut/pkg/compiler/core"
	"github.com/pulumi/coconut/pkg/diag"
	"github.com/pulumi/coconut/pkg/encoding"
	"github.com/pulumi/coconut/pkg/pack"
	"github.com/pulumi/coconut/pkg/tokens"
	"github.com/pulumi/coconut/pkg/util/contract"
)

// W offers functionality for interacting with Coconut workspaces.  A workspace influences compilation; for example, it
// can specify default versions of dependencies, easing the process of working with multiple projects.
type W interface {
	// Root returns the base path of the current workspace.
	Root() string
	// Settings returns a mutable pointer to the optional workspace settings info.
	Settings() *Workspace

	// ReadSettings reads in the settings file and returns it, returning nil if there is none.
	ReadSettings() (*diag.Document, error)

	// DetectPackage locates the closest Nutfile from the given path, searching "upwards" in the directory hierarchy.
	DetectPackage() (string, error)
	// DepCandidates fetches all candidate locations for resolving a dependency name to its installed artifacts.
	DepCandidates(dep pack.PackageURL) []string
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

	// Memoize the root directory before returning.
	if _, err := ws.initRootInfo(); err != nil {
		return nil, err
	}

	return &ws, nil
}

type workspace struct {
	ctx      *core.Context // the shared compiler context object.
	path     string        // the path at which the workspace was constructed.
	home     string        // the home directory to use for this workspace.
	root     string        // the root of the workspace.
	muspace  string        // a path to the Nutspace file, if any.
	settings Workspace     // an optional bag of workspace-wide settings.
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
				if IsNutspace(muspace, w.ctx.Diag) {
					glog.V(3).Infof("Coconut workspace detected; setting root to %v", w.root)
					w.root = root
					w.muspace = muspace
					break Search
				}
			}

			// If neither succeeded, keep looking in our parent directory.
			root = filepath.Dir(root)
			if isTop(root) {
				// We reached the top of the filesystem.  Just set root back to the path and stop.
				glog.V(3).Infof("No Coconut workspace found; defaulting to current path %v", w.root)
				w.root = w.path
				break
			}
		}
	}

	return w.root, nil
}

func (w *workspace) Root() string         { return w.root }
func (w *workspace) Settings() *Workspace { return &w.settings }

func (w *workspace) ReadSettings() (*diag.Document, error) {
	if w.muspace == "" {
		return nil, nil
	}

	// If there is a workspace settings file in here, load it up before returning.
	return diag.ReadDocument(w.muspace)
}

func (w *workspace) DetectPackage() (string, error) {
	return DetectPackage(w.path, w.ctx.Diag)
}

func (w *workspace) DepCandidates(dep pack.PackageURL) []string {
	// The search order for dependencies is specified in https://github.com/pulumi/coconut/blob/master/docs/deps.md.
	//
	// Roughly speaking, these locations are are searched, in order:
	//
	// 		1. The current Workspace, for intra-Workspace but inter-Nut dependencies.
	// 		2. The current Workspace's .mu/stacks/ directory.
	// 		3. The global Workspace's .mu/stacks/ directory.
	// 		4. The Coconut installation location's $COCOROOT/lib/ directory (default /usr/local/coconut/lib).
	//
	// In each location, we prefer a fully qualified hit if it exists -- containing both the base of the reference plus
	// the name -- however, we also accept name-only hits.  This allows developers to organize their workspace without
	// worrying about where their Nuts are hosted.  Most of the Coconut tools, however, prefer fully qualified paths.
	//
	// To be more precise, given a NutRef r and a workspace root w, we look in these locations, in order:
	//
	//		1. w/base(r)/name(r)
	//		2. w/name(r)
	//		3. w/.Nuts/base(r)/name(r)
	//		4. w/.Nuts/name(r)
	//		5. ~/.Nuts/base(r)/name(r)
	//		6. ~/.Nuts/name(r)
	//		7. $COCOROOT/lib/base(r)/name(r)
	//		8. $COCOROOT/lib/name(r)
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
		cands = append(cands, filepath.Join(w.root, base, name, Nutpack+ext))
		cands = append(cands, filepath.Join(w.root, wsname, Nutpack+ext))
		cands = append(cands, filepath.Join(w.root, Nutdeps, base, name, Nutpack+ext))
		cands = append(cands, filepath.Join(w.root, Nutdeps, name, Nutpack+ext))
		cands = append(cands, filepath.Join(w.home, Nutdeps, base, name, Nutpack+ext))
		cands = append(cands, filepath.Join(w.home, Nutdeps, name, Nutpack+ext))
		cands = append(cands, filepath.Join(InstallRoot(), InstallRootLibdir, base, name, Nutpack+ext))
		cands = append(cands, filepath.Join(InstallRoot(), InstallRootLibdir, name, Nutpack+ext))
	}
	return cands
}

// namePath just cleans a name and makes sure it's appropriate to use as a path.
func namePath(nm tokens.Name) string {
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
