package logs

import logs "github.com/pulumi/pulumi/sdk/v3/pkg/cmd/pulumi/logs"

func NewLogsCmd(ws pkgWorkspace.Context) *cobra.Command {
	return logs.NewLogsCmd(ws)
}

