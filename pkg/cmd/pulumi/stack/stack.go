package stack

import stack "github.com/pulumi/pulumi/sdk/v3/pkg/cmd/pulumi/stack"

func NewStackCmd() *cobra.Command {
	return stack.NewStackCmd()
}

