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
	// Use the shared binding routine; but just use a default URL since we didn't fetch this as a dependency.
	// TODO: detect the version from the workspace.
	respkg := b.resolveBindPackage(pkg, pack.PackageURL{Name: pkg.Name})
	return respkg.Pkg
}

func (b *binder) resolveBindPackage(pkg *pack.Package, pkgurl pack.PackageURL) *symbols.ResolvedPackage {
	// If the package name isn't a legal identifier, say so.
	if !tokens.IsQName(string(pkg.Name)) {
		b.Diag().Errorf(errors.ErrorInvalidPackageName.At(pkg.Doc))
	}

	// TODO: read the package's version and ensure that it is not a range.  In other words, resolved packages must have
	//     concrete versions, either semantic (e.g., "1.3.9-beta2") or SHA hashes.  See errors.ErrorIllegalStackVersion.

	// Create a symbol with empty dependencies and modules; this allows child symbols to parent to it.
	pkgsym := symbols.NewPackageSym(pkg)

	// Enter this package into the table so that it can be resolved cyclically (including self-dependencies).  Note that
	// we canonicalize the URL so that we can depend on its fully bound state in subsequent lookups.
	respkg := &symbols.ResolvedPackage{
		Pkg: pkgsym,
		URL: pkgurl.Defaults(),
	}
	b.ctx.Pkgs[pkg.Name] = respkg
	glog.V(5).Infof("Registered resolved package symbol: pkg=%v url=%v", pkgsym, pkgurl)

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

	return respkg
}

// resolvePackageDeps resolves all package dependencies, ensuring that all symbols are available to us.  This recurses
// into dependency dependencies, and so on, until we have reached a fixed point.
func (b *binder) resolvePackageDeps(pkg *symbols.Package) {
	contract.Require(pkg != nil, "pkg")

	if pkg.Node.Dependencies != nil {
		for _, dep := range pack.StableDependencies(*pkg.Node.Dependencies) {
			depurl := (*pkg.Node.Dependencies)[dep]
			// The dependency is a URL.  Transform it into a name used for symbol resolution.
			dep, err := depurl.Parse()
			if err != nil {
				b.Diag().Errorf(errors.ErrorMalformedPackageURL.At(pkg.Node), depurl, err)
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

// resolveDep actually performs the package resolution process, populating the compiler symbol tables.
func (b *binder) resolveDep(dep pack.PackageURL) *symbols.ResolvedPackage {
	// First, see if we've already loaded this package.  If yes, reuse it.
	if pkgsym, exists := b.ctx.Pkgs[dep.Name]; exists {
		// TODO: ensure versions match.
		return pkgsym
	}

	// There are many places a dependency could come from.  Consult the workspace for a list of those paths.  It will
	// return a number of them, in preferred order, and we simply probe each one until we find something.
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

			// Now perform the binding and return it.
			return b.resolveBindPackage(pkg, dep)
		}
	}

	// If we got to this spot, we could not find the dependency.  Issue an error and bail out.
	b.Diag().Errorf(errors.ErrorImportNotFound, dep)
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
		b.bindModuleMethodBodies(module)
	}
}
