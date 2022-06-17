package codegen

import "github.com/pulumi/pulumi/pkg/v3/codegen/schema"

type ResourceOptions struct {
	DocComment string
	Properties []*schema.Property
}

// This is the single source of truth for resource options
var PulumiResourceOptions ResourceOptions = ResourceOptions{
	DocComment: "ResourceOptions is a bag of optional settings that control a resources behavior.",
	Properties: []*schema.Property{
		{
			Name:    "parent",
			Comment: "If provided, the currently-constructing resource should be the child of the provided parent resource.",
			Type: &schema.OptionalType{
				ElementType: &schema.ResourceType{
					Token: "pulumi:index:Resource",
				},
			},
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
