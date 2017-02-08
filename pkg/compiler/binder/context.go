// Copyright 2016 Marapongo, Inc. All rights reserved.

package binder

import (
	"fmt"
	"reflect"

	"github.com/golang/glog"

	"github.com/marapongo/mu/pkg/compiler/ast"
	"github.com/marapongo/mu/pkg/compiler/core"
	"github.com/marapongo/mu/pkg/compiler/errors"
	"github.com/marapongo/mu/pkg/compiler/symbols"
	"github.com/marapongo/mu/pkg/compiler/types"
	"github.com/marapongo/mu/pkg/tokens"
	"github.com/marapongo/mu/pkg/util/contract"
)

// Context holds binder-specific context information, like symbol and type binding information.
type Context struct {
	*core.Context           // inherits all of the other context info.
	Scope         *Scope    // the current (mutable) scope.
	Types         TypeMap   // the type-checked type symbols for expressions.
	Symbols       SymbolMap // the fully bound symbol information for all definitions.
}

// TypeMap maps AST expression nodes to their corresponding type.  This is used during binding, type checking, and
// evaluation, to perform type-sensitive operations.  This avoids needing to recreate scopes in subsequent passes of the
// compiler and/or storing type information on every single node in the AST.
type TypeMap map[ast.Expression]symbols.Type

// SymbolMap maps all known definition AST definition nodes to their corresponding symbols.
type SymbolMap map[ast.Definition]symbols.Symbol

// NewContextFrom allocates a fresh binding context linked to the shared context object.
func NewContextFrom(ctx *core.Context) *Context {
	bctx := &Context{
		Context: ctx,
		Types:   make(TypeMap),
		Symbols: make(SymbolMap),
	}
	NewScope(bctx, true)
	return bctx
}

// HasType checks whether a typemap entry already exists.
func (ctx *Context) HasType(node ast.Expression) bool {
	_, has := ctx.Types[node]
	return has
}

// LookupType binds a type token AST node to a symbol.
func (ctx *Context) LookupType(node *ast.TypeToken) symbols.Type {
	if node == nil {
		return nil
	}
	return ctx.LookupTypeToken(node, node.Tok, true)
}

// LookupModule binds a module token AST node to a symbol.
func (ctx *Context) LookupModule(node *ast.ModuleToken) *symbols.Module {
	if node == nil {
		return nil
	}

	sym := ctx.LookupSymbol(node, tokens.Token(node.Tok), true)
	if sym != nil {
		if module, ok := sym.(*symbols.Module); ok {
			return module
		}
		ctx.Diag.Errorf(errors.ErrorSymbolNotFound.At(node),
			node.Tok, fmt.Sprintf("symbol isn't a module: %v", reflect.TypeOf(sym)))
	}
	return nil
}

// LookupSymbol performs a complex lookup for a complex token; if require is true, failed lookups will issue an
// error; and in any case, the AST node is used as the context for errors (lookup, accessibility, or otherwise).
func (ctx *Context) LookupSymbol(node ast.Node, tok tokens.Token, require bool) symbols.Symbol {
	var sym symbols.Symbol // the symbol, if found.
	var reason string      // the reason reason, if a symbol could not be found.

	// If the token is decorated, use the type lookup routine.
	t := tokens.Type(tok)
	if t.Primitive() || t.Decorated() {
		if tok.HasClassMember() {
			t = tok.ClassMember().Class() // strip off any member part.
		}
		if tysym := ctx.lookupBasicType(node, t, require); tysym != nil {
			if tok.HasClassMember() {
				// If there's a class member part, yank it out, and bind it.
				clm := tokens.ClassMember(tok).Name()
				if clmsym := tysym.TypeMembers()[clm]; clmsym != nil {
					sym = clmsym
				} else {
					reason = fmt.Sprintf("class member '%v' not found", clm)
				}
			} else {
				sym = tysym
			}
		} else {
			reason = fmt.Sprintf("basic type '%v' not found", t)
		}
	} else {
		// Otherwise, start searching the elements of the token, beginning with package.
		pkg := tok.Package()
		if pkgsym := ctx.LookupPackageSymbol(pkg.Name()); pkgsym != nil {
			if tok.HasModule() {
				mod := tok.Module().Name()
				if modsym := ctx.LookupModuleSymbol(pkgsym, mod); modsym != nil {
					if tok.HasModuleMember() {
						mem := tok.ModuleMember().Name()
						if memsym := modsym.Members[mem]; memsym != nil {
							if tok.HasClassMember() {
								if class, isclass := memsym.(*symbols.Class); isclass {
									clm := tok.ClassMember().Name()
									if clmsym := class.Members[clm]; clmsym != nil {
										sym = clmsym
									} else {
										reason = fmt.Sprintf("class member '%v' not found", clm)
									}
								} else {
									reason = fmt.Sprintf("module member '%v' is not a class", mem)
								}
							} else {
								sym = memsym
							}
						} else {
							reason = fmt.Sprintf("module member '%v' not found", mem)
						}
					} else {
						sym = modsym
					}
				} else {
					reason = fmt.Sprintf("module '%v' not found", mod)
				}
			} else {
				sym = pkgsym
			}
		} else {
			reason = fmt.Sprintf("package '%v' not found", pkg)
		}
	}

	if sym == nil {
		glog.V(5).Infof("Failed to bind symbol token; '%v'", tok)
		if require {
			// If requested, issue an error.
			contract.Assert(reason != "")
			ctx.Diag.Errorf(errors.ErrorSymbolNotFound.At(node), tok, reason)
		}
	} else if glog.V(7) {
		glog.V(7).Infof("Successfully bound symbol token: %v", tok)
	}

	return sym
}

func (ctx *Context) LookupPackageSymbol(name tokens.PackageName) *symbols.Package {
	if pkg, has := ctx.Pkgs[name]; has {
		return pkg.Pkg
	}
	return nil
}

func (ctx *Context) LookupModuleSymbol(pkg *symbols.Package, module tokens.ModuleName) *symbols.Module {
	if module == tokens.DefaultModule {
		// Fetch the default module (if there is one; if not, we fall through and fail below).
		if defmodule := pkg.Default(); defmodule != nil {
			module = *defmodule
			contract.Assert(module != "")
		}
	}

	return pkg.Modules[module]
}

// LookupTypeToken binds a type token to its symbol, creating elements if needed.  The node context is used for errors.
func (ctx *Context) LookupTypeToken(node ast.Node, tok tokens.Type, require bool) symbols.Type {
	contract.Require(node != nil, "node")

	var ty symbols.Type // the type, if any.
	var reason string   // the reason a type could not be found.
	if tok.Primitive() || tok.Decorated() {
		// If primitive or decorated, handle it separately.
		ty = ctx.lookupBasicType(node, tok, require)
		if ty == nil {
			reason = "basic type not found"
		}
	} else {
		// Otherwise, we will need to perform a more exhaustive lookup of a qualified type token.
		if sym := ctx.LookupSymbol(node, tokens.Token(tok), require); sym != nil {
			if typ, ok := sym.(symbols.Type); ok {
				ty = typ
			} else {
				reason = fmt.Sprintf("%v symbol is not a type", reflect.TypeOf(sym))
			}
		} else {
			reason = "type symbol not found"
		}
	}

	// The type was not found; issue an error, and return Any so we can proceed with typechecking.
	if ty == nil {
		if require {
			contract.Assert(reason != "")
			ctx.Diag.Errorf(errors.ErrorTypeNotFound.At(node), tok, reason)
		}
		ty = types.Any
	}
	return ty
}

// LookupFunctionType binds a function node to its corresponding FunctionType symbol.
func (ctx *Context) LookupFunctionType(node ast.Function) *symbols.FunctionType {
	var params []symbols.Type
	np := node.GetParameters()
	if np != nil {
		for _, param := range *np {
			// If there was an explicit type, look it up.
			ptysym := ctx.LookupType(param.Type)

			// If either the parameter's type was unknown, or the lookup failed (leaving an error), use the any type.
			if ptysym == nil {
				ptysym = types.Any
			}

			params = append(params, ptysym)
		}
	}

	// Bind the optional return type.
	ret := ctx.LookupType(node.GetReturnType())

	return symbols.NewFunctionType(params, ret)
}

// lookupBasicType handles decorated types (pointers, arrays, maps, functions) and primitives.
func (ctx *Context) lookupBasicType(node ast.Node, tok tokens.Type, require bool) symbols.Type {
	contract.Require(node != nil, "node")
	contract.Requiref(tok.Primitive() || tok.Decorated(), "tok", "Primitive() || Decorated()")

	// If a pointer, parse it, bind the element type, and create a new pointer type.
	if tok.Pointer() {
		ptr := tokens.ParsePointerType(tok)
		elem := ctx.LookupTypeToken(node, ptr.Elem, require)
		return symbols.NewPointerType(elem)
	}

	// If an array, parse it, bind the element type, and create a new array symbol.
	if tok.Array() {
		arr := tokens.ParseArrayType(tok)
		elem := ctx.LookupTypeToken(node, arr.Elem, require)
		return symbols.NewArrayType(elem)
	}

	// If a map, parse it, bind the key and value types, and create a new map symbol.
	if tok.Map() {
		ma := tokens.ParseMapType(tok)
		key := ctx.LookupTypeToken(node, ma.Key, require)
		elem := ctx.LookupTypeToken(node, ma.Elem, require)
		return symbols.NewMapType(key, elem)
	}

	// If a function, parse and bind the parameters and return types, and create a new symbol.
	if tok.Function() {
		fnc := tokens.ParseFunctionType(tok)
		var params []symbols.Type
		for _, param := range fnc.Parameters {
			params = append(params, ctx.LookupTypeToken(node, param, require))
		}
		var ret symbols.Type
		if fnc.Return != nil {
			ret = ctx.LookupTypeToken(node, *fnc.Return, require)
		}
		return symbols.NewFunctionType(params, ret)
	}

	// If a primitive type, simply do a lookup into our table of primitives.
	contract.Assert(tok.Primitive())
	if ty, has := types.Primitives[tok.Name()]; has {
		return ty
	}
	glog.V(5).Infof("Failed to bind primitive type '%v'", tok)

	if require {
		ctx.Diag.Errorf(errors.ErrorTypeNotFound.At(node), tok, "unrecognized primitive type name")
	}
	return types.Any
}

func (ctx *Context) checkModuleVisibility(node ast.Node, module *symbols.Module, member symbols.ModuleMember) {
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
		if module != ctx.Currmodule {
			ctx.Diag.Errorf(errors.ErrorMemberNotAccessible.At(node), member, *acc)
		}
	default:
		contract.Failf("Unrecognized module member accessibility: %v", *acc)
	}
}

func (ctx *Context) checkClassVisibility(node ast.Node, class *symbols.Class, member symbols.ClassMember) {
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
		if class != ctx.Currclass {
			ctx.Diag.Errorf(errors.ErrorMemberNotAccessible.At(node), member, *acc)
		}
	case tokens.ProtectedClassAccessibility:
		if !types.CanConvert(ctx.Currclass, class) {
			ctx.Diag.Errorf(errors.ErrorMemberNotAccessible.At(node), member, *acc)
		}
	default:
		contract.Failf("Unrecognized class member accessibility: %v", *acc)
	}
}

// RequireToken takes a token of unknown kind and binds it to a symbol in either the symbol table or import map, through
// a series of lookups.  It is a verfification error if the token could not be found.
func (ctx *Context) RequireToken(node ast.Node, tok tokens.Token) symbols.Symbol {
	if tok.HasModule() {
		// A complex token is bound through the normal token binding lookup process.
		if sym := ctx.LookupSymbol(node, tok, true); sym != nil {
			return sym
		}
		ctx.Diag.Errorf(errors.ErrorSymbolNotFound.At(node), tok, "qualified token not found")
	} else {
		// A simple token has no package, module, or class part.  It refers to the symbol table.
		if sym := ctx.Scope.Lookup(tok.Name()); sym != nil {
			return sym
		}
		ctx.Diag.Errorf(errors.ErrorSymbolNotFound.At(node), tok, "simple name not found")
	}
	return nil
}

// LookupClassMember takes a class member token and binds it to a member of the class symbol.  The type of the class
// must match the class part of the token expression and the member must be found.  If not, a verification error occurs.
func (ctx *Context) RequireClassMember(node ast.Node,
	class symbols.Type, tok tokens.ClassMember) symbols.ClassMember {
	if sym := ctx.LookupSymbol(node, tokens.Token(tok), true); sym != nil {
		switch s := sym.(type) {
		case symbols.ClassMember:
			return s
		default:
			ctx.Diag.Errorf(errors.ErrorSymbolNotFound.At(node), tok, "class member not found")
		}
	}
	return nil
}

// RequireType requires that a type exists for the given AST expression node.
func (ctx *Context) RequireType(node ast.Expression) symbols.Type {
	contract.Require(node != nil, "node")
	contract.Requiref(ctx.HasType(node), "ctx", "HasType(node)")
	return ctx.Types[node]
}

// RegisterType registers an expression's type.
func (ctx *Context) RegisterType(node ast.Expression, tysym symbols.Type) {
	contract.Require(node != nil, "node")
	contract.Requiref(!ctx.HasType(node), "ctx", "!HasType(node)")
	if glog.V(7) {
		glog.V(7).Infof("Registered expression type: '%v' => %v", node.GetKind(), tysym.Name())
	}
	ctx.Types[node] = tysym
}

// RequireDefinition fetches the non-nil registered symbol for a given definition node.
func (ctx *Context) RequireDefinition(node ast.Definition) symbols.Symbol {
	contract.Require(node != nil, "node")
	sym := ctx.Symbols[node]
	contract.Assertf(sym != nil, "Expected a symbol entry for %v node", node.GetKind())
	return sym
}

// RegisterDefinition registers a definition's symbol.
func (ctx *Context) RegisterSymbol(node ast.Definition, sym symbols.Symbol) {
	contract.Require(node != nil, "node")
	contract.Require(sym != nil, "sym")
	contract.Assert(ctx.Symbols[node] == nil)
	if glog.V(7) {
		glog.V(7).Infof("Registered definition symbol: '%v' => %v", node.GetKind(), sym.Name())
	}
	ctx.Symbols[node] = sym
}

func (ctx *Context) RequireFunction(fnc ast.Function) symbols.Function {
	return ctx.RequireDefinition(fnc).(symbols.Function)
}

func (ctx *Context) RequireVariable(fnc ast.Variable) symbols.Variable {
	return ctx.RequireDefinition(fnc).(symbols.Variable)
}
