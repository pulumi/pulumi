package trace

import trace "github.com/pulumi/pulumi/sdk/v3/pkg/cmd/pulumi/trace"

func NewViewTraceCmd() *cobra.Command {
	return trace.NewViewTraceCmd()
}

