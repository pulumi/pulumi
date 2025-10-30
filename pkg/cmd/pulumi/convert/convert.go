package convert

import convert "github.com/pulumi/pulumi/sdk/v3/pkg/cmd/pulumi/convert"

func NewConvertCmd(ws pkgWorkspace.Context) *cobra.Command {
	return convert.NewConvertCmd(ws)
}

