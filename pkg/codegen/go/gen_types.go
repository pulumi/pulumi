package gen

import (
	"fmt"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

// outputType represents an `output(T)`. This type is used as part of the logic in noteType to distinguish between
// `T` and `output(T)`.
type outputType struct {
	schema.Type

	elementType schema.Type
}

func (t *outputType) String() string {
	return fmt.Sprintf("Output<%v>", t.elementType)
}

// typeDetails tracks the input and output types associated with a resource, object, or enum type defined in a Pulumi
// package. This includes includes inputs and outputs that use constructed types (optionals, arrays, and maps) that
// reference the resource, object, or enum type. For example, the details for an object type `example::Foo` might
// contain the input types `input(example::Foo)`, `input(array(input(example::Foo)))`, and
// `input(optional(example::Foo))`, and the output types `output(example::Foo)`, `ouptut(array(example::Foo))`, and
// `output(optional(example::Foo))`.
type typeDetails struct {
	optionalInputType bool

	inputTypes  []*schema.InputType
	outputTypes []*outputType
}

// genContext provides a common context for storing information about an input Pulumi package and its output Go SDK.
type genContext struct {
	tool          string
	pulumiPackage *schema.Package
	info          GoPackageInfo

	generateExtraInputTypes bool

	goPackages map[string]*pkgContext

	// notedTypes tracks the set of schema types seen by genContext.noteType in order to ensure that we don't record the
	// same type more than once in a resource, object, or enum type's type details.
	notedTypes codegen.StringSet
}

func newGenContext(tool string, pulumiPackage *schema.Package, info GoPackageInfo) *genContext {
	return &genContext{
		tool:                    tool,
		pulumiPackage:           pulumiPackage,
		info:                    info,
		generateExtraInputTypes: info.GenerateExtraInputTypes,
		goPackages:              map[string]*pkgContext{},
		notedTypes:              codegen.StringSet{},
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

// getRepresentativeTypeAndPackage returns the representative type (if any) for a particular schema type. The
// representative type is the object, enum, or resource type referenced by the input type. For constructed types--
// outputs, inputs, optionals, arrays, and maps--this is defined recursively as the representative type of the
// element type. For objects, enums, and resources, this is the type itself. Otherwise, there is no representative
// type and package. Representative types that are defined in other Pulumi packages are ignored.
//
// This method is used to associate input and output types with any object, enum, or resource type they reference.
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

// outputType creates a reference to `output(T)`. If `T` is the input shape of an object type, it is replaced with
// its plain shape.
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

// noteType records the usage of a particular schema type. This function is used in particular to calculate the set of
// input and output types that are used by the input Pulumi package, which drives the set of input and output types we
// need to generate in the output SDK.
//
// The mapping from schema type to Go type has a few special cases around optionals and inputs:
//
// - given optional(T), if the Go representation of T is nilable, then we just generate T. Otherwise, we generate *T.
// - optional(input(T)) and input(optional(T)) generate the same type.
// - by the two rules above, input(optional(T)) and input(T) generate the same type if the Go representation of T is
//   nilable.
//
// Each noted input or output type must be associated with a particular resource, object, or enum type defined in the
// input Pulumi package in order to be present in the output SDK. Input and output types that are associated with types
// defined in other Pulumi packages must be present in the SDKs for their associated types, and we instead generate
// references to those definitions. For example, the type `input(array(input(string)))` is associated with the core
// Pulumi SDK, and will generate a reference to `pulumi.StringArrayInput`, and the type
// `input(map(input(resource(aws:s3/bucket:Bucket))))` is associated with the `aws` Pulumi package, and will generate
// a reference to `s3.BucketInput`.
//
// Because the generated code for each input type depends on the existence of its corresponding output type, noting
// an input type will also note its output type.
//
// If the package has been configured to generate extra input types, we always note input(T) for T if T is not already
// an input or output type.
func (ctx *genContext) noteType(t schema.Type) {
	if ctx.notedTypes.Has(t.String()) {
		return
	}
	ctx.notedTypes.Add(t.String())

	isInputOrOutputType := false
	switch t := t.(type) {
	case *outputType:
		ctx.noteOutputType(t)
		isInputOrOutputType = true
	case *schema.InputType:
		ctx.noteInputType(t)
		isInputOrOutputType = true
	case *schema.OptionalType:
		ctx.noteOptionalType(t)
		_, isInputOrOutputType = t.ElementType.(*schema.InputType)
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

	if !isInputOrOutputType && ctx.generateExtraInputTypes {
		ctx.noteType(codegen.InputType(t))
	}
}

func (ctx *genContext) foldOptionalInputOutputType(t *schema.OptionalType) bool {
	// If the element type of the optional is nilable, we fold the optional and required input/output types together:
	// the required input/output type can already represent the lack of a value.
	return isNilType(t.ElementType)
}

func (ctx *genContext) noteOutputType(t *outputType) {
	// If the context has been configured to generate extra types, note the corresponding input type.
	if ctx.generateExtraInputTypes {
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
			ctx.noteType(codegen.InputType(t.ElementType))
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
