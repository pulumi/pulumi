package operations

import operations "github.com/pulumi/pulumi/sdk/v3/pkg/cmd/pulumi/operations"

func NewWatchCmd() *cobra.Command {
	return operations.NewWatchCmd()
}

