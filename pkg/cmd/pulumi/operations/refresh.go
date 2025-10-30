package operations

import operations "github.com/pulumi/pulumi/sdk/v3/pkg/cmd/pulumi/operations"

func NewRefreshCmd() *cobra.Command {
	return operations.NewRefreshCmd()
}

