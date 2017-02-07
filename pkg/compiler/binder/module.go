// Copyright 2016 Marapongo, Inc. All rights reserved.

package binder

import (
	"github.com/golang/glog"

	"github.com/marapongo/mu/pkg/compiler/ast"
	"github.com/marapongo/mu/pkg/compiler/symbols"
)

// createModule simply creates a module symbol with its immutable information.  In particular, it doesn't yet bind
// members, since inter-module references must be resolved in later passes.
func (b *binder) createModule(node *ast.Module, parent *symbols.Package) *symbols.Module {
	glog.V(3).Infof("Creating package '%v' module '%v'", parent.Name(), node.Name.Ident)

	// Create the module symbol and register it.
	module := symbols.NewModuleSym(node, parent)
	b.ctx.RegisterSymbol(node, module)
	return module
}

// bindModuleImports binds module import tokens to their symbols.  This is done as a second pass just in case there are
// inter-module dependencies.
func (b *binder) bindModuleImports(module *symbols.Module) {
	// Now bind all imports to concrete symbols: these are simple token bindings.
	if module.Node.Imports != nil {
		for _, imptok := range *module.Node.Imports {
			if imp := b.bindModuleToken(imptok); imp != nil {
				module.Imports = append(module.Imports, imp)
			}
		}
	}
}

// bindModuleClasses binds a module's classes.  This must be done before binding variables and exports since they might
// mention classes by name, and so the symbolic information must have been registered beforehand.  Note that class
// method bodies are not yet bound during this pass, since they could reference module members not yet bound.
func (b *binder) bindModuleClasses(module *symbols.Module) {
	glog.V(3).Infof("Binding module '%v' classes", module.Token())

	// Set the current module in the context so we can e.g. enforce accessibility.
	priormodule := b.ctx.Currmodule
	b.ctx.Currmodule = module
	defer func() { b.ctx.Currmodule = priormodule }()

	// Now bind all class members.
	if module.Node.Members != nil {
		members := *module.Node.Members
		for _, nm := range ast.StableModuleMembers(members) {
			member := members[nm]
			if class, isclass := member.(*ast.Class); isclass {
				module.Members[nm] = b.bindClass(class, module)
			}
		}
	}
}

// bindModuleMembers binds a module's property and method members.  This must be done after binding classes, and before
// binding exports, so that any classes referenced are found (and because exports might refer to these).
func (b *binder) bindModuleMembers(module *symbols.Module) {
	glog.V(3).Infof("Binding module '%v' methods and properties", module.Token())

	// Set the current module in the context so we can e.g. enforce accessibility.
	priormodule := b.ctx.Currmodule
	b.ctx.Currmodule = module
	defer func() { b.ctx.Currmodule = priormodule }()

	// Now bind all module methods and properties.
	if module.Node.Members != nil {
		members := *module.Node.Members
		for _, nm := range ast.StableModuleMembers(members) {
			member := members[nm]
			if method, ismethod := member.(*ast.ModuleMethod); ismethod {
				module.Members[nm] = b.bindModuleMethod(method, module)
			} else if property, isproperty := member.(*ast.ModuleProperty); isproperty {
				module.Members[nm] = b.bindModuleProperty(property, module)
			}
		}
	}
}

// bindModuleExports binds a module's exports.  This must be done after binding classes and members, since an export
// might refer to those by symbolic token reference.  This can also safely be done before method bodies.
func (b *binder) bindModuleExports(module *symbols.Module) {
	glog.V(3).Infof("Binding module '%v' methods and properties", module.Token())

	// Set the current module in the context so we can e.g. enforce accessibility.
	priormodule := b.ctx.Currmodule
	b.ctx.Currmodule = module
	defer func() { b.ctx.Currmodule = priormodule }()

	// Now bind all module methods and properties.
	if module.Node.Members != nil {
		members := *module.Node.Members
		for _, nm := range ast.StableModuleMembers(members) {
			member := members[nm]
			if export, isexport := member.(*ast.Export); isexport {
				module.Members[nm] = b.bindExport(export, module)
			}
		}
	}
}

func (b *binder) bindExport(node *ast.Export, parent *symbols.Module) *symbols.Export {
	glog.V(3).Infof("Binding module '%v' export '%v'", parent.Name(), node.Name.Ident)

	// To bind an export, simply look up the referent symbol and associate this name with it.
	if refsym := b.lookupSymbolToken(node.Referent, node.Referent.Tok, true); refsym != nil {
		sym := symbols.NewExportSym(node, parent, refsym)
		b.ctx.RegisterSymbol(node, sym)
		return sym
	}
	return nil
}

func (b *binder) bindModuleProperty(node *ast.ModuleProperty, parent *symbols.Module) *symbols.ModuleProperty {
	glog.V(3).Infof("Binding module '%v' property '%v'", parent.Name(), node.Name.Ident)

	// Look up this node's type and inject it into the type table.
	ty := b.bindType(node.Type)
	sym := symbols.NewModulePropertySym(node, parent, ty)
	b.ctx.RegisterSymbol(node, sym)
	return sym
}

func (b *binder) bindModuleMethod(node *ast.ModuleMethod, parent *symbols.Module) *symbols.ModuleMethod {
	glog.V(3).Infof("Binding module '%v' method '%v'", parent.Name(), node.Name.Ident)

	// Make a function type out of this method and inject it into the type table.
	ty := b.bindFunctionType(node)
	sym := symbols.NewModuleMethodSym(node, parent, ty)
	b.ctx.RegisterSymbol(node, sym)

	// Note that we don't actually bind the body of this method yet.  Until we have gone ahead and injected *all*
	// top-level symbols into the type table, we would potentially encounter missing intra-module symbols.
	return sym
}

// bindModuleMethodBodies binds both the module's direct methods in addition to class methods.  This must be done after
// all top-level symbolic information has been bound, in case definitions, statements, and expressions depend upon them.
func (b *binder) bindModuleMethodBodies(module *symbols.Module) {
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
			b.bindClassMethodBodies(m)
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
