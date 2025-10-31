package state

import state "github.com/pulumi/pulumi/sdk/v3/pkg/cmd/pulumi/state"

func NewStateCmd() *cobra.Command {
	return state.NewStateCmd()
}

