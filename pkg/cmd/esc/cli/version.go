// Copyright 2023, Pulumi Corporation.

package cli

import (
	"fmt"

	"github.com/pulumi/esc/cmd/esc/cli/version"
	"github.com/spf13/cobra"
)

func newVersionCmd(esc *escCommand) *cobra.Command {
	return &cobra.Command{
		Use:          "version",
		Short:        "Print esc's version number",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(esc.stdout, "%v\n", version.Version)
			return nil
		},
	}
}
