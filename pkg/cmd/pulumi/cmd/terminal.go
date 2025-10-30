package cmd

import cmd "github.com/pulumi/pulumi/sdk/v3/pkg/cmd/pulumi/cmd"

type OptimalPageSizeOpts = cmd.OptimalPageSizeOpts

// Computes how many options to display in a Terminal UI multi-select.
// Tries to auto-detect and take terminal height into account.
func OptimalPageSize(opts OptimalPageSizeOpts) int {
	return cmd.OptimalPageSize(opts)
}

