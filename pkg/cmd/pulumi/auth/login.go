package auth

import auth "github.com/pulumi/pulumi/sdk/v3/pkg/cmd/pulumi/auth"

func NewLoginCmd(ws pkgWorkspace.Context) *cobra.Command {
	return auth.NewLoginCmd(ws)
}

