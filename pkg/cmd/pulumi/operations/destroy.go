package operations

import operations "github.com/pulumi/pulumi/sdk/v3/pkg/cmd/pulumi/operations"

func NewDestroyCmd() *cobra.Command {
	return operations.NewDestroyCmd()
}

