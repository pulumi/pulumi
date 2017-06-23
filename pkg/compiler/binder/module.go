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
	"reflect"

	"github.com/golang/glog"

	"github.com/pulumi/lumi/pkg/compiler/ast"
	"github.com/pulumi/lumi/pkg/compiler/errors"
	"github.com/pulumi/lumi/pkg/compiler/symbols"
	"github.com/pulumi/lumi/pkg/util/contract"
)

func (b *binder) bindModuleDeclarations(node *ast.Module, parent *symbols.Package) *symbols.Module {
	contract.Assert(node != nil)
	contract.Assert(parent != nil)
	glog.V(3).Infof("Binding package '%v' module '%v' decls", parent.Name(), node.Name.Ident)

	// Create the module symbol and register it.
	module := symbols.NewModuleSym(node, parent)
	b.ctx.RegisterSymbol(node, module)

	// Now bind module member declarations; this just populates the top-level declaration symbolic information, without
	// any inter-dependencies on other module declarations that may not have yet been completed.
	b.bindModuleMemberDeclarations(module)

	return module
}

// bindModuleMemberDeclarations binds a module's member names.  This must be done before binding definitions because
// they could mention other members whose symbolic information may not have been registered yet.  Note that class
// definitions are not yet bound during this pass, since they could reference module members not yet bound.
func (b *binder) bindModuleMemberDeclarations(module *symbols.Module) {
	glog.V(3).Infof("Binding module '%v' member decls", module.Token())

	// Set the current module in the context so we can e.g. enforce accessibility.
	pop := b.ctx.PushModule(module)
	defer pop()

	// Bind all export declarations.
	if module.Node.Exports != nil {
		exports := *module.Node.Exports
		for _, nm := range ast.StableModuleExports(exports) {
			contract.Assert(module.Exports[nm] == nil)
			module.Exports[nm] = b.bindExportDeclaration(exports[nm], module)
		}
	}

	// Now bind all member declarations.
	if module.Node.Members != nil {
		members := *module.Node.Members
		for _, nm := range ast.StableModuleMembers(members) {
			contract.Assert(module.Members[nm] == nil)
			switch m := members[nm].(type) {
			case *ast.Class:
				module.Members[nm] = b.bindClassDeclaration(m, module)
			case *ast.ModuleMethod:
				module.Members[nm] = b.bindModuleMethodDeclaration(m, module)
			case *ast.ModuleProperty:
				module.Members[nm] = b.bindModulePropertyDeclaration(m, module)
			default:
				contract.Failf("Unrecognized module member type: %v", reflect.TypeOf(m))
			}
		}
	}
}

func (b *binder) bindExportDeclaration(node *ast.Export, parent *symbols.Module) *symbols.Export {
	glog.V(3).Infof("Binding module '%v' export '%v' decl", parent.Token(), node.Name.Ident)

	// Simply register an empty export, unlinked to the referent yet.
	sym := symbols.NewExportSym(node, parent, nil)
	b.ctx.RegisterSymbol(node, sym)
	return sym
}

func (b *binder) bindModulePropertyDeclaration(node *ast.ModuleProperty,
	parent *symbols.Module) *symbols.ModuleProperty {
	glog.V(3).Infof("Binding module '%v' property '%v' decl", parent.Token(), node.Name.Ident)

	// Simply create an untyped property declaration.  The type lookup will happen in a subsequent pass.
	sym := symbols.NewModulePropertySym(node, parent, nil)
	b.ctx.RegisterSymbol(node, sym)
	return sym
}

func (b *binder) bindModuleMethodDeclaration(node *ast.ModuleMethod,
	parent *symbols.Module) *symbols.ModuleMethod {
	glog.V(3).Infof("Binding module '%v' method '%v' decl", parent.Token(), node.Name.Ident)

	// Simply create a function declaration without any type.  That will happen in a subsequent pass.
	sym := symbols.NewModuleMethodSym(node, parent, nil)
	b.ctx.RegisterSymbol(node, sym)

	// Ensure that module methods have bodies.
	if node.Body == nil {
		b.Diag().Errorf(errors.ErrorMethodsMustHaveBodies.At(node), sym, "a module method")
	}

	return sym
}

func (b *binder) bindModuleExports(module *symbols.Module) {
	glog.V(3).Infof("Binding module '%v' exports", module.Token())

	// Set the current module in the context so we can e.g. enforce accessibility.
	pop := b.ctx.PushModule(module)
	defer pop()

	for _, nm := range symbols.StableModuleExportMap(module.Exports) {
		export := module.Exports[nm]
		glog.V(3).Infof("Binding module export '%v' defn", export.Token())
		// To bind an export definition, simply look up the referent symbol and associate this name with it.  Note that
		// we can't fully resolve the export recursively, since other exports might still being bound.
		export.Referent = b.ctx.LookupShallowSymbol(export.Node.Referent, export.Node.Referent.Tok, true)
	}
}

func (b *binder) bindModuleDefinitions(module *symbols.Module) {
	// And finish binding the members themselves.
	b.bindModuleMemberDefinitions(module)
}

// bindModuleMemberDefinitions finishes binding module members, by doing lookups sensitive to the definition pass.
func (b *binder) bindModuleMemberDefinitions(module *symbols.Module) {
	glog.V(3).Infof("Binding module '%v' member defns", module.Token())

	// Set the current module in the context so we can e.g. enforce accessibility.
	pop := b.ctx.PushModule(module)
	defer pop()

	// Now complete all member definitions.
	for _, nm := range symbols.StableModuleMemberMap(module.Members) {
		switch m := module.Members[nm].(type) {
		case *symbols.Class:
			b.bindClassDefinition(m)
		case *symbols.ModuleMethod:
			b.bindModuleMethodDefinition(m)
		case *symbols.ModuleProperty:
			b.bindModulePropertyDefinition(m)
		default:
			contract.Failf("Unrecognized module member type: %v", reflect.TypeOf(m))
		}
	}
}

func (b *binder) bindModulePropertyDefinition(property *symbols.ModuleProperty) {
	glog.V(3).Infof("Binding module property '%v' defn", property.Token())

	// Look up this node's type and remember the type on the symbol.
	property.Ty = b.ctx.LookupType(property.Node.Type)
}

func (b *binder) bindModuleMethodDefinition(method *symbols.ModuleMethod) {
	glog.V(3).Infof("Binding module method '%v' defn", method.Token())

	// Make a function type out of this method and store it on the symbol.
	method.Sig = b.ctx.LookupFunctionType(method.Node)

	// Note that we don't actually bind the body of this method yet.  Until we have gone ahead and injected *all*
	// top-level symbols into the type table, we would potentially encounter missing intra-module symbols.
}

// bindModuleBodies binds both the module's direct methods in addition to class methods.  This must be done after
// all top-level symbolic information has been bound, in case definitions, statements, and expressions depend upon them.
func (b *binder) bindModuleBodies(module *symbols.Module) {
	// Set the current module in the context so we can e.g. enforce accessibility.  We need to do this again while
	// binding the module bodies so that the correct context is reestablished for lookups, etc.
	priormodule := b.ctx.Currmodule
	b.ctx.Currmodule = module
	defer func() { b.ctx.Currmodule = priormodule }()

	// Just dig through, find all ModuleMethod and ClassMethod symbols, and bind them.
	for _, nm := range symbols.StableModuleMemberMap(module.Members) {
		member := module.Members[nm]
		switch m := member.(type) {
		case *symbols.ModuleMethod:
			b.bindModuleMethodBody(m)
		case *symbols.Class:
			b.bindClassBodies(m)
		}
	}
}

func (b *binder) bindModuleMethodBody(method *symbols.ModuleMethod) {
	glog.V(3).Infof("Binding module method '%v' body", method.Token())
	// Push a new activation frame and bind the body.
	scope := b.ctx.Scope.Push(true)
	defer scope.Pop()
	b.bindFunctionBody(method.Node)
}
