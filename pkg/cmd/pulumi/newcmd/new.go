package newcmd

import newcmd "github.com/pulumi/pulumi/sdk/v3/pkg/cmd/pulumi/newcmd"

// NewNewCmd creates a New command with default dependencies.
func NewNewCmd() *cobra.Command {
	return newcmd.NewNewCmd()
}

