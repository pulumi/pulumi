package deployment

import deployment "github.com/pulumi/pulumi/sdk/v3/pkg/cmd/pulumi/deployment"

func NewDeploymentCmd(ws pkgWorkspace.Context) *cobra.Command {
	return deployment.NewDeploymentCmd(ws)
}

