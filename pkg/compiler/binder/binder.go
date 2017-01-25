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

	// Ctx represents the contextual information resulting from binding.
	Ctx() *Context

	// BindPackages takes a package AST, resolves all dependencies and tokens inside of it, and returns a fully bound
	// package symbol that can be used for semantic operations (like interpretation and evaluation).
	BindPackage(pkg *pack.Package) *symbols.Package
}

// New allocates a fresh binder object with the given workspace, context, and metadata reader.
func New(w workspace.W, ctx *core.Context, reader metadata.Reader) Binder {
	// Create a new binder with a fresh binding context.
	return &binder{
		w:      w,
		ctx:    NewContextFrom(ctx),
		reader: reader,
	}
}

type binder struct {
	w      workspace.W     // a workspace in which this compilation is happening.
	ctx    *Context        // a binding context shared with future phases of compilation.
	reader metadata.Reader // a metadata reader (in case we encounter package references).
}

func (b *binder) Ctx() *Context   { return b.ctx }
func (b *binder) Diag() diag.Sink { return b.ctx.Diag }

// bindType binds a type token AST node to a symbol.
func (b *binder) bindType(node *ast.TypeToken) symbols.Type {
	if node == nil {
		return nil
	} else {
		return b.bindTypeToken(node, node.Tok)
	}
}

// bindTypeToken binds a type token to a symbol.  The node context is used for issuing errors.
func (b *binder) bindTypeToken(node ast.Node, tok tokens.Type) symbols.Type {
	contract.Require(node != nil, "node")

	var reason string // the reason a type could not be found.
	if tok.Primitive() {
		// If a primitive type, simply do a lookup into our table of primitives.
		if ty, has := types.Primitives[tok.Name()]; has {
			return ty
		} else {
			glog.V(5).Infof("Failed to bind primitive type '%v'", tok)
			reason = "primitive type unknown"
		}
	} else if tok.Pointer() {
		// If a pointer, parse it, bind the element type, and create a new pointer type.
		ptr := tokens.ParsePointerType(tok)
		elem := b.bindTypeToken(node, ptr.Elem)
		return symbols.NewPointerType(elem)
	} else if tok.Array() {
		// If an array, parse it, bind the element type, and create a new array symbol.
		arr := tokens.ParseArrayType(tok)
		elem := b.bindTypeToken(node, arr.Elem)
		return symbols.NewArrayType(elem)
	} else if tok.Map() {
		// If a map, parse it, bind the key and value types, and create a new map symbol.
		ma := tokens.ParseMapType(tok)
		key := b.bindTypeToken(node, ma.Key)
		elem := b.bindTypeToken(node, ma.Elem)
		return symbols.NewMapType(key, elem)
	} else if tok.Function() {
		// If a function, parse and bind the parameters and return types, and create a new symbol.
		fnc := tokens.ParseFunctionType(tok)
		var params []symbols.Type
		for _, param := range fnc.Parameters {
			params = append(params, b.bindTypeToken(node, param))
		}
		var ret symbols.Type
		if fnc.Return != nil {
			ret = b.bindTypeToken(node, *fnc.Return)
		}
		return symbols.NewFunctionType(params, ret)
	} else {
		// Otherwise, we will need to perform a more exhaustive lookup of a qualified type token.
		sym := b.lookupSymbolToken(node, tokens.Token(tok), false)
		if ty, ok := sym.(symbols.Type); ok {
			return ty
		}
		reason = "symbol not found"
	}

	// The type was not found; issue an error, and return Any so we can proceed with typechecking.
	b.Diag().Errorf(errors.ErrorTypeNotFound.At(node), tok, reason)
	return types.Any
}

// bindFunctionType binds a function node to its corresponding FunctionType symbol.
func (b *binder) bindFunctionType(node ast.Function) *symbols.FunctionType {
	var params []symbols.Type
	np := node.GetParameters()
	if np != nil {
		for _, param := range *np {
			// If there was an explicit type, look it up.
			ptysym := b.bindType(param.Type)

			// If either the parameter's type was unknown, or the lookup failed (leaving an error), use the any type.
			if ptysym == nil {
				ptysym = types.Any
			}

			params = append(params, ptysym)
		}
	}

	// Bind the optional return type.
	ret := b.bindType(node.GetReturnType())

	return symbols.NewFunctionType(params, ret)
}

func (b *binder) checkModuleVisibility(node ast.Node, module *symbols.Module, member symbols.ModuleMember) {
	acc := member.MemberNode().GetAccess()
	if acc == nil {
		a := tokens.PrivateAccessibility // private is the default
		acc = &a
	}

	// Module members have two accessibilities: public or private.  If it's public, no problem.  Otherwise, unless the
	// target module and current module are the same, we should issue an error.
	switch *acc {
	case tokens.PublicAccessibility:
		// ok.
	case tokens.PrivateAccessibility:
		if module != b.ctx.Currmodule {
			b.Diag().Errorf(errors.ErrorMemberNotAccessible.At(node), member, *acc)
		}
	default:
		contract.Failf("Unrecognized module member accessibility: %v", *acc)
	}
}

func (b *binder) checkClassVisibility(node ast.Node, class *symbols.Class, member symbols.ClassMember) {
	acc := member.MemberNode().GetAccess()
	if acc == nil {
		a := tokens.PrivateClassAccessibility // private is the default.
		acc = &a
	}

	// Class members have three accessibilities: public, private, or protected.  If it's public, anything goes.  If
	// private, only permit access from within the same class.  If protected, only the same class or base-classes.
	switch *acc {
	case tokens.PublicClassAccessibility:
		// ok
	case tokens.PrivateClassAccessibility:
		if class != b.ctx.Currclass {
			b.Diag().Errorf(errors.ErrorMemberNotAccessible.At(node), member, *acc)
		}
	case tokens.ProtectedClassAccessibility:
		if !types.CanConvert(b.ctx.Currclass, class) {
			b.Diag().Errorf(errors.ErrorMemberNotAccessible.At(node), member, *acc)
		}
	default:
		contract.Failf("Unrecognized class member accessibility: %v", *acc)
	}
}

// lookupSymbolToken performs a complex lookup for a complex token; if require is true, failed lookups will issue an
// error; and in any case, the AST node is used as the context for errors (lookup, accessibility, or otherwise).
func (b *binder) lookupSymbolToken(node ast.Node, tok tokens.Token, require bool) symbols.Symbol {
	pkgnm, modnm, memnm, clmnm := tok.Parts()
	var sym symbols.Symbol
	var extra string // extra error details
	if pkg, has := b.ctx.Pkgs[pkgnm]; has {
		if modnm == "" {
			sym = pkg.Pkg
		} else if mod, has := pkg.Pkg.Modules[modnm]; has {
			if memnm == "" {
				sym = mod
			} else if member, has := mod.Members[memnm]; has {
				// The member was found; validate that it's got the right accessibility.
				b.checkModuleVisibility(node, mod, member)

				if clmnm == "" {
					sym = member
				} else if class, isclass := member.(*symbols.Class); isclass {
					if clmember, has := class.Members[clmnm]; has {
						// The member was found; validate that it's got the right accessibility.
						b.checkClassVisibility(node, class, clmember)
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
			// TODO: edit distance checking to help suggest a fix.
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
		if sym := b.ctx.Scope.Lookup(tok); sym != nil {
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
