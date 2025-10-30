package trace

import trace "github.com/pulumi/pulumi/sdk/v3/pkg/cmd/pulumi/trace"

func NewConvertTraceCmd() *cobra.Command {
	return trace.NewConvertTraceCmd()
}

