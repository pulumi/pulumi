package version

import version "github.com/pulumi/pulumi/sdk/v3/pkg/cmd/pulumi/version"

func NewVersionCmd() *cobra.Command {
	return version.NewVersionCmd()
}

