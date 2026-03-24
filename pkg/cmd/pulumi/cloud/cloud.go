// Copyright 2026, Pulumi Corporation.  All rights reserved.

package cloud

import (
	"github.com/spf13/cobra"

	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
)

// NewCloudCmd creates a new command group for Pulumi Cloud operations.
func NewCloudCmd(ws pkgWorkspace.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cloud",
		Short: "Manage and query Pulumi Cloud resources",
		Long:  "Commands for querying Pulumi Cloud services beyond stack operations.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newContextCmd(ws))

	return cmd
}
