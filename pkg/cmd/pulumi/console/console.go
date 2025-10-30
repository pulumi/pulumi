package console

import console "github.com/pulumi/pulumi/sdk/v3/pkg/cmd/pulumi/console"

func NewConsoleCmd(ws pkgWorkspace.Context) *cobra.Command {
	return console.NewConsoleCmd(ws)
}

