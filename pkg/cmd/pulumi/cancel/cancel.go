package cancel

import cancel "github.com/pulumi/pulumi/sdk/v3/pkg/cmd/pulumi/cancel"

func NewCancelCmd(ws pkgWorkspace.Context) *cobra.Command {
	return cancel.NewCancelCmd(ws)
}

