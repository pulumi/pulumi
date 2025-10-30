package schema

import schema "github.com/pulumi/pulumi/sdk/v3/pkg/cmd/pulumi/schema"

func NewSchemaCmd() *cobra.Command {
	return schema.NewSchemaCmd()
}

