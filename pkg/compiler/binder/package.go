// Copyright 2016 Marapongo, Inc. All rights reserved.

package binder

import (
	"github.com/golang/glog"

	"github.com/marapongo/mu/pkg/compiler/errors"
	"github.com/marapongo/mu/pkg/compiler/symbols"
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/pack"
	"github.com/marapongo/mu/pkg/tokens"
	"github.com/marapongo/mu/pkg/util/contract"
	"github.com/marapongo/mu/pkg/workspace"
)

// BindPackages takes a package AST, resolves all dependencies and tokens inside of it, and returns a fully bound
// package symbol that can be used for semantic operations (like interpretation and evaluation).
func (b *binder) BindPackage(pkg *pack.Package) *symbols.Package {
	b.resolvePackageDeps(pkg)
	// TODO: create a package symbol.
	return nil
}

// resolvePackageDeps resolves all package dependencies, ensuring that all symbols are available to us.  This recurses
// into dependency dependencies, and so on, until we have reached a fixed point.
func (b *binder) resolvePackageDeps(pkg *pack.Package) {
	contract.Require(pkg != nil, "pkg")
	if pkg.Dependencies != nil {
		for _, dep := range *pkg.Dependencies {
			glog.V(3).Infof("Resolving package %v dependency %v", pkg.Name, dep)
			b.resolveDep(dep)
		}
	}
}

var cyclicTombstone = &symbols.Package{}

// resolveDep actually performs the package resolution process, populating the compiler symbol tables.
func (b *binder) resolveDep(dep tokens.Package) *symbols.Package {
	// First, see if we've already loaded this package.  If yes, reuse it.
	// TODO: ensure versions match.
	if pkg, exists := b.ctx.Pkgs[dep]; exists {
		// Check for cycles.  If one exists, do not process this dependency any further.
		if pkg == cyclicTombstone {
			// TODO: report the full transitive loop to help debug cycles.
			b.Diag().Errorf(errors.ErrorImportCycle, dep)
		}
		return pkg
	}

	// There are many places a dependency could come from.  Consult the workspace for a list of those paths.  It will
	// return a number of them, in preferred order, and we simply probe each one until we find something.
	ref := tokens.Ref(dep).MustParse()
	for _, loc := range b.w.DepCandidates(ref) {
		// See if this candidate actually exists.
		isMufile := workspace.IsMufile(loc, b.Diag())
		glog.V(5).Infof("Probing for dependency %v at %v: %v", dep, loc, isMufile)

		// If it does, go ahead and read it in, and bind it (recursively).
		if isMufile {
			// Read in the package AST.
			doc, err := diag.ReadDocument(loc)
			if err != nil {
				b.Diag().Errorf(errors.ErrorCouldNotReadMufile.AtFile(loc), err)
				return nil
			}
			pkg := b.reader.ReadPackage(doc)

			// Inject a tombstone so we can easily detect cycles.
			b.ctx.Pkgs[dep] = cyclicTombstone

			// Now perform the binding.
			pkgsym := b.BindPackage(pkg)

			// Memoize this in the compiler's cache and return it.
			b.ctx.Pkgs[dep] = pkgsym
			return pkgsym
		}
	}

	// If we got to this spot, we could not find the dependency.  Issue an error and bail out.
	b.Diag().Errorf(errors.ErrorPackageNotFound, ref)
	return nil
}
