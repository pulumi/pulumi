package gen

import (
	"fmt"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

type outputType struct {
	schema.Type

	elementType schema.Type
}

func (t *outputType) String() string {
	return fmt.Sprintf("Output<%v>", t.elementType)
}

type typeDetails struct {
	optionalInputType bool

	inputTypes  []*schema.InputType
	outputTypes []*outputType
}

type genContext struct {
	tool          string
	pulumiPackage *schema.Package
	info          GoPackageInfo

	generateExtraTypes bool

	goPackages map[string]*pkgContext

	notedTypes codegen.StringSet
}

func newGenContext(tool string, pulumiPackage *schema.Package, info GoPackageInfo) *genContext {
	return &genContext{
		tool:               tool,
		pulumiPackage:      pulumiPackage,
		info:               info,
		generateExtraTypes: true,
		goPackages:         map[string]*pkgContext{},
		notedTypes:         codegen.StringSet{},
	}
}

func (ctx *genContext) getPackageForModule(mod string) *pkgContext {
	p, ok := ctx.goPackages[mod]
	if !ok {
		p = &pkgContext{
			ctx:                           ctx,
			pkg:                           ctx.pulumiPackage,
			mod:                           mod,
			importBasePath:                ctx.info.ImportBasePath,
			rootPackageName:               ctx.info.RootPackageName,
			typeDetails:                   map[string]*typeDetails{},
			names:                         codegen.NewStringSet(),
			schemaNames:                   codegen.NewStringSet(),
			renamed:                       map[string]string{},
			duplicateTokens:               map[string]bool{},
			functionNames:                 map[*schema.Function]string{},
			tool:                          ctx.tool,
			modToPkg:                      ctx.info.ModuleToPackage,
			pkgImportAliases:              ctx.info.PackageImportAliases,
			packages:                      ctx.goPackages,
			liftSingleValueMethodReturns:  ctx.info.LiftSingleValueMethodReturns,
			disableInputTypeRegistrations: ctx.info.DisableInputTypeRegistrations,
		}
		ctx.goPackages[mod] = p
	}
	return p
}

func (ctx *genContext) getPackageForToken(token string) *pkgContext {
	return ctx.getPackageForModule(tokenToPackage(ctx.pulumiPackage, ctx.info.ModuleToPackage, token))
}

func (ctx *genContext) getPackageForType(t schema.Type) *pkgContext {
	_, pkg := ctx.getRepresentativeTypeAndPackage(t)
	return pkg
}

func (ctx *genContext) getRepresentativeTypeAndPackage(t schema.Type) (schema.Type, *pkgContext) {
	switch t := t.(type) {
	case *outputType:
		return ctx.getRepresentativeTypeAndPackage(t.elementType)
	case *schema.InputType:
		return ctx.getRepresentativeTypeAndPackage(t.ElementType)
	case *schema.OptionalType:
		return ctx.getRepresentativeTypeAndPackage(t.ElementType)
	case *schema.ArrayType:
		return ctx.getRepresentativeTypeAndPackage(t.ElementType)
	case *schema.MapType:
		return ctx.getRepresentativeTypeAndPackage(t.ElementType)
	case *schema.ObjectType:
		if t.Package != ctx.pulumiPackage {
			return nil, nil
		}
		return t, ctx.getPackageForToken(t.Token)
	case *schema.EnumType:
		if t.Package != ctx.pulumiPackage {
			return nil, nil
		}
		return t, ctx.getPackageForToken(t.Token)
	case *schema.ResourceType:
		if t.Resource.Package != ctx.pulumiPackage {
			return nil, nil
		}
		return t, ctx.getPackageForToken(t.Token)
	case *schema.TokenType:
		return t, ctx.getPackageForToken(t.Token)
	default:
		return nil, nil
	}
}

func (ctx *genContext) inputType(elementType schema.Type) *schema.InputType {
	if obj, ok := elementType.(*schema.ObjectType); ok && !obj.IsInputShape() {
		elementType = obj.InputShape
	}
	return &schema.InputType{ElementType: elementType}
}

func (ctx *genContext) outputType(elementType schema.Type) *outputType {
	if obj, ok := elementType.(*schema.ObjectType); ok && !obj.IsPlainShape() {
		elementType = obj.PlainShape
	}
	return &outputType{elementType: elementType}
}

func (ctx *genContext) resourceType(resource *schema.Resource) *schema.ResourceType {
	return &schema.ResourceType{
		Token:    resource.Token,
		Resource: resource,
	}
}

func (ctx *genContext) noteType(t schema.Type) {
	if ctx.notedTypes.Has(t.String()) {
		return
	}
	ctx.notedTypes.Add(t.String())

	switch t := t.(type) {
	case *outputType:
		ctx.noteOutputType(t)
	case *schema.InputType:
		ctx.noteInputType(t)
	case *schema.OptionalType:
		ctx.noteOptionalType(t)
	case *schema.ArrayType:
		ctx.noteType(t.ElementType)
	case *schema.MapType:
		ctx.noteType(t.ElementType)
	case *schema.UnionType:
		ctx.noteUnionType(t)
	case *schema.ObjectType:
		if t.Package != ctx.pulumiPackage {
			return
		}
		ctx.noteObjectType(t)
	case *schema.EnumType:
		if t.Package != ctx.pulumiPackage || t.IsOverlay {
			return
		}
		pkg := ctx.getPackageForType(t)
		pkg.enums = append(pkg.enums, t)
	}
}

func (ctx *genContext) foldOptionalInputOutputType(t *schema.OptionalType) bool {
	// If the element type of the optional is nilable, we fold the optional and required input/output types together:
	// the required input/output type can already represent the lack of a value.
	return isNilType(t.ElementType)
}

func (ctx *genContext) noteOutputType(t *outputType) {
	// If the context has been configured to generate extra types, note the corresponding input type.
	if ctx.generateExtraTypes {
		ctx.noteType(codegen.InputType(t.elementType))
	}

	// For optional, array, map, and object output types, we need to note some additional
	// output types. In the former three cases, we need to note the output of the type's
	// element type so we can generate element accessors. In the latter case, we need to
	// note an output type per property type so we can generate property accessors.
	switch t := t.elementType.(type) {
	case *schema.OptionalType:
		ctx.noteType(ctx.outputType(t.ElementType))
		if ctx.foldOptionalInputOutputType(t) {
			return
		}
	case *schema.ArrayType:
		ctx.noteType(ctx.outputType(t.ElementType))
	case *schema.MapType:
		ctx.noteType(ctx.outputType(t.ElementType))
	case *schema.ObjectType:
		for _, p := range t.Properties {
			ctx.noteType(ctx.outputType(p.Type))
		}
	}

	if representativeType, pkg := ctx.getRepresentativeTypeAndPackage(t); pkg != nil {
		details := pkg.detailsForType(representativeType)
		details.outputTypes = append(details.outputTypes, t)
	}
}

func (ctx *genContext) noteInputType(t *schema.InputType) {
	if t, isOptional := t.ElementType.(*schema.OptionalType); isOptional {
		if ctx.foldOptionalInputOutputType(t) {
			ctx.noteType(ctx.inputType(t.ElementType))
			return
		}
		if pkg := ctx.getPackageForType(t.ElementType); pkg != nil {
			pkg.detailsForType(t.ElementType).optionalInputType = true
		}
	}

	ctx.noteType(t.ElementType)
	ctx.noteType(ctx.outputType(codegen.ResolvedType(t.ElementType)))
	if representativeType, pkg := ctx.getRepresentativeTypeAndPackage(t); pkg != nil {
		details := pkg.detailsForType(representativeType)
		details.inputTypes = append(details.inputTypes, t)
	}
}

func (ctx *genContext) noteOptionalType(t *schema.OptionalType) {
	// Go generates optional inputs as inputs of optionals.
	if input, ok := t.ElementType.(*schema.InputType); ok {
		ctx.noteType(&schema.InputType{
			ElementType: &schema.OptionalType{
				ElementType: input.ElementType,
			},
		})
		return
	}

	ctx.noteType(t.ElementType)
}

func (ctx *genContext) noteUnionType(t *schema.UnionType) {
	for _, t := range t.ElementTypes {
		ctx.noteType(t)
	}
}

func (ctx *genContext) noteObjectType(t *schema.ObjectType) {
	if !t.IsInputShape() {
		pkg := ctx.getPackageForType(t)
		pkg.types = append(pkg.types, t)
	}
	ctx.notePropertyTypes(t.Properties)
}

func (ctx *genContext) notePropertyTypes(props []*schema.Property) {
	for _, p := range props {
		ctx.noteType(p.Type)
	}
}

func (ctx *genContext) noteOutputPropertyTypes(props []*schema.Property) {
	for _, p := range props {
		ctx.noteType(ctx.outputType(p.Type))
	}
}
