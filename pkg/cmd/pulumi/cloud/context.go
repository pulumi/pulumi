// Copyright 2026, Pulumi Corporation.  All rights reserved.

package cloud

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func newContextCmd(ws pkgWorkspace.Context) *cobra.Command {
	var orgName string

	cmd := &cobra.Command{
		Use:   "context <urn>",
		Short: "Get assembled context for a resource from Pulumi Cloud",
		Long: `Queries the Pulumi Cloud Context Engine to assemble comprehensive context
for a resource identified by its URN. The response includes resource metadata,
dependency graphs, deployment history, policy violations, team ownership,
cost estimates, ESC environments, source code location, and priority signals.

Accepts both IaC URNs (urn:pulumi:...) and Insights URNs (urn:insights:...).

Examples:
  pulumi cloud context "urn:pulumi:prod::infra::aws:s3/bucket:Bucket::data"
  pulumi cloud context "urn:pulumi:prod::infra::aws:s3/bucket:Bucket::data" --org my-org
  pulumi cloud context "urn:insights:aws-prod::aws::aws:s3/bucket:Bucket::my-bucket-id"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			urn := args[0]

			opts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			// Try to read the current project (may not exist if running outside a Pulumi project).
			project, _, err := ws.ReadProject()
			if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
				return err
			}

			currentBackend, err := cmdBackend.CurrentBackend(
				ctx, ws, cmdBackend.DefaultLoginManager, project, opts,
			)
			if err != nil {
				return err
			}

			cloudBackend, isCloud := currentBackend.(httpstate.Backend)
			if !isCloud {
				return fmt.Errorf("this command requires a Pulumi Cloud backend; run `pulumi login` first")
			}

			// Resolve org name: use --org flag, or fall back to first org from current user.
			if orgName == "" {
				_, orgs, _, err := cloudBackend.CurrentUser()
				if err != nil {
					return fmt.Errorf("failed to get current user: %w", err)
				}
				if len(orgs) == 0 {
					return fmt.Errorf("no organizations found; specify --org explicitly")
				}
				orgName = orgs[0]
			}

			// Call the Context Engine API. We use json.RawMessage so we can
			// pass through the full response without defining all nested types.
			result, err := cloudBackend.Client().GetResourceContext(ctx, orgName, urn)
			if err != nil {
				return fmt.Errorf("failed to get resource context: %w", err)
			}

			// Pretty-print JSON to stdout.
			formatted, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to format response: %w", err)
			}

			fmt.Println(string(formatted))
			return nil
		},
	}

	cmd.Flags().StringVar(&orgName, "org", "", "Pulumi organization name (defaults to first org of current user)")

	return cmd
}
