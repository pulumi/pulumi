//nolint:lll
package testing

import (
	"github.com/stretchr/testify/assert"
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
func (ctx *StackContext) URNGenerator() *rapid.Generator[resource.URN] {
	return urnGenerator(ctx)
}

// URNSampler samples URNs from the stack's resources.
func (ctx *StackContext) URNSampler() *rapid.Generator[resource.URN] {
	return rapid.Custom(func(t *rapid.T) resource.URN {
		return rapid.SampledFrom(ctx.Resources()).Draw(t, "referenced resource").URN
	})
}

// ResourceReferenceGenerator generates resource.ResourceReference values. The referenced resource is
func (ctx *StackContext) ResourceReferenceGenerator() *rapid.Generator[resource.ResourceReference] {
	if len(ctx.Resources()) == 0 {
		panic("cannot generate resource references: stack context has no resources")
	}
	return resourceReferenceGenerator(ctx)
}

// ResourceReferencePropertyGenerator generates resource reference resource.PropertyValues.
func (ctx *StackContext) ResourceReferencePropertyGenerator() *rapid.Generator[resource.PropertyValue] {
	return resourceReferencePropertyGenerator(ctx)
}

// ArrayPropertyGenerator generates array resource.PropertyValues. The maxDepth parameter controls the maximum
// depth of the elements of the array.
func (ctx *StackContext) ArrayPropertyGenerator(maxDepth int) *rapid.Generator[resource.PropertyValue] {
	return arrayPropertyGenerator(ctx, maxDepth)
}

// PropertyMapGenerator generates resource.PropertyMap values. The maxDepth parameter controls the maximum
// depth of the elements of the map.
func (ctx *StackContext) PropertyMapGenerator(maxDepth int) *rapid.Generator[resource.PropertyMap] {
	return propertyMapGenerator(ctx, maxDepth)
}

// ObjectPropertyGenerator generates object resource.PropertyValues. The maxDepth parameter controls the maximum
// depth of the elements of the object.
func (ctx *StackContext) ObjectPropertyGenerator(maxDepth int) *rapid.Generator[resource.PropertyValue] {
	return objectPropertyGenerator(ctx, maxDepth)
}

// OutputPropertyGenerator generates output resource.PropertyValues. The maxDepth parameter controls the maximum
// depth of the resolved value of the output, if any. The output's dependencies will only refer to resources in
// the context.
func (ctx *StackContext) OutputPropertyGenerator(maxDepth int) *rapid.Generator[resource.PropertyValue] {
	return outputPropertyGenerator(ctx, maxDepth)
}

// SecretPropertyGenerator generates secret resource.PropertyValues. The maxDepth parameter controls the maximum
// depth of the plaintext value of the secret, if any.
func (ctx *StackContext) SecretPropertyGenerator(maxDepth int) *rapid.Generator[resource.PropertyValue] {
	return secretPropertyGenerator(ctx, maxDepth)
}

// PropertyValueGenerator generates arbitrary resource.PropertyValues. The maxDepth parameter controls the maximum
// number of times the generator may recur.
func (ctx *StackContext) PropertyValueGenerator(maxDepth int) *rapid.Generator[resource.PropertyValue] {
	return propertyValueGenerator(ctx, maxDepth)
}

// TypeGenerator generates legal tokens.Type values.
func TypeGenerator() *rapid.Generator[tokens.Type] {
	return rapid.Custom(func(t *rapid.T) tokens.Type {
		return tokens.Type(rapid.StringMatching(`^[a-zA-Z][-a-zA-Z0-9_]*:([^0-9][a-zA-Z0-9._/]*)?:[^0-9][a-zA-Z0-9._/]*$`).Draw(t, "type token"))
	})
}

// URNGenerator generates legal resource.URN values.
func URNGenerator() *rapid.Generator[resource.URN] {
	return urnGenerator(nil)
}

func urnGenerator(ctx *StackContext) *rapid.Generator[resource.URN] {
	var stackNameGenerator, projectNameGenerator *rapid.Generator[string]
	if ctx == nil {
		stackNameGenerator = rapid.StringMatching(`^((:[^:])[^:]*)*:?$`)
		projectNameGenerator = rapid.StringMatching(`^((:[^:])[^:]*)*:?$`)
	} else {
		stackNameGenerator = rapid.Just(ctx.StackName())
		projectNameGenerator = rapid.Just(ctx.ProjectName())
	}

	return rapid.Custom(func(t *rapid.T) resource.URN {
		stackName := tokens.QName(stackNameGenerator.Draw(t, "stack name"))
		projectName := tokens.PackageName(projectNameGenerator.Draw(t, "project name"))
		parentType := TypeGenerator().Draw(t, "parent type")
		resourceType := TypeGenerator().Draw(t, "resource type")
		resourceName := rapid.StringMatching(`^((:[^:])[^:]*)*:?$`).Draw(t, "resource name")
		return resource.NewURN(stackName, projectName, parentType, resourceType, resourceName)
	})
}

// IDGenerator generates legal resource.ID values.
func IDGenerator() *rapid.Generator[resource.ID] {
	return rapid.Custom(func(t *rapid.T) resource.ID {
		return resource.ID(rapid.StringMatching(`..*`).Draw(t, "ids"))
	})
}

// SemverStringGenerator generates legal semver strings.
func SemverStringGenerator() *rapid.Generator[string] {
	return rapid.StringMatching(`^v?(?P<major>0|[1-9]\d*)\.(?P<minor>0|[1-9]\d*)\.(?P<patch>0|[1-9]\d*)(?:-(?P<prerelease>(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+(?P<buildmetadata>[0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`)
}

// UnknownPropertyGenerator generates the unknown resource.PropertyValue.
func UnknownPropertyGenerator() *rapid.Generator[resource.PropertyValue] {
	return rapid.Custom(func(t *rapid.T) resource.PropertyValue {
		return rapid.Just(resource.MakeComputed(resource.NewStringProperty(""))).Draw(t, "unknowns")
	})
}

// NullPropertyGenerator generates the null resource.PropertyValue.
func NullPropertyGenerator() *rapid.Generator[resource.PropertyValue] {
	return rapid.Custom(func(t *rapid.T) resource.PropertyValue {
		return rapid.Just(resource.NewNullProperty()).Draw(t, "nulls")
	})
}

// BoolPropertyGenerator generates boolean resource.PropertyValues.
func BoolPropertyGenerator() *rapid.Generator[resource.PropertyValue] {
	return rapid.Custom(func(t *rapid.T) resource.PropertyValue {
		return resource.NewBoolProperty(rapid.Bool().Draw(t, "booleans"))
	})
}

// NumberPropertyGenerator generates numeric resource.PropertyValues.
func NumberPropertyGenerator() *rapid.Generator[resource.PropertyValue] {
	return rapid.Custom(func(t *rapid.T) resource.PropertyValue {
		return resource.NewNumberProperty(rapid.Float64().Draw(t, "numbers"))
	})
}

// StringPropertyGenerator generates string resource.PropertyValues.
func StringPropertyGenerator() *rapid.Generator[resource.PropertyValue] {
	return rapid.Custom(func(t *rapid.T) resource.PropertyValue {
		return resource.NewStringProperty(rapid.String().Draw(t, "strings"))
	})
}

// TextAssetGenerator generates textual *resource.Asset values.
func TextAssetGenerator() *rapid.Generator[*resource.Asset] {
	return rapid.Custom(func(t *rapid.T) *resource.Asset {
		asset, err := resource.NewTextAsset(rapid.String().Draw(t, "text asset contents"))
		assert.NoError(t, err)
		return asset
	})
}

// AssetGenerator generates *resource.Asset values.
func AssetGenerator() *rapid.Generator[*resource.Asset] {
	return TextAssetGenerator()
}

// AssetPropertyGenerator generates asset resource.PropertyValues.
func AssetPropertyGenerator() *rapid.Generator[resource.PropertyValue] {
	return rapid.Custom(func(t *rapid.T) resource.PropertyValue {
		return resource.NewAssetProperty(AssetGenerator().Draw(t, "assets"))
	})
}

// LiteralArchiveGenerator generates *resource.Archive values with literal archive contents.
func LiteralArchiveGenerator(maxDepth int) *rapid.Generator[*resource.Archive] {
	return rapid.Custom(func(t *rapid.T) *resource.Archive {
		var contentsGenerator *rapid.Generator[map[string]interface{}]
		if maxDepth > 0 {
			contentsGenerator = rapid.MapOfN(
				rapid.StringMatching(`^(/[^[:cntrl:]/]+)*/?[^[:cntrl:]/]+$`),
				rapid.OneOf(AssetGenerator().AsAny(), ArchiveGenerator(maxDepth-1).AsAny()),
				0,  // min length
				16, // max length
			)
		} else {
			contentsGenerator = rapid.Just(map[string]interface{}{})
		}
		archive, err := resource.NewAssetArchive(contentsGenerator.Draw(t, "literal archive contents"))
		assert.NoError(t, err)
		return archive
	})
}

// ArchiveGenerator generates *resource.Archive values.
func ArchiveGenerator(maxDepth int) *rapid.Generator[*resource.Archive] {
	return LiteralArchiveGenerator(maxDepth)
}

// ArchivePropertyGenerator generates archive resource.PropertyValues.
func ArchivePropertyGenerator(maxDepth int) *rapid.Generator[resource.PropertyValue] {
	return rapid.Custom(func(t *rapid.T) resource.PropertyValue {
		return resource.NewArchiveProperty(ArchiveGenerator(maxDepth).Draw(t, "archives"))
	})
}

// ResourceReferenceGenerator generates resource.ResourceReference values.
func ResourceReferenceGenerator() *rapid.Generator[resource.ResourceReference] {
	return resourceReferenceGenerator(nil)
}

func resourceReferenceGenerator(ctx *StackContext) *rapid.Generator[resource.ResourceReference] {
	var resourceGenerator *rapid.Generator[*resource.State]
	if ctx == nil {
		resourceGenerator = rapid.Custom(func(t *rapid.T) *resource.State {
			id := resource.ID("")
			custom := !rapid.Bool().Draw(t, "component")
			if custom {
				id = IDGenerator().Draw(t, "resource ID")
			}

			return &resource.State{
				URN:    URNGenerator().Draw(t, "resource URN"),
				Custom: custom,
				ID:     id,
			}
		})
	} else {
		resourceGenerator = rapid.SampledFrom(ctx.Resources())
	}

	return rapid.Custom(func(t *rapid.T) resource.ResourceReference {
		r := resourceGenerator.Draw(t, "referenced resource")

		// Only pull the resource's ID if it is a custom resource. Component resources do not have IDs.
		var id resource.PropertyValue
		if r.Custom {
			id = rapid.OneOf(
				UnknownPropertyGenerator(),
				rapid.Just(resource.NewStringProperty(string(r.ID))),
			).Draw(t, "referenced ID")
		}

		return resource.ResourceReference{
			URN:            r.URN,
			ID:             id,
			PackageVersion: SemverStringGenerator().Draw(t, "package version"),
		}
	})
}

// ResourceReferencePropertyGenerator generates resource reference resource.PropertyValues.
func ResourceReferencePropertyGenerator() *rapid.Generator[resource.PropertyValue] {
	return resourceReferencePropertyGenerator(nil)
}

func resourceReferencePropertyGenerator(ctx *StackContext) *rapid.Generator[resource.PropertyValue] {
	return rapid.Custom(func(t *rapid.T) resource.PropertyValue {
		return resource.NewResourceReferenceProperty(resourceReferenceGenerator(ctx).Draw(t, "resource reference"))
	})
}

// ArrayPropertyGenerator generates array resource.PropertyValues. The maxDepth parameter controls the maximum
// depth of the elements of the array.
func ArrayPropertyGenerator(maxDepth int) *rapid.Generator[resource.PropertyValue] {
	return arrayPropertyGenerator(nil, maxDepth)
}

func arrayPropertyGenerator(ctx *StackContext, maxDepth int) *rapid.Generator[resource.PropertyValue] {
	return rapid.Custom(func(t *rapid.T) resource.PropertyValue {
		return resource.NewArrayProperty(
			rapid.SliceOfN(propertyValueGenerator(ctx, maxDepth-1), 0, 32).
				Draw(t, "array elements"))
	})
}

// PropertyKeyGenerator generates legal resource.PropertyKey values.
func PropertyKeyGenerator() *rapid.Generator[resource.PropertyKey] {
	return rapid.Custom(func(t *rapid.T) resource.PropertyKey {
		return resource.PropertyKey(rapid.String().Draw(t, "property key"))
	})
}

// PropertyMapGenerator generates resource.PropertyMap values. The maxDepth parameter controls the maximum
// depth of the elements of the map.
func PropertyMapGenerator(maxDepth int) *rapid.Generator[resource.PropertyMap] {
	return propertyMapGenerator(nil, maxDepth)
}

func propertyMapGenerator(ctx *StackContext, maxDepth int) *rapid.Generator[resource.PropertyMap] {
	return rapid.Custom(func(t *rapid.T) resource.PropertyMap {
		return resource.PropertyMap(
			rapid.MapOfN(
				PropertyKeyGenerator(),
				propertyValueGenerator(ctx, maxDepth-1),
				0,  // min length
				32, // max length
			).Draw(t, "property map"))
	})
}

// ObjectPropertyGenerator generates object resource.PropertyValues. The maxDepth parameter controls the maximum
// depth of the elements of the object.
func ObjectPropertyGenerator(maxDepth int) *rapid.Generator[resource.PropertyValue] {
	return objectPropertyGenerator(nil, maxDepth)
}

func objectPropertyGenerator(ctx *StackContext, maxDepth int) *rapid.Generator[resource.PropertyValue] {
	return rapid.Custom(func(t *rapid.T) resource.PropertyValue {
		return resource.NewObjectProperty(propertyMapGenerator(ctx, maxDepth).Draw(t, "object contents"))
	})
}

// OutputPropertyGenerator generates output resource.PropertyValues. The maxDepth parameter controls the maximum
// depth of the resolved value of the output, if any. If a StackContext, the output's dependencies will only refer to
// resources in the context.
func OutputPropertyGenerator(maxDepth int) *rapid.Generator[resource.PropertyValue] {
	return outputPropertyGenerator(nil, maxDepth)
}

func outputPropertyGenerator(ctx *StackContext, maxDepth int) *rapid.Generator[resource.PropertyValue] {
	var urnGenerator *rapid.Generator[resource.URN]
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

	return rapid.Custom(func(t *rapid.T) resource.PropertyValue {
		var element resource.PropertyValue

		known := rapid.Bool().Draw(t, "known")
		if known {
			element = propertyValueGenerator(ctx, maxDepth-1).Draw(t, "output element")
		}

		return resource.NewOutputProperty(resource.Output{
			Element:      element,
			Known:        known,
			Secret:       rapid.Bool().Draw(t, "secret"),
			Dependencies: rapid.SliceOfN(urnGenerator, 0, dependenciesUpperBound).Draw(t, "dependencies"),
		})
	})
}

// SecretPropertyGenerator generates secret resource.PropertyValues. The maxDepth parameter controls the maximum
// depth of the plaintext value of the secret, if any.
func SecretPropertyGenerator(maxDepth int) *rapid.Generator[resource.PropertyValue] {
	return secretPropertyGenerator(nil, maxDepth)
}

func secretPropertyGenerator(ctx *StackContext, maxDepth int) *rapid.Generator[resource.PropertyValue] {
	return rapid.Custom(func(t *rapid.T) resource.PropertyValue {
		return resource.NewSecretProperty(&resource.Secret{
			Element: propertyValueGenerator(ctx, maxDepth-1).Draw(t, "secret element"),
		})
	})
}

// PropertyValueGenerator generates arbitrary resource.PropertyValues. The maxDepth parameter controls the maximum
// number of times the generator may recur.
func PropertyValueGenerator(maxDepth int) *rapid.Generator[resource.PropertyValue] {
	return propertyValueGenerator(nil, maxDepth)
}

func propertyValueGenerator(ctx *StackContext, maxDepth int) *rapid.Generator[resource.PropertyValue] {
	choices := []*rapid.Generator[resource.PropertyValue]{
		UnknownPropertyGenerator(),
		NullPropertyGenerator(),
		BoolPropertyGenerator(),
		NumberPropertyGenerator(),
		StringPropertyGenerator(),
		AssetPropertyGenerator(),
	}

	if ctx == nil || len(ctx.Resources()) > 0 {
		choices = append(choices, resourceReferencePropertyGenerator(ctx))
	}

	if maxDepth > 0 {
		choices = append(choices,
			ArchivePropertyGenerator(maxDepth),
			arrayPropertyGenerator(ctx, maxDepth),
			objectPropertyGenerator(ctx, maxDepth),
			outputPropertyGenerator(ctx, maxDepth),
			secretPropertyGenerator(ctx, maxDepth))
	}
	return rapid.OneOf(choices...)
}
