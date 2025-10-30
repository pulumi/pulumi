package install

import install "github.com/pulumi/pulumi/sdk/v3/pkg/cmd/pulumi/install"

func NewInstallCmd(ws pkgWorkspace.Context) *cobra.Command {
	return install.NewInstallCmd(ws)
}

