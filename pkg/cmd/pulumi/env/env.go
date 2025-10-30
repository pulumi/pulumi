package env

import env "github.com/pulumi/pulumi/sdk/v3/pkg/cmd/pulumi/env"

func NewEnvCmd() *cobra.Command {
	return env.NewEnvCmd()
}

