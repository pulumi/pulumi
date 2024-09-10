// Copyright 2024, Pulumi Corporation.

package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/pulumi/esc/cmd/esc/cli/client"
	"github.com/spf13/cobra"
)

func newEnvCloneCmd(env *envCommand) *cobra.Command {
	var (
		preserveHistory         bool
		preserveAccess          bool
		preserveEnvironmentTags bool
		preserveRevisionTags    bool
	)

	cmd := &cobra.Command{
		Use:   "clone [<org-name>/]<src-project-name>/<src-environment-name> [<dest-project-name>/]<dest-environment-name>",
		Args:  cobra.ExactArgs(2),
		Short: "Clone an existing environment into a new environment.",
		Long: "Clone an existing environment into a new environment.\n" +
			"\n" +
			"This command clones an existing environment with the given identifier into a new environment.\n" +
			"If a project is omitted from the new environment identifier the new environment will be created\n" +
			"within the same project as the environment being cloned.\n",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			if err := env.esc.getCachedClient(ctx); err != nil {
				return err
			}

			ref, args, err := env.getExistingEnvRef(ctx, args)
			// An error will arise if an environment reference is ambiguous and can't be
			// resolved to a single environment. The ref for the non-legacy environment will
			// also be returned in this case.
			// If the original ref is using a legacy ID of just env name return an error
			// Otherwise we will ignore any conflict errors and assume the user meant <project-name>/<env-name>
			var ambiguousIdErr ambiguousIdentifierError
			if err != nil && !errors.As(err, &ambiguousIdErr) {
				return err
			}

			if ref.isUsingLegacyID {
				return errors.New("referring to an environment name ('env' or 'org/env') without a project is not supported")
			}

			if ref.version != "" {
				return errors.New("the clone command does not accept versions")
			}

			var destProject string
			destName := args[0]
			if project, name, hasDelimiter := strings.Cut(args[0], "/"); hasDelimiter {
				destProject = project
				destName = name
			}

			destEnv := client.CloneEnvironmentRequest{
				Project:                 destProject,
				Name:                    destName,
				PreserveHistory:         preserveHistory,
				PreserveAccess:          preserveAccess,
				PreserveEnvironmentTags: preserveEnvironmentTags,
				PreserveRevisionTags:    preserveRevisionTags,
			}
			if err := env.esc.client.CloneEnvironment(ctx, ref.orgName, ref.projectName, ref.envName, destEnv); err != nil {
				return fmt.Errorf("cloning environment: %w", err)
			}

			if destProject == "" {
				destProject = ref.projectName
			}
			fmt.Fprintf(
				env.esc.stdout,
				"Environment %s/%s/%s cloned into %s/%s/%s.\n",
				ref.orgName, ref.projectName, ref.envName,
				ref.orgName, destProject, destName,
			)
			return nil
		},
	}

	cmd.Flags().BoolVar(&preserveHistory,
		"preserve-history", false,
		"preserve history of the environment being cloned")

	cmd.Flags().BoolVar(&preserveAccess,
		"preserve-access", false,
		"preserve the same team access on the environment being cloned")

	cmd.Flags().BoolVar(&preserveEnvironmentTags,
		"preserve-env-tags", false,
		"preserve any tags on the environment being cloned")

	cmd.Flags().BoolVar(&preserveRevisionTags,
		"preserve-rev-tags", false,
		"preserve any tags on the environment revisions being cloned")

	return cmd
}
