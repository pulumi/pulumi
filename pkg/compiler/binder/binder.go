// Copyright 2016 Marapongo, Inc. All rights reserved.

package binder

import (
	"github.com/golang/glog"

	"github.com/marapongo/mu/pkg/compiler/ast"
	"github.com/marapongo/mu/pkg/compiler/core"
	"github.com/marapongo/mu/pkg/compiler/errors"
	"github.com/marapongo/mu/pkg/compiler/metadata"
	"github.com/marapongo/mu/pkg/compiler/symbols"
	"github.com/marapongo/mu/pkg/compiler/types"
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/pack"
	"github.com/marapongo/mu/pkg/tokens"
	"github.com/marapongo/mu/pkg/util/contract"
	"github.com/marapongo/mu/pkg/workspace"
)

// Binder annotates an existing parse tree with semantic information.
type Binder interface {
	core.Phase

	// BindPackages takes a package AST, resolves all dependencies and tokens inside of it, and returns a fully bound
	// package symbol that can be used for semantic operations (like interpretation and evaluation).
	BindPackage(pkg *pack.Package) *symbols.Package
}

// TypeMap maps AST nodes to their corresponding type.  The semantics of this differ based on the kind of node.  For
// example, an ast.Expression's type is the type of its evaluation; an ast.LocalVariable's type is the bound type of its
// value.  And so on.  This is used during binding, type checking, and evaluation, to perform type-sensitive operations.
// This avoids needing to recreate scopes and/or storing type information on every single node in the AST.
type TypeMap map[ast.Node]symbols.Type

// New allocates a fresh binder object with the given workspace, context, and metadata reader.
func New(w workspace.W, ctx *core.Context, reader metadata.Reader) Binder {
	// Create a new binder and a new scope with an empty symbol table.
	b := &binder{
		w:      w,
		ctx:    ctx,
		reader: reader,
		types:  make(TypeMap),
	}

	// Create a global scope and populate it with all of the predefined type names.  This one's never popped.
	NewScope(&b.scope)
	for _, prim := range types.Primitives {
		b.scope.MustRegister(prim)
	}

	return b
}

type binder struct {
	w      workspace.W     // a workspace in which this compilation is happening.
	ctx    *core.Context   // a context shared across all phases of compilation.
	reader metadata.Reader // a metadata reader (in case we encounter package references).
	scope  *Scope          // the current (mutable) scope.
	types  TypeMap         // the typechecked types for expressions (see TypeMap's comments above).
}

func (b *binder) Diag() diag.Sink {
	return b.ctx.Diag
}

// bindType binds a type token to a symbol.  The node context is used for issuing errors.
func (b *binder) bindType(node *ast.TypeToken) symbols.Type {
	contract.Require(node != nil, "node")

	var extra string
	tok := node.Tok
	tyname := tok.Name()

	if tok.Primitive() {
		// If a primitive type, simply do a lookup into our table of primitives.
		if ty, has := types.Primitives[tyname]; has {
			return ty
		} else {
			glog.V(5).Infof("Failed to bind primitive type '%v'", tok)
			extra = "primitive type unknown"
		}
	} else {
		// Otherwise, we will need to perform a more exhaustive lookup of a qualified type token.
		sym := b.lookupSymbolToken(node, tokens.Token(node.Tok), false)
		if ty, ok := sym.(symbols.Type); ok {
			return ty
		}
	}

	// The type was not found; issue an error, and return Any so we can proceed with typechecking.
	b.Diag().Errorf(errors.ErrorTypeNotFound.At(node), tok, extra)
	return types.Any
}

func (b *binder) getTokenParts(tok tokens.Token) (tokens.PackageName,
	tokens.ModuleName, tokens.ModuleMemberName, tokens.ClassMemberName) {
	// Switch on the token kind to populate the set of names.
	var pkg tokens.PackageName
	var mod tokens.ModuleName
	var mem tokens.ModuleMemberName
	var clm tokens.ClassMemberName
	if tok.HasClassMember() {
		clmtok := tokens.ClassMember(tok)
		pkg = clmtok.Package().Name()
		mod = clmtok.Module().Name()
		mem = tokens.ModuleMemberName(clmtok.Class().Name())
		clm = clmtok.Name()
	} else if tok.HasModuleMember() {
		memtok := tokens.ModuleMember(tok)
		pkg = memtok.Package().Name()
		mod = memtok.Module().Name()
		mem = memtok.Name()
	} else if tok.HasModule() {
		modtok := tokens.Module(tok)
		pkg = modtok.Package().Name()
		mod = modtok.Name()
	} else {
		pkg = tokens.Package(tok).Name()
	}
	contract.Assert(pkg != "")
	return pkg, mod, mem, clm
}

// lookupSymbolToken performs a complex lookup for a complex token; if require is true, failed lookups will issue an
// error; and in any case, the AST node is used as the context for errors (lookup, accessibility, or otherwise).
func (b *binder) lookupSymbolToken(node ast.Node, tok tokens.Token, require bool) symbols.Symbol {
	pkgnm, modnm, memnm, clmnm := b.getTokenParts(tok)
	var sym symbols.Symbol
	var extra string // extra error details
	if pkg, has := b.ctx.Pkgs[pkgnm]; has {
		if modnm == "" {
			sym = pkg
		} else if mod, has := pkg.Modules[modnm]; has {
			if memnm == "" {
				sym = mod
			} else if member, has := mod.Members[memnm]; has {
				// The member was found; validate that it's got the right accessibility.
				acc := member.MemberNode().GetAccess()
				if pkg != b.ctx.Currpkg && (acc == nil || *acc != tokens.PublicAccessibility) {
					b.Diag().Errorf(errors.ErrorMemberNotPublic.At(node), member)
				}

				if clmnm == "" {
					sym = member
				} else if class, isclass := member.(*symbols.Class); isclass {
					if clmember, has := class.Members[clmnm]; has {
						// The member was found; validate that it's got the right accessibility.
						acc := member.MemberNode().GetAccess()
						if pkg != b.ctx.Currpkg && (acc == nil || *acc != tokens.PublicAccessibility) {
							b.Diag().Errorf(errors.ErrorMemberNotPublic.At(node), member)
						}

						sym = clmember
					} else {
						extra = "class found, but member was not found"
					}
				} else {
					extra = "module member is not a class"
				}
			} else {
				extra = "module found, but member was not found"
			}
		} else {
			extra = "package found, but module was not found"
		}
	} else {
		extra = "package was not found"
	}

	if sym == nil {
		glog.V(5).Info("Failed to bind qualified token; %v: '%v'", extra, tok)
		if require {
			// If requested, issue an error.
			b.Diag().Errorf(errors.ErrorSymbolNotFound.At(node), tok, extra)
		}
	}

	return sym
}

// requireToken takes a token of unknown kind and binds it to a symbol in either the symbol table or import map, through
// a series of lookups.  It is a verfification error if the token could not be found.
func (b *binder) requireToken(node ast.Node, tok tokens.Token) symbols.Symbol {
	if tok.HasModule() {
		// A complex token is bound through the normal token binding lookup process.
		if sym := b.lookupSymbolToken(node, tok, true); sym != nil {
			return sym
		}
	} else {
		// A simple token has no package, module, or class part.  It refers to the symbol table.
		if sym := b.scope.Lookup(tok); sym != nil {
			return sym
		} else {
			b.Diag().Errorf(errors.ErrorSymbolNotFound.At(node), tok, "simple name not found")
		}
	}
	return types.Any
}

// lookupClassMember takes a class member token and binds it to a member of the class symbol.  The type of the class
// must match the class part of the token expression and the member must be found.  If not, a verification error occurs.
func (b *binder) requireClassMember(node ast.Node, class symbols.Type, tok tokens.ClassMember) symbols.ClassMember {
	if sym := b.lookupSymbolToken(node, tokens.Token(tok), true); sym != nil {
		switch s := sym.(type) {
		case symbols.ClassMember:
			return s
		default:
			contract.Failf("Expected symbol to be a class member: %v", tok)
		}
	}
	return nil
}

// requireType requires that a type exists for the given AST node.
func (b *binder) requireType(node ast.Node) symbols.Type {
	ty := b.types[node]
	contract.Assertf(ty != nil, "Expected a typemap entry for %v node", node.GetKind())
	return ty
}

// requireExprType fetches an expression's non-nil type.
func (b *binder) requireExprType(node ast.Expression) symbols.Type {
	return b.requireType(node)
}

// registerExprType registers an expression's type.
func (b *binder) registerExprType(node ast.Expression, tysym symbols.Type) {
	contract.Require(tysym != nil, "tysym")
	contract.Assert(b.types[node] == nil)
	if glog.V(7) {
		glog.V(7).Infof("Registered expression type: '%v' => %v", node.GetKind(), tysym.Name())
	}
	b.types[node] = tysym
}

// requireFunctionType fetches the non-nil registered type for a given function.
func (b *binder) requireFunctionType(node ast.Function) *symbols.FunctionType {
	ty := b.requireType(node)
	fty, ok := ty.(*symbols.FunctionType)
	contract.Assertf(ok, "Expected function type for %v; got %v", node.GetKind(), fty.Token())
	return fty
}

// registerFunctionType understands how to turn any function node into a type, and adds it to the type table.  This
// works for any kind of function-like AST node: module property, class property, or lambda.
func (b *binder) registerFunctionType(node ast.Function) {
	// Make a function type and inject it into the type table.
	var params []symbols.Type
	np := node.GetParameters()
	if np != nil {
		for _, param := range *np {
			var ptysym symbols.Type

			// If there was an explicit type, look it up.
			if param.Type != nil {
				ptysym = b.scope.LookupType(param.Type.Tok)
			}

			// If either the parameter's type was unknown, or the lookup failed (leaving an error), use the any type.
			if ptysym == nil {
				ptysym = types.Any
			}

			params = append(params, ptysym)
		}
	}

	var ret symbols.Type
	nr := node.GetReturnType()
	if nr != nil {
		ret = b.scope.LookupType(nr.Tok)
	}

	tysym := symbols.NewFunctionType(params, ret)
	if glog.V(7) {
		glog.V(7).Infof("Registered function type: '%v' => %v", node.GetName().Ident, tysym.Name())
	}
	b.types[node] = tysym
}

// requireVariableType fetches the non-nil registered type for a given variable.
func (b *binder) requireVariableType(node ast.Variable) symbols.Type {
	return b.requireType(node)
}

// registerVariableType understands how to turn any variable node into a type, and adds it to the type table.  This
// works for any kind of variable-like AST node: module property, class property, parameter, or local variable.
func (b *binder) registerVariableType(node ast.Variable) {
	var tysym symbols.Type

	// If there is an explicit node type, use it.
	nt := node.GetType()
	if nt != nil {
		tysym = b.scope.LookupType(nt.Tok)
	}

	// Otherwise, either there was no type, or the lookup failed (leaving behind an error); use the any type.
	if tysym == nil {
		tysym = types.Any
	}

	if glog.V(7) {
		glog.V(7).Infof("Registered variable type: '%v' => %v", node.GetName().Ident, tysym.Name())
	}
	b.types[node] = tysym
}
