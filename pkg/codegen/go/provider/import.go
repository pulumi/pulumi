package provider

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/hashicorp/hcl/v2"
)

func isInvalidType(typ types.Type) bool {
	return typ == nil || typ == types.Typ[types.Invalid]
}

func getMethod(typ types.Type, name string) (*types.Func, bool) {
	methodSet := types.NewMethodSet(typ)
	for i := 0; i < methodSet.Len(); i++ {
		m := methodSet.At(i).Obj().(*types.Func)
		if m.Name() == name {
			return m, true
		}
	}
	return nil, false
}

func findPackage(path string, imports []*types.Package) (*types.Package, bool) {
	for _, p := range imports {
		if p.Path() == path {
			return p, true
		}
	}
	return nil, false
}

func (m *pulumiModule) importPulumiSDK() (*pulumiSDK, hcl.Diagnostics) {
	if m.pulumiSDK != nil {
		if m.pulumiSDK.types == nil {
			return nil, nil
		}
		return m.pulumiSDK, nil
	}

	packageName := m.goPackage.Syntax[0].Name

	sdk, ok := findPackage("github.com/pulumi/pulumi/sdk/v2/go/pulumi", m.goPackage.Types.Imports())
	if !ok {
		m.pulumiSDK = &pulumiSDK{}
		return nil, hcl.Diagnostics{m.errorf(packageName, "package %v does not import the Pulumi SDK", packageName.Name)}
	}

	var diags hcl.Diagnostics
	getType := func(name string) types.Type {
		typ := sdk.Scope().Lookup(name)
		if typ == nil {
			diags = append(diags, m.errorf(packageName, "Pulumi SDK %v does not define the %v type", sdk.Path(), name))
			return nil
		}
		return typ.Type()
	}

	m.pulumiSDK = &pulumiSDK{
		types:             sdk,
		context:           getType("Context"),
		resourceOption:    getType("ResourceOption"),
		componentResource: getType("ComponentResource"),
		input:             getType("Input"),
		output:            getType("Output"),
	}
	if diags.HasErrors() {
		m.pulumiSDK.types = nil
	}
	return m.pulumiSDK, diags
}

func (m *pulumiModule) importProviderSDK() (*providerSDK, hcl.Diagnostics) {
	if m.providerSDK != nil {
		if m.providerSDK.types == nil {
			return nil, nil
		}
		return m.providerSDK, nil
	}

	packageName := m.goPackage.Syntax[0].Name

	sdk, ok := findPackage("github.com/pulumi/pulumi/sdk/v2/go/x/provider", m.goPackage.Types.Imports())
	if !ok {
		m.providerSDK = &providerSDK{}
		return nil, hcl.Diagnostics{m.errorf(packageName, "package %v does not import the Pulumi resource SDK", packageName.Name)}
	}

	var diags hcl.Diagnostics
	getType := func(name string) types.Type {
		typ := sdk.Scope().Lookup(name)
		if typ == nil {
			diags = append(diags, m.errorf(packageName, "Pulumi resource SDK %v does not define the %v type", sdk.Path(), name))
			return nil
		}
		return typ.Type()
	}

	m.providerSDK = &providerSDK{
		types:         sdk,
		id:            getType("ID"),
		context:       getType("Context"),
		createOptions: getType("CreateOptions"),
		readOptions:   getType("ReadOptions"),
		updateOptions: getType("UpdateOptions"),
		deleteOptions: getType("DeleteOptions"),
		callOptions:   getType("CallOptions"),
	}
	if diags.HasErrors() {
		m.providerSDK.types = nil
	}
	return m.providerSDK, diags
}

func (m *pulumiModule) markType(typ types.Type) {
	switch typ := typ.(type) {
	case *types.Array:
		m.markType(typ.Elem())
	case *types.Slice:
		m.markType(typ.Elem())
	case *types.Map:
		m.markType(typ.Elem())
	case *types.Pointer:
		m.markType(typ.Elem())
	case *types.Named:
		if output, ok := m.pulumiSDK.hasOutputType(typ); ok {
			m.pulumiPackage.outputTypes.set(output, output)
		}
		if m.pulumiPackage.roots.has(typ.Obj().Pkg()) {
			m.pulumiPackage.typeClosure.add(typ)
		}
		m.markType(typ.Underlying())
	case *types.Struct:
		for i := 0; i < typ.NumFields(); i++ {
			m.markType(typ.Field(i).Type())
		}
	}
}

func (m *pulumiModule) markTopLevelType(typ types.Type) {
	if ptr, ok := typ.(*types.Pointer); ok {
		typ = ptr.Elem()
	}
	namedType, ok := typ.(*types.Named)
	if !ok {
		return
	}
	structType, ok := namedType.Underlying().(*types.Struct)
	if !ok {
		return
	}
	for i := 0; i < structType.NumFields(); i++ {
		m.markType(structType.Field(i).Type())
	}
}

func (m *pulumiModule) checkComponentResourceFields(syntax *ast.TypeSpec, implements types.Type, kind string) hcl.Diagnostics {
	structType := syntax.Type.(*ast.StructType)

	var diags hcl.Diagnostics
	for _, field := range structType.Fields.List {
		fieldName := ""
		if len(field.Names) != 0 {
			fieldName = field.Names[len(field.Names)-1].Name
		}

		// Ignore untagged fields.
		_, ok := getPulumiPropertyDesc(field)
		if !ok {
			continue
		}

		fieldType, ok := m.goPackage.TypesInfo.Types[field.Type]
		if !ok {
			diags = append(diags, m.errorf(field, "internal error: no type information for %v", fieldName))
			continue
		}

		if !types.ConvertibleTo(fieldType.Type, implements) {
			diags = append(diags, m.errorf(field, "component resource %v %v does not implement %v", kind, fieldName, implements))
		}
	}
	return diags
}

func (m *pulumiModule) checkComponentResources() hcl.Diagnostics {
	sdk, sdkDiags := m.importPulumiSDK()
	if sdkDiags.HasErrors() || sdk == nil {
		return sdkDiags
	}

	var diags hcl.Diagnostics
	for _, r := range m.resources {
		if !r.isComponent {
			continue
		}

		if !isInvalidType(r.args) {
			namedType := r.args.(*types.Pointer).Elem().(*types.Named)
			syntax := m.paramsTypes.get(namedType)
			if syntax == nil {
				diags = append(diags, m.errorf(r.syntax, "could not find a definition for %v in %v", namedType, m.goPackage))
			} else {
				checkDiags := m.checkComponentResourceFields(syntax.(*ast.TypeSpec), sdk.input, "input")
				diags = append(diags, checkDiags...)
			}
		}

		checkDiags := m.checkComponentResourceFields(r.syntax, sdk.output, "output")
		diags = append(diags, checkDiags...)
	}
	return diags
}

func (m *pulumiModule) gatherMarkedTypesInDecl(decl *ast.GenDecl) hcl.Diagnostics {
	var diags hcl.Diagnostics
	for _, spec := range decl.Specs {
		typeSpec := spec.(*ast.TypeSpec)

		typeDef, ok := m.goPackage.TypesInfo.Defs[typeSpec.Name]
		if !ok {
			diags = append(diags, m.errorf(typeSpec.Name, "internal error: no type information for %v", typeSpec.Name.Name))
			continue
		}

		doc := typeSpec.Doc
		if len(decl.Specs) == 1 && doc == nil {
			doc = decl.Doc
		}

		namedType := typeDef.Type().(*types.Named)
		switch {
		case m.paramsTypes.has(namedType):
			m.paramsTypes.set(namedType, typeSpec)
		case m.componentTypes.has(namedType):
			resource := m.componentTypes.get(namedType).(*pulumiResource)
			resource.token = m.getToken(typeSpec.Name, pascalCase)
			resource.doc = doc
			resource.syntax = typeSpec
		case m.pulumiPackage.typeClosure.has(namedType):
			if _, isStruct := typeSpec.Type.(*ast.StructType); !isStruct {
				break
			}

			pulumiType := &pulumiType{
				token:  m.getToken(typeSpec.Name, pascalCase),
				doc:    doc,
				syntax: typeSpec,
				typ:    namedType,
			}
			m.types = append(m.types, pulumiType)
			m.pulumiPackage.typeClosure.set(namedType, pulumiType)
		}
	}
	return diags
}

func (m *pulumiModule) gatherMarkedTypes() hcl.Diagnostics {
	var diags hcl.Diagnostics
	for _, f := range m.goPackage.Syntax {
		for _, decl := range f.Decls {
			if decl, ok := decl.(*ast.GenDecl); ok && decl.Tok == token.TYPE {
				declDiags := m.gatherMarkedTypesInDecl(decl)
				diags = append(diags, declDiags...)
			}
		}
	}
	return diags
}

func (m *pulumiModule) newParam(name string, t types.Type) *types.Var {
	return types.NewParam(token.NoPos, m.goPackage.Types, name, t)
}

func (m *pulumiModule) newFunc(name string, parameters, results *types.Tuple) *types.Func {
	sig := types.NewSignature(nil, parameters, results, false)
	return types.NewFunc(token.NoPos, m.goPackage.Types, name, sig)
}

func (m *pulumiModule) extractParamsType(memberName *ast.Ident, params *types.Tuple, index int, kind string) (types.Type, hcl.Diagnostics) {
	if params.Len() < index+1 {
		// The signature mismatch will be handled later.
		return types.Typ[types.Invalid], nil
	}

	// Args type must be a pointer to a named type that has an underlying struct type.
	paramType := params.At(index).Type()
	ptrType, isPtrType := paramType.(*types.Pointer)
	if !isPtrType {
		return paramType, hcl.Diagnostics{m.errorf(memberName, "the %v type for %v must be *T, where T is `type T struct{}`", kind, memberName.Name)}
	}
	namedType, isNamedType := ptrType.Elem().(*types.Named)
	if !isNamedType {
		return paramType, hcl.Diagnostics{m.errorf(memberName, "the %v type for %v must be *T, where T is `type T struct{}`", kind, memberName.Name)}
	}
	if _, isStructType := namedType.Underlying().(*types.Struct); !isStructType {
		return paramType, hcl.Diagnostics{m.errorf(memberName, "the %v type for %v must be *T, where T is `type T struct{}`", kind, memberName.Name)}
	}
	m.paramsTypes.add(namedType)
	return paramType, nil
}

// A provider must be of type T s.t. T's underlying type is a struct or a pointer to a struct, and T or *T
// implements the following interface:
//
//     interface {
//         Configure(ctx *provider.Context, args ArgsType, options OptionsType) error
//     }
//
// OptionsType is a type from the Pulumi provider SDK (NYI).
func (m *pulumiModule) importProvider(provider *pulumiResource) hcl.Diagnostics {
	providerName := provider.syntax.Name

	typeDef, ok := m.goPackage.TypesInfo.Defs[providerName]
	if !ok {
		return hcl.Diagnostics{m.errorf(providerName, "internal error: no type information for %v", providerName.Name)}
	}

	sdk, sdkDiags := m.importProviderSDK()
	if sdkDiags.HasErrors() {
		return sdkDiags
	}

	namedType := typeDef.Type().(*types.Named)
	if _, ok := namedType.Underlying().(*types.Struct); !ok {
		return hcl.Diagnostics{m.errorf(providerName, "the underlying type for provider %v must be a struct type", providerName.Name)}
	}
	provider.typ = types.NewPointer(namedType)

	methodSet := types.NewMethodSet(provider.typ)

	configure := methodSet.Lookup(nil, "Configure")
	if configure == nil {
		return hcl.Diagnostics{m.errorf(providerName, "provider %v is missing a Configure method", providerName.Name)}
	}

	optionsType := types.NewInterface(nil, nil)
	errorType := types.Universe.Lookup("error").Type()

	// Pull the args type from the configure signature.
	argsType, argsDiags := m.extractParamsType(providerName, configure.Type().(*types.Signature).Params(), 1, "args")
	diags := append(hcl.Diagnostics{}, argsDiags...)
	provider.args = argsType

	// Check that the provider type implements the appropriate interface.
	providerInterface := types.NewInterface([]*types.Func{
		m.newFunc("Configure",
			types.NewTuple(
				m.newParam("context", types.NewPointer(sdk.context)),
				m.newParam("args", argsType),
				m.newParam("options", optionsType)),
			types.NewTuple(
				m.newParam("err", errorType))),
	}, nil).Complete()

	missing, wrongType := types.MissingMethod(provider.typ, providerInterface, true)
	if missing != nil {
		if wrongType {
			actual := methodSet.Lookup(missing.Pkg(), missing.Name())
			diags = append(diags, m.errorf(providerName, "%v has the wrong type: expected %v, got %v", providerName.Name, actual.Obj(), missing))
		} else {
			diags = append(diags, m.errorf(provider.syntax.Name, "provider %v is missing method %v", providerName.Name, missing))
		}
	}

	m.markTopLevelType(provider.args)

	return diags
}

// A resource Foo must be of type T s.t. T's underlying type is a struct or a pointer to a struct, and T or *T
// implements the following interface:
//
//     interface {
//         Args() *ArgsType
//         Create(ctx *provider.Context, provider *ProviderType, args *ArgsType, options provider.CreateOptions) (id provider.ID, err error)
//         Read(ctx *provider.Context, provider *ProviderType, id provider.ID, options provider.ReadOptions) error
//         Update(ctx *provider.Context, provider *ProviderType, id provider.ID, args *ArgsType, options provider.UpdateOptions) error
//         Delete(ctx *provider.Context, provider *ProviderType, id provider.ID, options provider.DeleteOptions) error
//     }
//
// ProviderType is the provider type for the package. ArgsType is determined by the type of the second parameters
// to the Create method. It is an error if no Create method is present.
func (m *pulumiModule) importResource(resource *pulumiResource) hcl.Diagnostics {
	resourceName := resource.syntax.Name

	typeDef, ok := m.goPackage.TypesInfo.Defs[resourceName]
	if !ok {
		return hcl.Diagnostics{m.errorf(resourceName, "internal error: no type information for %v", resourceName.Name)}
	}

	sdk, sdkDiags := m.importProviderSDK()
	if sdkDiags.HasErrors() {
		return sdkDiags
	}

	namedType := typeDef.Type().(*types.Named)
	if _, ok := namedType.Underlying().(*types.Struct); !ok {
		return hcl.Diagnostics{m.errorf(resourceName, "the underlying type for resource %v must be a struct type", resourceName.Name)}
	}
	resource.typ = types.NewPointer(namedType)

	methodSet := types.NewMethodSet(resource.typ)

	var argsSig *types.Signature
	if argsFunc := methodSet.Lookup(nil, "Args"); argsFunc != nil {
		argsSig = argsFunc.Type().(*types.Signature)
	} else {
		return hcl.Diagnostics{m.errorf(resourceName, "resource %v is missing an Args method", resourceName.Name)}
	}

	errorType := types.Universe.Lookup("error").Type()
	providerType := m.pulumiPackage.provider.typ

	// Pull the args type from the Args signature.
	argsType, argsDiags := m.extractParamsType(resourceName, argsSig.Results(), 0, "args")
	diags := append(hcl.Diagnostics{}, argsDiags...)
	resource.args = argsType

	// Check that the resource type implements the appropriate interface.
	resourceInterface := types.NewInterface([]*types.Func{
		m.newFunc("Args",
			types.NewTuple(),
			types.NewTuple(m.newParam("args", argsType))),
		m.newFunc("Create",
			types.NewTuple(
				m.newParam("context", types.NewPointer(sdk.context)),
				m.newParam("provider", providerType),
				m.newParam("args", argsType),
				m.newParam("options", sdk.createOptions)),
			types.NewTuple(
				m.newParam("id", sdk.id),
				m.newParam("err", errorType))),
		m.newFunc("Read",
			types.NewTuple(
				m.newParam("context", types.NewPointer(sdk.context)),
				m.newParam("provider", providerType),
				m.newParam("id", sdk.id),
				m.newParam("options", sdk.readOptions)),
			types.NewTuple(m.newParam("err", errorType))),
		m.newFunc("Update",
			types.NewTuple(
				m.newParam("context", types.NewPointer(sdk.context)),
				m.newParam("provider", providerType),
				m.newParam("id", sdk.id),
				m.newParam("args", argsType),
				m.newParam("options", sdk.updateOptions)),
			types.NewTuple(m.newParam("err", errorType))),
		m.newFunc("Delete",
			types.NewTuple(
				m.newParam("context", types.NewPointer(sdk.context)),
				m.newParam("provider", providerType),
				m.newParam("id", sdk.id),
				m.newParam("options", sdk.deleteOptions)),
			types.NewTuple(m.newParam("err", errorType))),
	}, nil).Complete()

	missing, wrongType := types.MissingMethod(resource.typ, resourceInterface, true)
	if missing != nil {
		if wrongType {
			actual := methodSet.Lookup(missing.Pkg(), missing.Name())
			diags = append(diags, m.errorf(resourceName, "%v has the wrong type: expected %v, got %v", resourceName.Name, actual.Obj(), missing))
		} else {
			diags = append(diags, m.errorf(resource.syntax.Name, "resource %v is missing method %v", resourceName.Name, missing))
		}
	}

	m.markTopLevelType(resource.typ)
	m.markTopLevelType(resource.args)

	return diags
}

// A constructor must have the signature
//
//     func (ctx *pulumi.Context, name string, args *ArgsType, options ...pulumi.ResourceOption) (*ResourceType, error)
//
// ResourceType must be assignable to pulumi.Resource. ArgsType must be a struct where all fields are inputs.
func (m *pulumiModule) importConstructor(constructor *pulumiFunction) hcl.Diagnostics {
	constructorName := constructor.syntax.Name

	typ, ok := m.goPackage.TypesInfo.Defs[constructor.syntax.Name]
	if !ok {
		return hcl.Diagnostics{m.errorf(constructorName, "internal error: no type information for %v", constructorName.Name)}
	}

	sdk, sdkDiags := m.importPulumiSDK()
	if sdkDiags.HasErrors() {
		return sdkDiags
	}

	var diags hcl.Diagnostics

	errorType := types.Universe.Lookup("error").Type()

	signature := typ.(*types.Func).Type().(*types.Signature)

	// Pull the args type from the constructor signature.
	argsType, argsDiags := m.extractParamsType(constructorName, signature.Params(), 2, "args")
	diags = append(diags, argsDiags...)

	// Pull the resource type from the constructor signature.
	resourceType, resourceDiags := m.extractParamsType(constructorName, signature.Results(), 0, "resource")
	diags = append(diags, resourceDiags...)

	// Check the function signature.
	expectedSig := types.NewSignature(nil,
		types.NewTuple(
			m.newParam("context", types.NewPointer(sdk.context)),
			m.newParam("name", types.Typ[types.String]),
			m.newParam("args", argsType),
			m.newParam("options", types.NewSlice(sdk.resourceOption))),
		types.NewTuple(
			m.newParam("resource", resourceType),
			m.newParam("err", errorType)),
		true)
	expected := types.NewFunc(token.NoPos, m.goPackage.Types, constructorName.Name+"Func", expectedSig)

	if !types.ConvertibleTo(signature, expected.Type()) {
		diags = append(diags, m.errorf(constructorName, "%v has the wrong signature: expected %v, got %v", constructorName.Name, expected, signature))
	}

	// Validate the resource type.
	if !isInvalidType(resourceType) {
		// Get at the resource's named type. extractParamsType will have ensured that this is safe.
		namedType := resourceType.(*types.Pointer).Elem().(*types.Named)

		// extractParamsType sticks its result in the paramsType map. Remove the resourceType from that map now, as it
		// isn't actually a params type.
		m.paramsTypes.delete(namedType)

		if !types.ConvertibleTo(resourceType, sdk.componentResource) {
			result := constructor.syntax.Type.Results.List[0]
			diags = append(diags, m.errorf(result, "the first result of a constructor must be convertible to %v", sdk.componentResource))
		} else {
			// syntactical information will be filled in by gatherMarkedTypesInDecl.
			resource := &pulumiResource{
				isComponent: true,
				args:        argsType,
				typ:         resourceType,
			}
			m.resources = append(m.resources, resource)
			m.componentTypes.set(namedType, resource)
		}
	}

	m.markTopLevelType(argsType)
	m.markTopLevelType(resourceType)

	return diags
}

// A function must have the signature
//
//     func (ctx *provider.Context, provider ProviderType, args *ArgsType, options provider.CallOptions) (*ResultType, error)
//
func (m *pulumiModule) importFunction(function *pulumiFunction) hcl.Diagnostics {
	functionName := function.syntax.Name

	typ, ok := m.goPackage.TypesInfo.Defs[function.syntax.Name]
	if !ok {
		return hcl.Diagnostics{m.errorf(functionName, "internal error: no type information for %v", functionName.Name)}
	}

	sdk, sdkDiags := m.importProviderSDK()
	if sdkDiags.HasErrors() {
		return sdkDiags
	}

	signature := typ.(*types.Func).Type().(*types.Signature)

	errorType := types.Universe.Lookup("error").Type()
	providerType := m.pulumiPackage.provider.typ

	// Pull the args and result types from the create signature.
	var diags hcl.Diagnostics
	argsType, argsDiags := m.extractParamsType(functionName, signature.Params(), 2, "args")
	diags = append(diags, argsDiags...)

	resultType, resultDiags := m.extractParamsType(functionName, signature.Results(), 0, "result")
	diags = append(diags, resultDiags...)

	function.args, function.result = argsType, resultType

	// Check the function signature.
	expected := m.newFunc(functionName.Name+"Func",
		types.NewTuple(
			m.newParam("context", types.NewPointer(sdk.context)),
			m.newParam("provider", providerType),
			m.newParam("args", argsType),
			m.newParam("options", sdk.callOptions)),
		types.NewTuple(
			m.newParam("result", resultType),
			m.newParam("err", errorType)))

	if !types.ConvertibleTo(signature, expected.Type()) {
		diags = append(diags, m.errorf(functionName, "%v has the wrong signature: expected %v, got %v", functionName.Name, expected, signature))
	}

	m.markTopLevelType(function.args)
	m.markTopLevelType(function.result)

	return diags
}

func (m *pulumiModule) importModule() hcl.Diagnostics {
	var diags hcl.Diagnostics
	for _, r := range m.resources {
		resourceDiags := m.importResource(r)
		diags = append(diags, resourceDiags...)
	}
	for _, c := range m.constructors {
		constructorDiags := m.importConstructor(c)
		diags = append(diags, constructorDiags...)
	}
	for _, f := range m.functions {
		functionDiags := m.importFunction(f)
		diags = append(diags, functionDiags...)
	}
	return diags
}

func (p *pulumiPackage) importProviderType() hcl.Diagnostics {
	return p.providerModule.importProvider(p.provider)
}
