// Copyright 2016 Marapongo, Inc. All rights reserved.

package binder

import (
	"github.com/golang/glog"

	"github.com/marapongo/mu/pkg/compiler/errors"
	"github.com/marapongo/mu/pkg/compiler/symbols"
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/pack"
	"github.com/marapongo/mu/pkg/util/contract"
	"github.com/marapongo/mu/pkg/workspace"
)

// BindPackages takes a package AST, resolves all dependencies and tokens inside of it, and returns a fully bound
// package symbol that can be used for semantic operations (like interpretation and evaluation).
func (b *binder) BindPackage(pkg *pack.Package) *symbols.Package {
	// If the package is missing a name, issue an error.
	if pkg.Name == "" {
		b.Diag().Errorf(errors.ErrorMissingPackageName.At(pkg))
	}

	// TODO: read the package's version and ensure that it is not a range.  In other words, resolved packages must have
	//     concrete versions, either semantic (e.g., "1.3.9-beta2") or SHA hashes.  See errors.ErrorIllegalStackVersion.

	// Create a symbol with empty dependencies and modules; this allows child symbols to parent to it.
	pkgsym := symbols.NewPackageSym(pkg)

	// Set the current package in the context so we can e.g. enforce accessibility.
	priorpkg := b.ctx.Currpkg
	b.ctx.Currpkg = pkgsym
	defer func() { b.ctx.Currpkg = priorpkg }()

	// Resolve all package dependencies.
	b.resolvePackageDeps(pkgsym)

	// Now bind all of the package's modules (if any).  This pass does not yet actually bind bodies.
	b.bindPackageModules(pkgsym)

	// Finally, bind all of the package's method bodies.  This second pass is required to ensure that inter-module
	// dependencies can resolve to symbols, after reaching the symbol-level fixed point above.
	b.bindPackageMethodBodies(pkgsym)

	return pkgsym
}

// resolvePackageDeps resolves all package dependencies, ensuring that all symbols are available to us.  This recurses
// into dependency dependencies, and so on, until we have reached a fixed point.
func (b *binder) resolvePackageDeps(pkg *symbols.Package) {
	contract.Require(pkg != nil, "pkg")

	if pkg.Node.Dependencies != nil {
		for _, depurl := range *pkg.Node.Dependencies {
			// The dependency is a URL.  Transform it into a name used for symbol resolution.
			dep, err := depurl.Parse()
			if err != nil {
				b.Diag().Errorf(errors.ErrorPackageURLMalformed, depurl, err)
			} else {
				glog.V(3).Infof("Resolving package '%v' dependency name=%v, url=%v", pkg.Name(), dep.Name, dep.URL())
				if depsym := b.resolveDep(dep); depsym != nil {
					// Store the symbol and URL with all its empty parts resolved (a.k.a. canonicalized) in the map.
					pkg.Dependencies[dep.Name] = depsym
				}
			}
		}
	}
}

var cyclicTombstone = &symbols.ResolvedPackage{Pkg: symbols.NewPackageSym(nil)}

// resolveDep actually performs the package resolution process, populating the compiler symbol tables.
func (b *binder) resolveDep(dep pack.PackageURL) *symbols.ResolvedPackage {
	// First, see if we've already loaded this package.  If yes, reuse it.
	if pkgsym, exists := b.ctx.Pkgs[dep.Name]; exists {
		// Check for cycles.  If one exists, do not process this dependency any further.
		if pkgsym == cyclicTombstone {
			// TODO: report the full transitive loop to help debug cycles.
			b.Diag().Errorf(errors.ErrorImportCycle, dep.Name)
			return nil
		}

		// TODO: ensure versions match.
		return pkgsym
	}

	// There are many places a dependency could come from.  Consult the workspace for a list of those paths.  It will
	// return a number of them, in preferred order, and we simply probe each one until we find something.
	dep = dep.Defaults() // use defaults for missing parts
	for _, loc := range b.w.DepCandidates(dep) {
		// See if this candidate actually exists.
		isMufile := workspace.IsMufile(loc, b.Diag())
		glog.V(5).Infof("Probing for dependency '%v' at '%v': isMufile=%v", dep, loc, isMufile)

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
			b.ctx.Pkgs[dep.Name] = cyclicTombstone

			// Now perform the binding.
			pkgsym := b.BindPackage(pkg)

			// Erase the tombstone, memoize the symbol in the compiler's cache, and return it.  Note that we have
			// canonicalized the URL by substituting defaults for missing parts, to ease future binding endeavors.
			respkg := &symbols.ResolvedPackage{
				Pkg: pkgsym,
				URL: dep,
			}
			b.ctx.Pkgs[dep.Name] = respkg
			return respkg
		}
	}

	// If we got to this spot, we could not find the dependency.  Issue an error and bail out.
	b.Diag().Errorf(errors.ErrorPackageNotFound, dep)
	return nil
}

// bindPackageModules recursively binds all modules and stores them in the given package.
func (b *binder) bindPackageModules(pkg *symbols.Package) {
	contract.Require(pkg != nil, "pkg")
	if pkg.Node.Modules != nil {
		for modtok, mod := range *pkg.Node.Modules {
			pkg.Modules[modtok] = b.bindModule(mod, pkg)
		}
	}
}

// bindPackageMethodBodies binds all method bodies, in a second pass, after binding all symbol-level information.
func (b *binder) bindPackageMethodBodies(pkg *symbols.Package) {
	contract.Require(pkg != nil, "pkg")
	for _, module := range pkg.Modules {
		b.bindModuleBodies(module)
	}
}
