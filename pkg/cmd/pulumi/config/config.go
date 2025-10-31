package config

import config "github.com/pulumi/pulumi/sdk/v3/pkg/cmd/pulumi/config"

func NewConfigCmd(ws pkgWorkspace.Context) *cobra.Command {
	return config.NewConfigCmd(ws)
}

