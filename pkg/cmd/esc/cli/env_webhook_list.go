// Copyright 2026, Pulumi Corporation.

package cli

import (
	"context"
	"fmt"
	"io"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"

	"github.com/pulumi/esc/cmd/esc/cli/client"
)

func newEnvWebhookListCmd(env *envCommand) *cobra.Command {
	var count int

	cmd := &cobra.Command{
		Use:     "list [<org-name>/][<project-name>/]<environment-name>",
		Aliases: []string{"ls"},
		Short:   "List environment webhooks.",
		Long: "[EXPERIMENTAL] List environment webhooks\n" +
			"\n" +
			"This command lists the webhooks attached to the given environment.\n",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			if err := env.esc.getCachedClient(ctx); err != nil {
				return err
			}

			ref, _, err := env.getExistingEnvRef(ctx, args)
			if err != nil {
				return err
			}
			if ref.version != "" {
				return fmt.Errorf("the list command does not accept versions")
			}
			if count < 0 {
				return fmt.Errorf("--count must be non-negative")
			}

			hooks, err := env.esc.client.ListEnvironmentWebhooks(ctx, ref.orgName, ref.projectName, ref.envName)
			if err != nil {
				return err
			}

			if count > 0 && len(hooks) > count {
				hooks = hooks[:count]
			}

			printWebhooks(env.esc.stdout, hooks)
			return nil
		},
	}

	cmd.Flags().IntVar(&count, "count", 0, "The maximum number of webhooks to return (all if unset)")

	return cmd
}

func printWebhooks(stdout io.Writer, hooks []client.EnvironmentWebhook) {
	if len(hooks) == 0 {
		return
	}
	rows := make([]cmdutil.TableRow, 0, len(hooks))
	for _, h := range hooks {
		format := h.Format
		if format == "" {
			format = "-"
		}
		rows = append(rows, cmdutil.TableRow{
			Columns: []string{h.Name, h.DisplayName, h.PayloadURL, strconv.FormatBool(h.Active), format},
		})
	}
	_ = cmdutil.FprintTable(stdout, cmdutil.Table{
		Headers: []string{"NAME", "DISPLAY NAME", "URL", "ACTIVE", "FORMAT"},
		Rows:    rows,
	})
}
