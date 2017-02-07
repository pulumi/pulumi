// Copyright 2016 Marapongo, Inc. All rights reserved.

package binder

import (
	"fmt"
	"reflect"

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
	}
	return b.bindTypeToken(node, node.Tok)
}

// bindTypeToken binds a type token to a symbol.  The node context is used for issuing errors.
func (b *binder) bindTypeToken(node ast.Node, tok tokens.Type) symbols.Type {
	contract.Require(node != nil, "node")

	var reason string // the reason a type could not be found.
	if tok.Primitive() {
		// If a primitive type, simply do a lookup into our table of primitives.
		if ty, has := types.Primitives[tok.Name()]; has {
			return ty
		}
		glog.V(5).Infof("Failed to bind primitive type '%v'", tok)
		reason = "primitive type unknown"
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
	} else if sym := b.lookupSymbolToken(node, tokens.Token(tok), false); sym != nil {
		// Otherwise, we will need to perform a more exhaustive lookup of a qualified type token.
		if ty, ok := sym.(symbols.Type); ok {
			return ty
		}
		reason = fmt.Sprintf("symbol kind %v incorrect", reflect.TypeOf(sym))
	} else {
		reason = "symbol missing"
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

// bindModuleToken binds a module token AST node to a symbol.
func (b *binder) bindModuleToken(node *ast.ModuleToken) *symbols.Module {
	if node == nil {
		return nil
	}

	sym := b.lookupSymbolToken(node, tokens.Token(node.Tok), true)
	if sym != nil {
		if module, ok := sym.(*symbols.Module); ok {
			return module
		}
		b.Diag().Errorf(errors.ErrorSymbolNotFound.At(node),
			node.Tok, fmt.Sprintf("symbol isn't a module: %v", reflect.TypeOf(sym)))
	}
	return nil
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
	// If the token has a default module embedded inside of it, expand it out.
	pkg, mod, mem, clm := tok.Tokens()
	if mod != "" && mod.Name() == tokens.DefaultModule {
		if pkgsym, has := b.ctx.Pkgs[pkg.Name()]; has {
			// Fetch the default module (if there is one; if not, we fall through and fail below).
			defmod := pkgsym.Pkg.Default()
			if defmod != nil {
				contract.Assert(*defmod != "")

				// Recreate the token using the new module value.
				mod = tokens.NewModuleToken(pkg, *defmod)
				if mem != "" {
					mem = tokens.NewModuleMemberToken(mod, mem.Name())
					if clm != "" {
						clm = tokens.NewClassMemberToken(tokens.Type(mem), clm.Name())
						tok = tokens.Token(clm)
					} else {
						tok = tokens.Token(mem)
					}
				} else {
					tok = tokens.Token(mod)
				}
			}
		}
	}

	// Simply look up the symbol in the tokens map.
	sym := b.ctx.Tokens[tok]

	if sym == nil {
		glog.V(5).Infof("Failed to bind qualified token; '%v'", tok)
		if require {
			// If requested, issue an error.  First, attempt to gather a good error message, however.
			// TODO: edit distance checking to help suggest a fix.
			var reason string
			if _, has := b.ctx.Pkgs[pkg.Name()]; !has {
				reason = fmt.Sprintf("package '%v' not found", pkg)
			} else if _, has := b.ctx.Tokens[tokens.Token(mod)]; !has {
				reason = fmt.Sprintf("module '%v' not found", mod)
			} else if _, has := b.ctx.Tokens[tokens.Token(mem)]; !has {
				reason = fmt.Sprintf("class '%v' not found", mem)
			} else if _, has := b.ctx.Tokens[tokens.Token(clm)]; !has {
				reason = fmt.Sprintf("class member '%v' not found", clm)
			} else {
				reason = "invalid symbol token"
			}
			b.Diag().Errorf(errors.ErrorSymbolNotFound.At(node), tok, reason)
		}
	} else if glog.V(7) {
		glog.V(7).Infof("Successfully bound qualified token: %v", tok)
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
		b.Diag().Errorf(errors.ErrorSymbolNotFound.At(node), tok, "qualified token not found")
	} else {
		// A simple token has no package, module, or class part.  It refers to the symbol table.
		if sym := b.ctx.Scope.Lookup(tok.Name()); sym != nil {
			return sym
		}
		b.Diag().Errorf(errors.ErrorSymbolNotFound.At(node), tok, "simple name not found")
	}
	return nil
}

// lookupClassMember takes a class member token and binds it to a member of the class symbol.  The type of the class
// must match the class part of the token expression and the member must be found.  If not, a verification error occurs.
func (b *binder) requireClassMember(node ast.Node, class symbols.Type, tok tokens.ClassMember) symbols.ClassMember {
	if sym := b.lookupSymbolToken(node, tokens.Token(tok), true); sym != nil {
		switch s := sym.(type) {
		case symbols.ClassMember:
			return s
		default:
			b.Diag().Errorf(errors.ErrorSymbolNotFound.At(node), tok, "class member not found")
		}
	}
	return nil
}
