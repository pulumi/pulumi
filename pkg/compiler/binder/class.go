// Copyright 2016-2017, Pulumi Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package binder

import (
	"github.com/golang/glog"

	"github.com/pulumi/lumi/pkg/compiler/ast"
	"github.com/pulumi/lumi/pkg/compiler/errors"
	"github.com/pulumi/lumi/pkg/compiler/symbols"
	"github.com/pulumi/lumi/pkg/compiler/types/predef"
	"github.com/pulumi/lumi/pkg/util/contract"
)

func (b *binder) bindClassDeclaration(node *ast.Class, parent *symbols.Module) *symbols.Class {
	glog.V(3).Infof("Binding module '%v' class '%v'", parent.Token(), node.Name.Ident)

	// Now create an empty class symbol.  This is required as a parent for the members.
	class := symbols.NewClassSym(node, parent, nil, nil)
	b.ctx.RegisterSymbol(node, class)

	return class
}

func (b *binder) bindClassDefinition(class *symbols.Class) {
	glog.V(3).Infof("Binding class '%v' definition", class.Token())

	// Bind base type tokens to actual symbols.
	if class.Node.Extends != nil {
		class.SetBase(b.ctx.LookupType(class.Node.Extends))
	}

	if class.Node.Implements != nil {
		for _, impltok := range *class.Node.Implements {
			if impl := b.ctx.LookupType(impltok); impl != nil {
				class.Implements = append(class.Implements, impl)
			}
		}
	}

	b.bindClassMembers(class)

	if class.Ctor() == nil {
		// If there is no ctor defined, ensure that all base classes have no ctor also.
		ext := class.Extends
		for ext != nil {
			if ext.Ctor() != nil {
				b.Diag().Errorf(errors.ErrorDerivedClassHasNoCtor.At(class.Node), class, ext)
			}
			ext = ext.Base()
		}
	}
}

func (b *binder) bindClassMembers(class *symbols.Class) {
	// Set the current class in the context so we can e.g. enforce accessibility.
	pop := b.ctx.PushClass(class)
	defer pop()

	// Bind each member at the symbolic level; in particular, we do not yet bind bodies of methods.
	if class.Node.Members != nil {
		members := *class.Node.Members
		for _, memtok := range ast.StableClassMembers(members) {
			contract.Assert(class.Members[memtok] == nil)
			class.Members[memtok] = b.bindClassMember(members[memtok], class)
		}
	}
}

func (b *binder) bindClassMember(node ast.ClassMember, parent *symbols.Class) symbols.ClassMember {
	switch n := node.(type) {
	case *ast.ClassProperty:
		return b.bindClassProperty(n, parent)
	case *ast.ClassMethod:
		return b.bindClassMethod(n, parent)
	default:
		contract.Failf("Unrecognized class member kind: %v", node.GetKind())
		return nil
	}
}

func (b *binder) bindClassProperty(node *ast.ClassProperty, parent *symbols.Class) *symbols.ClassProperty {
	glog.V(3).Infof("Binding class '%v' property '%v'", parent.Token(), node.Name.Ident)

	// Look up this node's type.
	typ := b.ctx.LookupType(node.Type)

	// If there is a getter and/or setter, verify that it has the correct parameter/return types.
	var get *symbols.ClassMethod
	if getter := node.Getter; getter != nil {
		get = b.bindClassMethod(getter, parent)
		contract.Assert(get != nil)

		sig := get.Signature()
		if pc := len(sig.Parameters); pc != 0 {
			b.Diag().Errorf(errors.ErrorPropertyGetterParamCount.At(node.Getter), pc)
		}
		if ret := sig.Return; ret != typ {
			b.Diag().Errorf(errors.ErrorPropertyGetterReturnType.At(node.Getter), ret, typ)
		}
	}
	var set *symbols.ClassMethod
	if setter := node.Setter; setter != nil {
		set = b.bindClassMethod(setter, parent)
		contract.Assert(set != nil)

		sig := set.Signature()
		if params := sig.Parameters; len(params) != 1 {
			b.Diag().Errorf(errors.ErrorPropertySetterParamCount.At(node.Setter), len(params))
		} else if ptype := params[0]; ptype != typ {
			b.Diag().Errorf(errors.ErrorPropertySetterParamType.At(node.Setter), ptype, typ)
		}
		if ret := sig.Return; ret != nil {
			b.Diag().Errorf(errors.ErrorPropertySetterReturnType.At(node.Setter), ret)
		}
	}

	// If this is a resource property, mark it as latent so that we can speculate before true evaluation.
	latent := predef.IsLatentResourceProperty(parent, typ)

	// Now inject this into the symbol table and return it.
	sym := symbols.NewClassPropertySym(node, parent, typ, get, set, latent)
	b.ctx.RegisterSymbol(node, sym)
	return sym
}

func (b *binder) bindClassMethod(node *ast.ClassMethod, parent *symbols.Class) *symbols.ClassMethod {
	glog.V(3).Infof("Binding class '%v' method '%v'", parent.Token(), node.Name.Ident)

	// Make a function type out of this method and inject it into the type table.
	typ := b.ctx.LookupFunctionType(node)
	sym := symbols.NewClassMethodSym(node, parent, typ)
	b.ctx.RegisterSymbol(node, sym)

	// All methods have bodies (unless they are abstract).
	if node.Body == nil && (node.Abstract == nil || !*node.Abstract) {
		b.Diag().Errorf(errors.ErrorMethodsMustHaveBodies.At(node), sym, "a concrete class method")
	}

	// Note that we don't actually bind the body of this method yet.  Until we have gone ahead and injected *all*
	// top-level symbols into the type table, we would potentially encounter missing intra-module symbols.
	return sym
}

func (b *binder) bindClassBodies(class *symbols.Class) {
	for _, member := range symbols.StableClassMemberMap(class.Members) {
		switch m := class.Members[member].(type) {
		case *symbols.ClassMethod:
			b.bindClassMethodBody(m)
		case *symbols.ClassProperty:
			if m.Get != nil {
				b.bindClassMethodBody(m.Get)
			}
			if m.Set != nil {
				b.bindClassMethodBody(m.Set)
			}
		}
	}
}

func (b *binder) bindClassMethodBody(method *symbols.ClassMethod) {
	glog.V(3).Infof("Binding class method '%v' body", method.Token())

	// Set the current class in the context so we can e.g. enforce accessibility.  Note that we have to do it here, in
	// addition to the above during ordinary class binding, due to the two pass function body binding model.
	priorclass := b.ctx.Currclass
	b.ctx.Currclass = method.Parent
	defer func() { b.ctx.Currclass = priorclass }()

	// Push a new activation frame and, if this isn't a static, register the special this/super variables.
	scope := b.ctx.Scope.Push(true)
	defer scope.Pop()
	if !method.Static() {
		// Register the "this" and, if relevant, "super" special variables.
		this := method.Parent.This
		contract.Assert(this != nil)
		b.ctx.Scope.MustRegister(this)
		super := method.Parent.Super
		if super != nil {
			b.ctx.Scope.MustRegister(super)
		}
	}

	b.bindFunctionBody(method.Node)
}
