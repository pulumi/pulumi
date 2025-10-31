package whoami

import whoami "github.com/pulumi/pulumi/sdk/v3/pkg/cmd/pulumi/whoami"

func NewWhoAmICmd(ws pkgWorkspace.Context, lm cmdBackend.LoginManager) *cobra.Command {
	return whoami.NewWhoAmICmd(ws, lm)
}

