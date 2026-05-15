// Copyright 2026, Pulumi Corporation.

package cli

import (
	"github.com/spf13/cobra"
)

func newEnvReferrerCmd(env *envCommand) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "referrer",
		Short: "Manage environment referrers",
		Long: "Manage environment referrers\n" +
			"\n" +
			"A referrer is an entity that references an environment, such as another environment\n" +
			"that imports it, a Pulumi IaC stack that opens it, or a Pulumi Insights account\n" +
			"that consumes it.\n",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
	}

	cmd.AddCommand(newEnvReferrerListCmd(env))

	return cmd
}
