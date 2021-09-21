//nolint:lll
package testing

import (
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

// TypeGenerator generates legal tokens.Type values.
func TypeGenerator() *rapid.Generator {
	return rapid.Custom(func(t *rapid.T) tokens.Type {
		return tokens.Type(rapid.StringMatching(`^[a-zA-Z][-a-zA-Z0-9_]*:([^0-9][a-zA-Z0-9._/]*)?:[^0-9][a-zA-Z0-9._/]*$`).Draw(t, "type token").(string))
	})
}

// URNGenerator generates legal resource.URN values.
func URNGenerator() *rapid.Generator {
	return rapid.Custom(func(t *rapid.T) resource.URN {
		stackName := tokens.QName(rapid.StringMatching(`^((:[^:])[^:]*)*:?$`).Draw(t, "stack name").(string))
		projectName := tokens.PackageName(rapid.StringMatching(`^((:[^:])[^:]*)*:?$`).Draw(t, "project name").(string))
		parentType := TypeGenerator().Draw(t, "parent type").(tokens.Type)
		resourceType := TypeGenerator().Draw(t, "resource type").(tokens.Type)
		resourceName := tokens.QName(rapid.StringMatching(`^((:[^:])[^:]*)*:?$`).Draw(t, "resource name").(string))
		return resource.NewURN(stackName, projectName, parentType, resourceType, resourceName)
	})
}

// IDGenerator generates legal resource.ID values.
func IDGenerator() *rapid.Generator {
	return rapid.Custom(func(t *rapid.T) resource.ID {
		return resource.ID(rapid.String().Draw(t, "ids").(string))
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

// LiteralArchiveGenerator generates *resource.Archive values with literal archive contents.
func LiteralArchiveGenerator(maxDepth int) *rapid.Generator {
	return rapid.Custom(func(t *rapid.T) *resource.Archive {
		var contentsGenerator *rapid.Generator
		if maxDepth > 0 {
			contentsGenerator = rapid.MapOfN(rapid.StringMatching(`^(/[^[:cntrl:]/]+)*/?[^[:cntrl:]/]+$`), rapid.OneOf(AssetGenerator(), ArchiveGenerator(maxDepth-1)), 0, 16)
		} else {
			contentsGenerator = rapid.Just(map[string]interface{}{})
		}
		archive, err := resource.NewAssetArchive(contentsGenerator.Draw(t, "literal archive contents").(map[string]interface{}))
		require.NoError(t, err)
		return archive
	})
}

// ArchiveGenerator generates *resource.Archive values.
func ArchiveGenerator(maxDepth int) *rapid.Generator {
	return LiteralArchiveGenerator(maxDepth)
}

// ArchivePropertyGenerator generates archive resource.PropertyValues.
func ArchivePropertyGenerator(maxDepth int) *rapid.Generator {
	return rapid.Custom(func(t *rapid.T) resource.PropertyValue {
		return resource.NewArchiveProperty(ArchiveGenerator(maxDepth).Draw(t, "archives").(*resource.Archive))
	})
}

// ResourceReferenceGenerator generates resource.ResourceReference values.
func ResourceReferenceGenerator() *rapid.Generator {
	return rapid.Custom(func(t *rapid.T) resource.ResourceReference {
		return resource.ResourceReference{
			URN:            URNGenerator().Draw(t, "referenced URN").(resource.URN),
			ID:             rapid.OneOf(UnknownPropertyGenerator(), StringPropertyGenerator()).Draw(t, "referenced ID").(resource.PropertyValue),
			PackageVersion: SemverStringGenerator().Draw(t, "package version").(string),
		}
	})
}

// ResourceReferencePropertyGenerator generates resource references resource.PropertyValues.
func ResourceReferencePropertyGenerator() *rapid.Generator {
	return rapid.Custom(func(t *rapid.T) resource.PropertyValue {
		return resource.NewResourceReferenceProperty(ResourceReferenceGenerator().Draw(t, "resource reference").(resource.ResourceReference))
	})
}

// ArrayPropertyGenerator generates array resource.PropertyValues. The maxDepth parameter controls the maximum
// depth of the elements of the array.
func ArrayPropertyGenerator(maxDepth int) *rapid.Generator {
	return rapid.Custom(func(t *rapid.T) resource.PropertyValue {
		return resource.NewArrayProperty(rapid.SliceOfN(PropertyValueGenerator(maxDepth-1), 0, 32).Draw(t, "array elements").([]resource.PropertyValue))
	})
}

// PropertyKeyGenerator generates legal resource.PropertyKey values.
func PropertyKeyGenerator() *rapid.Generator {
	return rapid.Custom(func(t *rapid.T) resource.PropertyKey {
		return resource.PropertyKey(rapid.String().Draw(t, "property key").(string))
	})
}

// PropertyMapGenerator generates resource.PropertyMap values. The maxDepth parameter controls the maximum
// depth of the elements of the map.
func PropertyMapGenerator(maxDepth int) *rapid.Generator {
	return rapid.Custom(func(t *rapid.T) resource.PropertyMap {
		return resource.PropertyMap(rapid.MapOfN(PropertyKeyGenerator(), PropertyValueGenerator(maxDepth-1), 0, 32).Draw(t, "property map").(map[resource.PropertyKey]resource.PropertyValue))
	})
}

// ObjectPropertyGenerator generates object resource.PropertyValues. The maxDepth parameter controls the maximum
// depth of the elements of the object.
func ObjectPropertyGenerator(maxDepth int) *rapid.Generator {
	return rapid.Custom(func(t *rapid.T) resource.PropertyValue {
		return resource.NewObjectProperty(PropertyMapGenerator(maxDepth).Draw(t, "object contents").(resource.PropertyMap))
	})
}

// OutputPropertyGenerator generates output resource.PropertyValues. The maxDepth parameter controls the maximum
// depth of the resolved value of the output, if any.
func OutputPropertyGenerator(maxDepth int) *rapid.Generator {
	return rapid.Custom(func(t *rapid.T) resource.PropertyValue {
		var element resource.PropertyValue

		known := rapid.Bool().Draw(t, "known").(bool)
		if known {
			element = PropertyValueGenerator(maxDepth-1).Draw(t, "output element").(resource.PropertyValue)
		}

		return resource.NewOutputProperty(resource.Output{
			Element:      element,
			Known:        known,
			Secret:       rapid.Bool().Draw(t, "secret").(bool),
			Dependencies: rapid.SliceOfN(URNGenerator(), 0, 32).Draw(t, "dependencies").([]resource.URN),
		})
	})
}

// SecretPropertyGenerator generates secret resource.PropertyValues. The maxDepth parameter controls the maximum
// depth of the plaintext value of the secret, if any.
func SecretPropertyGenerator(maxDepth int) *rapid.Generator {
	return rapid.Custom(func(t *rapid.T) resource.PropertyValue {
		return resource.NewSecretProperty(&resource.Secret{
			Element: PropertyValueGenerator(maxDepth-1).Draw(t, "secret element").(resource.PropertyValue),
		})
	})
}

// PropertyValueGenerator generates arbitrary resource.PropertyValues. The maxDepth parameter controls the maximum
// number of times the generator may recur.
func PropertyValueGenerator(maxDepth int) *rapid.Generator {
	choices := []*rapid.Generator{
		UnknownPropertyGenerator(),
		NullPropertyGenerator(),
		BoolPropertyGenerator(),
		NumberPropertyGenerator(),
		StringPropertyGenerator(),
		AssetPropertyGenerator(),
		ResourceReferencePropertyGenerator(),
	}
	if maxDepth > 0 {
		choices = append(choices,
			ArchivePropertyGenerator(maxDepth),
			ArrayPropertyGenerator(maxDepth),
			ObjectPropertyGenerator(maxDepth),
			OutputPropertyGenerator(maxDepth),
			SecretPropertyGenerator(maxDepth))
	}
	return rapid.OneOf(choices...)
}
