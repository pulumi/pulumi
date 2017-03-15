// Copyright 2017 Pulumi, Inc. All rights reserved.

package binder

import (
	"github.com/golang/glog"

	"github.com/pulumi/coconut/pkg/compiler/ast"
	"github.com/pulumi/coconut/pkg/compiler/errors"
	"github.com/pulumi/coconut/pkg/compiler/symbols"
	"github.com/pulumi/coconut/pkg/diag"
	"github.com/pulumi/coconut/pkg/pack"
	"github.com/pulumi/coconut/pkg/tokens"
	"github.com/pulumi/coconut/pkg/util/contract"
	"github.com/pulumi/coconut/pkg/workspace"
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

	// Now bind all of the package's declarations (if any).  This pass does not yet actually bind definitions.
	b.bindPackageDeclarations(pkgsym)

	// Now resolve all export entries to symbols (even though definitions aren't complete yet).
	b.bindPackageExports(pkgsym)

	// Next go ahead and bind definitions.  This happens in a full pass after resolving all modules, because this
	// phase might reference other declarations that are only now exposed after the above phase.
	b.bindPackageDefinitions(pkgsym)

	// Finally, bind all of the package's method bodies.  This second pass is required to ensure that inter-module
	// dependencies can resolve to symbols, after reaching the symbol-level fixed point above.
	b.bindPackageBodies(pkgsym)

	return respkg
}

// resolvePackageDeps resolves all package dependencies, ensuring that all symbols are available to us.  This recurses
// into dependency dependencies, and so on, until we have reached a fixed point.
func (b *binder) resolvePackageDeps(pkg *symbols.Package) {
	contract.Require(pkg != nil, "pkg")

	if pkg.Node.Dependencies != nil {
		for _, dep := range pack.StableDependencies(*pkg.Node.Dependencies) {
			depstr := (*pkg.Node.Dependencies)[dep]
			// The dependency is a URL.  Transform it into a name used for symbol resolution.
			depurl, err := depstr.Parse(dep)
			if err != nil {
				b.Diag().Errorf(errors.ErrorMalformedPackageURL.At(pkg.Node), depstr, err)
			} else {
				glog.V(3).Infof("Resolving package '%v' dependency name=%v, url=%v",
					pkg.Name(), depurl.Name, depurl.URL())
				if depsym := b.resolveDep(depurl); depsym != nil {
					// Store the symbol and URL with all its empty parts resolved (a.k.a. canonicalized) in the map.
					pkg.Dependencies[depurl.Name] = depsym
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
	locs := b.w.DepCandidates(dep)
	for _, loc := range locs {
		// See if this candidate actually exists.
		isCocopack := workspace.IsCocopack(loc, b.Diag())
		glog.V(5).Infof("Probing for dependency '%v' at '%v': isCocopack=%v", dep, loc, isCocopack)

		// If it does, go ahead and read it in, and bind it (recursively).
		if isCocopack {
			// Read in the package AST.
			doc, err := diag.ReadDocument(loc)
			if err != nil {
				b.Diag().Errorf(errors.ErrorCouldNotReadCocofile.AtFile(loc), err)
				return nil
			}
			pkg := b.reader.ReadPackage(doc)

			// Now perform the binding and return it.
			return b.resolveBindPackage(pkg, dep)
		}
	}

	// If we got to this spot, we could not find the dependency.  Issue an error and bail out.  Note that we tack on all
	// searched paths to help developers diagnose pathing problems that might cause dependency load failures.
	searched := ""
	for _, loc := range locs {
		if searched == "" {
			searched += "\n\tsearched paths: "
		} else {
			searched += "\n\t                "
		}
		searched += loc
	}
	b.Diag().Errorf(errors.ErrorImportNotFound, dep, searched)
	return nil
}

// bindPackageDeclarations recursively binds all named entities and stores them in the given package.  This doesn't yet
// bind definitions, because those require the top-level declarations for all modules and members to be available first.
func (b *binder) bindPackageDeclarations(pkg *symbols.Package) {
	contract.Require(pkg != nil, "pkg")

	// Bind all module names.
	if pkg.Node.Modules != nil {
		modules := *pkg.Node.Modules
		for _, modtok := range ast.StableModules(modules) {
			pkg.Modules[modtok] = b.bindModuleDeclarations(modules[modtok], pkg)
		}
	}

	// Now see if there are aliased module names; if yes, bind to the above symbols.
	if pkg.Node.Aliases != nil {
		aliases := *pkg.Node.Aliases
		for _, from := range pack.StableModuleAliases(aliases) {
			to := aliases[from]
			if target, has := pkg.Modules[to]; has {
				pkg.Modules[from] = target
			} else {
				b.Diag().Errorf(errors.ErrorModuleAliasTargetNotFound, to, from)
			}
		}
	}
}

func isModuleAlias(pkg *symbols.Package, mod tokens.ModuleName) bool {
	if pkg.Node.Aliases == nil {
		return false
	}
	_, isalias := (*pkg.Node.Aliases)[mod]
	return isalias
}

// bindPackageExports resolves exports to their referent symbols, one level deep.  During this phase, exports may depend
// upon other exports, and so until it has settled, we can't properly chase down export links.
func (b *binder) bindPackageExports(pkg *symbols.Package) {
	contract.Require(pkg != nil, "pkg")
	for _, mod := range symbols.StableModuleMap(pkg.Modules) {
		if !isModuleAlias(pkg, mod) {
			b.bindModuleExports(pkg.Modules[mod])
		}
	}
}

// bindPackageDefinitions binds all definitions within a package (classes, signatures, varaibles, etc), but doesn't
// actually bind any function bodies yet.  The function bodies may depend upon information that depends upon information
// that isn't fully computed until after the definition pass has been completed.
func (b *binder) bindPackageDefinitions(pkg *symbols.Package) {
	contract.Require(pkg != nil, "pkg")
	for _, mod := range symbols.StableModuleMap(pkg.Modules) {
		if !isModuleAlias(pkg, mod) {
			b.bindModuleDefinitions(pkg.Modules[mod])
		}
	}
}

// bindPackageBodies binds all method bodies, in a distinct pass, after binding all symbol-level information.
func (b *binder) bindPackageBodies(pkg *symbols.Package) {
	contract.Require(pkg != nil, "pkg")
	for _, mod := range symbols.StableModuleMap(pkg.Modules) {
		if !isModuleAlias(pkg, mod) {
			b.bindModuleBodies(pkg.Modules[mod])
		}
	}
}
