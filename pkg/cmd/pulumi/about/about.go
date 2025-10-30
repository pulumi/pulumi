package about

import about "github.com/pulumi/pulumi/sdk/v3/pkg/cmd/pulumi/about"

func NewAboutCmd(ws pkgWorkspace.Context) *cobra.Command {
	return about.NewAboutCmd(ws)
}

