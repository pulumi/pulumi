package codegen

import "github.com/pulumi/pulumi/pkg/v3/codegen/schema"

type ResourceOptions struct {
	DocComment string
	Properties []*schema.Property
}

var pulumiResourceType = &schema.ResourceType{Token: "pulumi:index:Resource"}
var pulumiProviderType = &schema.ResourceType{Token: "pulumi:index:ProviderResource"}

// This is the single source of truth for resource options
var PulumiResourceOptions ResourceOptions = ResourceOptions{
	DocComment: "ResourceOptions is a bag of optional settings that control a resources behavior.",
	Properties: []*schema.Property{
		{
			Name:    "parent",
			Comment: "If provided, the currently-constructing resource should be the child of the provided parent resource.",
			Type: &schema.OptionalType{
				ElementType: pulumiResourceType,
			},
		},
		{
			Name:    "dependsOn",
			Comment: "If provided, declares that the currently-constructing resource depends on the given resources.",
			Type: &schema.OptionalType{
				ElementType: &schema.InputType{
					ElementType: &schema.UnionType{
						ElementTypes: []schema.Type{
							pulumiResourceType,
							&schema.InputType{ElementType: pulumiResourceType},
						},
					},
				},
			},
		},
		{
			Name:    "protect",
			Comment: "If provided and True, this resource is not allowed to be deleted.",
			Type:    &schema.OptionalType{ElementType: schema.BoolType},
		},
		{
			Name:    "deleteBeforeReplace",
			Comment: "If provided and True, this resource must be deleted before it is replaced.",
			Type:    &schema.OptionalType{ElementType: schema.BoolType},
		},
		{
			Name: "provider",
			Comment: `An optional provider to use for this resource's CRUD operations. If no provider is supplied, the
default provider for the resource's package will be used. The default provider is pulled from
the parent's provider bag (see also ResourceOptions.providers).`,
			Type: &schema.OptionalType{ElementType: pulumiProviderType},
		},
		{
			Name:    "additionalSecretOutputs",
			Comment: "If provided, a list of output property names that should also be treated as secret.",
			Type: &schema.OptionalType{
				ElementType: &schema.ArrayType{
					ElementType: schema.StringType,
				},
			},
		},
		{
			Name: "pluginDownloadURL",
			Comment: `An optional url. If provided, the engine loads a provider with downloaded
from the provided url. This url overrides the plugin download url inferred from the current package and should
rarely be used.`,
			Type: &schema.OptionalType{ElementType: schema.StringType},
		},
	},
}

type ResourceOptionsGenerator interface {
	GenerateResourceOptions(opts ResourceOptions) ([]byte, error)
}
