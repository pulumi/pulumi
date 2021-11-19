//nolint:lll
package testing

import (
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

// A StackContext provides context for generating URNs and references to resources.
type StackContext struct {
	projectName string
	stackName   string
	resources   []*resource.State
}

// NewStackContext creates a new stack context with the given project name, stack name, and list of resources.
func NewStackContext(projectName, stackName string, resources ...*resource.State) *StackContext {
	ctx := &StackContext{projectName: projectName, stackName: stackName}
	return ctx.Append(resources...)
}

// ProjectName returns the context's project name.
func (ctx *StackContext) ProjectName() string {
	return ctx.projectName
}

// StackName returns the context's stack name.
func (ctx *StackContext) StackName() string {
	return ctx.stackName
}

// Resources returns the context's resources.
func (ctx *StackContext) Resources() []*resource.State {
	return ctx.resources
}

// Append creates a new context that contains the current context's resources and the given list of resources.
func (ctx *StackContext) Append(r ...*resource.State) *StackContext {
	rs := make([]*resource.State, len(ctx.resources)+len(r))
	copy(rs, ctx.resources)
	copy(rs[len(ctx.resources):], r)
	return &StackContext{ctx.projectName, ctx.stackName, rs}
}

// URNGenerator generates URNs that are valid within the context (i.e. the project name and stack name portions of the
// generated URNs will always be taken from the context).
func (ctx *StackContext) URNGenerator() *rapid.Generator {
	return urnGenerator(ctx)
}

// URNSampler samples URNs from the stack's resources.
func (ctx *StackContext) URNSampler() *rapid.Generator {
	return rapid.Custom(func(t *rapid.T) resource.URN {
		return rapid.SampledFrom(ctx.Resources()).Draw(t, "referenced resource").(*resource.State).URN
	})
}

// ResourceReferenceGenerator generates resource.ResourceReference values. The referenced resource is
func (ctx *StackContext) ResourceReferenceGenerator() *rapid.Generator {
	if len(ctx.Resources()) == 0 {
		panic("cannot generate resource references: stack context has no resources")
	}
	return resourceReferenceGenerator(ctx)
}

// ResourceReferencePropertyGenerator generates resource reference resource.PropertyValues.
func (ctx *StackContext) ResourceReferencePropertyGenerator() *rapid.Generator {
	return resourceReferencePropertyGenerator(ctx)
}

// ArrayPropertyGenerator generates array resource.PropertyValues. The
// maxDepth parameter controls the maximum depth of the elements of
// the array.
func (ctx *StackContext) ArrayPropertyGenerator(maxDepth int) *rapid.Generator {
	return arrayPropertyGenerator(ctx.PropertyValueGenerator(maxDepth - 1))
}

// PropertyMapGenerator generates resource.PropertyMap values. The
// maxDepth parameter controls the maximum depth of the elements of
// the map.
func (ctx *StackContext) PropertyMapGenerator(maxDepth int) *rapid.Generator {
	return propertyMapGenerator(ctx.PropertyValueGenerator(maxDepth))
}

// ObjectPropertyGenerator generates object resource.PropertyValues.
// The maxDepth parameter controls the maximum depth of the elements
// of the object.
func (ctx *StackContext) ObjectPropertyGenerator(maxDepth int) *rapid.Generator {
	return objectPropertyGenerator(ctx.PropertyValueGenerator(maxDepth - 1))
}

// OutputPropertyGenerator generates output resource.PropertyValues.
// The maxDepth parameter controls the maximum depth of the resolved
// value of the output, if any. The output's dependencies will only
// refer to resources in the context.
func (ctx *StackContext) OutputPropertyGenerator(maxDepth int) *rapid.Generator {
	return outputPropertyGenerator(ctx, ctx.PropertyValueGenerator(maxDepth-1))
}

// SecretPropertyGenerator generates secret resource.PropertyValues.
// The maxDepth parameter controls the maximum depth of the plaintext
// value of the secret, if any.
func (ctx *StackContext) SecretPropertyGenerator(maxDepth int) *rapid.Generator {
	return secretPropertyGenerator(ctx.PropertyValueGenerator(maxDepth - 1))
}

// PropertyValueGenerator generates arbitrary resource.PropertyValues.
// The maxDepth parameter controls the maximum number of times the
// generator may recur.
func (ctx *StackContext) PropertyValueGenerator(maxDepth int) *rapid.Generator {
	return propertyValueGenerator(ctx, maxDepth)
}

// TypeGenerator generates legal tokens.Type values.
func TypeGenerator() *rapid.Generator {
	return rapid.Custom(func(t *rapid.T) tokens.Type {
		return tokens.Type(rapid.StringMatching(`^[a-zA-Z][-a-zA-Z0-9_]*:([^0-9][a-zA-Z0-9._/]*)?:[^0-9][a-zA-Z0-9._/]*$`).Draw(t, "type token").(string))
	})
}

// URNGenerator generates legal resource.URN values.
func URNGenerator() *rapid.Generator {
	return urnGenerator(nil)
}

func urnGenerator(ctx *StackContext) *rapid.Generator {
	var stackNameGenerator, projectNameGenerator *rapid.Generator
	if ctx == nil {
		stackNameGenerator = rapid.StringMatching(`^((:[^:])[^:]*)*:?$`)
		projectNameGenerator = rapid.StringMatching(`^((:[^:])[^:]*)*:?$`)
	} else {
		stackNameGenerator = rapid.Just(ctx.StackName())
		projectNameGenerator = rapid.Just(ctx.ProjectName())
	}

	return rapid.Custom(func(t *rapid.T) resource.URN {
		stackName := tokens.QName(stackNameGenerator.Draw(t, "stack name").(string))
		projectName := tokens.PackageName(projectNameGenerator.Draw(t, "project name").(string))
		parentType := TypeGenerator().Draw(t, "parent type").(tokens.Type)
		resourceType := TypeGenerator().Draw(t, "resource type").(tokens.Type)
		resourceName := tokens.QName(rapid.StringMatching(`^((:[^:])[^:]*)*:?$`).Draw(t, "resource name").(string))
		return resource.NewURN(stackName, projectName, parentType, resourceType, resourceName)
	})
}

// IDGenerator generates legal resource.ID values.
func IDGenerator() *rapid.Generator {
	return rapid.Custom(func(t *rapid.T) resource.ID {
		return resource.ID(rapid.StringMatching(`..*`).Draw(t, "ids").(string))
	})
}

// SemverStringGenerator generates legal semver strings.
func SemverStringGenerator() *rapid.Generator {
	return rapid.StringMatching(`^v?(?P<major>0|[1-9]\d*)\.(?P<minor>0|[1-9]\d*)\.(?P<patch>0|[1-9]\d*)(?:-(?P<prerelease>(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+(?P<buildmetadata>[0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`)
}

// UnknownPropertyGenerator generates the unknown resource.PropertyValue.
func UnknownPropertyGenerator() *rapid.Generator {
	return rapid.Custom(func(t *rapid.T) resource.PropertyValue {
		return rapid.Just(resource.MakeComputed(resource.NewStringProperty(""))).Draw(t, "unknowns").(resource.PropertyValue)
	})
}

// NullPropertyGenerator generates the null resource.PropertyValue.
func NullPropertyGenerator() *rapid.Generator {
	return rapid.Custom(func(t *rapid.T) resource.PropertyValue {
		return rapid.Just(resource.NewNullProperty()).Draw(t, "nulls").(resource.PropertyValue)
	})
}

// BoolPropertyGenerator generates boolean resource.PropertyValues.
func BoolPropertyGenerator() *rapid.Generator {
	return rapid.Custom(func(t *rapid.T) resource.PropertyValue {
		return resource.NewBoolProperty(rapid.Bool().Draw(t, "booleans").(bool))
	})
}

// NumberPropertyGenerator generates numeric resource.PropertyValues.
func NumberPropertyGenerator() *rapid.Generator {
	return rapid.Custom(func(t *rapid.T) resource.PropertyValue {
		return resource.NewNumberProperty(rapid.Float64().Draw(t, "numbers").(float64))
	})
}

// StringPropertyGenerator generates string resource.PropertyValues.
func StringPropertyGenerator() *rapid.Generator {
	return rapid.Custom(func(t *rapid.T) resource.PropertyValue {
		return resource.NewStringProperty(rapid.String().Draw(t, "strings").(string))
	})
}

// TextAssetGenerator generates textual *resource.Asset values.
func TextAssetGenerator() *rapid.Generator {
	return rapid.Custom(func(t *rapid.T) *resource.Asset {
		asset, err := resource.NewTextAsset(rapid.String().Draw(t, "text asset contents").(string))
		require.NoError(t, err)
		return asset
	})
}

// AssetGenerator generates *resource.Asset values.
func AssetGenerator() *rapid.Generator {
	return TextAssetGenerator()
}

// AssetPropertyGenerator generates asset resource.PropertyValues.
func AssetPropertyGenerator() *rapid.Generator {
	return rapid.Custom(func(t *rapid.T) resource.PropertyValue {
		return resource.NewAssetProperty(AssetGenerator().Draw(t, "assets").(*resource.Asset))
	})
}

// LiteralArchiveGenerator generates *resource.Archive values with
// literal archive contents.
func LiteralArchiveGenerator(maxDepth int) *rapid.Generator {
	emptyContentsGen := rapid.Just(map[string]interface{}{})
	return treeGen(treeGenOptions{
		maxHeight: maxDepth,
		gen0: rapid.Custom(func(t *rapid.T) *resource.Archive {
			contents := emptyContentsGen.Draw(t, "literal archive contents").(map[string]interface{})
			archive, err := resource.NewAssetArchive(contents)
			require.NoError(t, err)
			return archive
		}),
		gen1: func(self *rapid.Generator) *rapid.Generator {
			keyGen := rapid.StringMatching(`^(/[^[:cntrl:]/]+)*/?[^[:cntrl:]/]+$`)
			valueGen := rapid.OneOf(AssetGenerator(), self)
			contentsGen := rapid.MapOfN(keyGen, valueGen, 0, 16)
			return rapid.Custom(func(t *rapid.T) *resource.Archive {
				content := contentsGen.Draw(t, "literal archive contents").(map[string]interface{})
				archive, err := resource.NewAssetArchive(content)
				require.NoError(t, err)
				return archive
			})
		},
	})
}

// ArchiveGenerator generates *resource.Archive values.
func ArchiveGenerator(maxDepth int) *rapid.Generator {
	return LiteralArchiveGenerator(maxDepth)
}

// ArchivePropertyGenerator generates archive resource.PropertyValues.
func ArchivePropertyGenerator(maxDepth int) *rapid.Generator {
	g := ArchiveGenerator(maxDepth)
	return rapid.Custom(func(t *rapid.T) resource.PropertyValue {
		return resource.NewArchiveProperty(g.Draw(t, "archives").(*resource.Archive))
	})
}

// ResourceReferenceGenerator generates resource.ResourceReference values.
func ResourceReferenceGenerator() *rapid.Generator {
	return resourceReferenceGenerator(nil)
}

func resourceReferenceGenerator(ctx *StackContext) *rapid.Generator {
	var resourceGenerator *rapid.Generator
	if ctx == nil {
		resourceGenerator = rapid.Custom(func(t *rapid.T) *resource.State {
			id := resource.ID("")
			custom := !rapid.Bool().Draw(t, "component").(bool)
			if custom {
				id = IDGenerator().Draw(t, "resource ID").(resource.ID)
			}

			return &resource.State{
				URN:    URNGenerator().Draw(t, "resource URN").(resource.URN),
				Custom: custom,
				ID:     id,
			}
		})
	} else {
		resourceGenerator = rapid.SampledFrom(ctx.Resources())
	}

	return rapid.Custom(func(t *rapid.T) resource.ResourceReference {
		r := resourceGenerator.Draw(t, "referenced resource").(*resource.State)

		// Only pull the resource's ID if it is a custom resource. Component resources do not have IDs.
		var id resource.PropertyValue
		if r.Custom {
			id = rapid.OneOf(UnknownPropertyGenerator(), rapid.Just(resource.NewStringProperty(string(r.ID)))).Draw(t, "referenced ID").(resource.PropertyValue)
		}

		return resource.ResourceReference{
			URN:            r.URN,
			ID:             id,
			PackageVersion: SemverStringGenerator().Draw(t, "package version").(string),
		}
	})
}

// ResourceReferencePropertyGenerator generates resource reference resource.PropertyValues.
func ResourceReferencePropertyGenerator() *rapid.Generator {
	return resourceReferencePropertyGenerator(nil)
}

func resourceReferencePropertyGenerator(ctx *StackContext) *rapid.Generator {
	return rapid.Custom(func(t *rapid.T) resource.PropertyValue {
		return resource.NewResourceReferenceProperty(resourceReferenceGenerator(ctx).Draw(t, "resource reference").(resource.ResourceReference))
	})
}

// ArrayPropertyGenerator generates array resource.PropertyValues. The
// maxDepth parameter controls the maximum depth of the elements of
// the array.
func ArrayPropertyGenerator(maxDepth int) *rapid.Generator {
	return arrayPropertyGenerator(PropertyValueGenerator(maxDepth - 1))
}

func arrayPropertyGenerator(propGen *rapid.Generator) *rapid.Generator {
	g := rapid.SliceOfN(propGen, 0, 32)
	return rapid.Custom(func(t *rapid.T) resource.PropertyValue {
		return resource.NewArrayProperty(g.Draw(t, "array elements").([]resource.PropertyValue))
	})
}

// PropertyKeyGenerator generates legal resource.PropertyKey values.
func PropertyKeyGenerator() *rapid.Generator {
	return rapid.Custom(func(t *rapid.T) resource.PropertyKey {
		return resource.PropertyKey(rapid.String().Draw(t, "property key").(string))
	})
}

// PropertyMapGenerator generates resource.PropertyMap values. The
// maxDepth parameter controls the maximum depth of the elements of
// the map.
func PropertyMapGenerator(maxDepth int) *rapid.Generator {
	return propertyMapGenerator(PropertyValueGenerator(maxDepth - 1))
}

func propertyMapGenerator(propGen *rapid.Generator) *rapid.Generator {
	g := rapid.MapOfN(PropertyKeyGenerator(), propGen, 0, 32)
	return rapid.Custom(func(t *rapid.T) resource.PropertyMap {
		return resource.PropertyMap(g.Draw(t, "property map").(map[resource.PropertyKey]resource.PropertyValue))
	})
}

// ObjectPropertyGenerator generates object resource.PropertyValues.
// The maxDepth parameter controls the maximum depth of the elements
// of the object.
func ObjectPropertyGenerator(maxDepth int) *rapid.Generator {
	return objectPropertyGenerator(PropertyValueGenerator(maxDepth - 1))
}

func objectPropertyGenerator(propGen *rapid.Generator) *rapid.Generator {
	g := propertyMapGenerator(propGen)
	return rapid.Custom(func(t *rapid.T) resource.PropertyValue {
		return resource.NewObjectProperty(g.Draw(t, "object contents").(resource.PropertyMap))
	})
}

// OutputPropertyGenerator generates output resource.PropertyValues.
// The maxDepth parameter controls the maximum depth of the resolved
// value of the output, if any.
func OutputPropertyGenerator(maxDepth int) *rapid.Generator {
	return outputPropertyGenerator(nil, PropertyValueGenerator(maxDepth-1))
}

func outputPropertyGenerator(ctx *StackContext, propGen *rapid.Generator) *rapid.Generator {
	var urnGenerator *rapid.Generator
	var dependenciesUpperBound int
	if ctx == nil {
		urnGenerator, dependenciesUpperBound = URNGenerator(), 32
	} else {
		urnGenerator = ctx.URNSampler()
		dependenciesUpperBound = len(ctx.Resources())
		if dependenciesUpperBound > 32 {
			dependenciesUpperBound = 32
		}
	}

	urnsGen := rapid.SliceOfN(urnGenerator, 0, dependenciesUpperBound)
	return rapid.Custom(func(t *rapid.T) resource.PropertyValue {
		var element resource.PropertyValue

		known := rapid.Bool().Draw(t, "known").(bool)
		if known {
			element = propGen.Draw(t, "output element").(resource.PropertyValue)
		}

		return resource.NewOutputProperty(resource.Output{
			Element:      element,
			Known:        known,
			Secret:       rapid.Bool().Draw(t, "secret").(bool),
			Dependencies: urnsGen.Draw(t, "dependencies").([]resource.URN),
		})
	})
}

// SecretPropertyGenerator generates secret resource.PropertyValues.
// The maxDepth parameter controls the maximum depth of the plaintext
// value of the secret, if any.
func SecretPropertyGenerator(maxDepth int) *rapid.Generator {
	return secretPropertyGenerator(PropertyValueGenerator(maxDepth - 1))
}

func secretPropertyGenerator(propGen *rapid.Generator) *rapid.Generator {
	return rapid.Custom(func(t *rapid.T) resource.PropertyValue {
		return resource.NewSecretProperty(&resource.Secret{
			Element: propGen.Draw(t, "secret element").(resource.PropertyValue),
		})
	})
}

// PropertyValueGenerator generates arbitrary resource.PropertyValues.
// The maxDepth parameter controls the maximum number of times the
// generator may recur.
func PropertyValueGenerator(maxDepth int) *rapid.Generator {
	return propertyValueGenerator(nil, maxDepth)
}

func propertyValueGenerator(ctx *StackContext, maxDepth int) *rapid.Generator {
	archivePropGen := ArchivePropertyGenerator(maxDepth)
	return treeGen(treeGenOptions{
		maxHeight: maxDepth,
		gen0: rapid.OneOf(
			UnknownPropertyGenerator(),
			NullPropertyGenerator(),
			BoolPropertyGenerator(),
			NumberPropertyGenerator(),
			StringPropertyGenerator(),
			AssetPropertyGenerator(),
		),
		gen1: func(self *rapid.Generator) *rapid.Generator {
			return rapid.OneOf(
				archivePropGen,
				arrayPropertyGenerator(self),
				objectPropertyGenerator(self),
				outputPropertyGenerator(ctx, self),
				secretPropertyGenerator(self),
			)
		},
	})
}

// Generating tree-like structures from the base case and inductive
// case, with max height specified. The implementation carefully takes
// advantage of rapid shrinking `OneOf` to the left: if a property is
// invalid, shrinking will continue to try smaller trees. Also it only
// uses `maxHeight` distinct generator values.
func treeGen(opts treeGenOptions) *rapid.Generator {
	var gens []*rapid.Generator
	g := opts.gen0
	for i := 0; i < opts.maxHeight; i++ {
		gens = append(gens, g)
		g = opts.gen1(g)
	}
	return rapid.OneOf(gens...)
}

type treeGenOptions struct {
	maxHeight int
	gen0      *rapid.Generator
	gen1      func(*rapid.Generator) *rapid.Generator
}
