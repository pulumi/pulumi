// Copyright 2026, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package policy

import (
	"context"
	"errors"
	"io"
	"os"

	cloudapitype "github.com/pulumi/pulumi-cloud-sdk/go/apitype"
	"github.com/spf13/cobra"

	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
)

// policyIssueExportClient is the narrow subset of cloud-API operations the
// export command needs.
type policyIssueExportClient interface {
	ExportPolicyIssues(
		ctx context.Context, orgName string, req cloudapitype.AngularGridGetRowsRequest, closed bool,
	) (io.ReadCloser, error)
}

type policyIssueExportArgs struct {
	org     string
	file    string
	closed  bool
	filters policyIssueFilters
}

func newPolicyIssueExportCmd(ws pkgWorkspace.Context, lm cmdBackend.LoginManager) *cobra.Command {
	var args policyIssueExportArgs

	cmd := &cobra.Command{
		Use:   "export",
		Short: "[EXPERIMENTAL] Export policy issues as CSV",
		Long: "[EXPERIMENTAL] Export policy issues for an organization as CSV.\n" +
			"\n" +
			"Exports every policy issue matching the given filters, or the closed\n" +
			"issues when --closed is passed. The CSV is produced by Pulumi Cloud and\n" +
			"written as-is to standard output, or to the file given by --file.\n" +
			"\n" +
			"The assignee column of the CSV holds the assignee's email address.",
		Example: "  # Export all active policy issues to a file\n" +
			"  pulumi policy issue export --file issues.csv\n\n" +
			"  # Export open critical issues assigned to you to standard output\n" +
			"  pulumi policy issue export --severity critical --status open --assigned-to <login>",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			if err := validatePolicyIssueFilters(args.filters); err != nil {
				return err
			}
			c, org, err := resolvePolicyIssueCloudClient(ctx, ws, lm, args.org, "exporting policy issues")
			if err != nil {
				return err
			}
			return runPolicyIssueExport(ctx, cmd.OutOrStdout(), c, org, args)
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.Flags().StringVar(&args.org, "org", "", "The organization to export policy issues for")
	cmd.Flags().StringVar(&args.file, "file", "", "Write the CSV to this file instead of standard output")
	cmd.Flags().BoolVar(&args.closed, "closed", false, "Export closed issues instead of active ones")
	addPolicyIssueFilterFlags(cmd.Flags(), &args.filters)

	return cmd
}

func runPolicyIssueExport(
	ctx context.Context, stdout io.Writer, c policyIssueExportClient, org string, args policyIssueExportArgs,
) (err error) {
	filterModel, sortModel, err := buildPolicyIssueGrid(args.filters)
	if err != nil {
		return err
	}

	stream, err := c.ExportPolicyIssues(ctx, org, cloudapitype.AngularGridGetRowsRequest{
		FilterModel: filterModel,
		SortModel:   sortModel,
	}, args.closed)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, stream.Close()) }()

	if args.file == "" {
		_, err = io.Copy(stdout, stream)
		return err
	}

	f, err := os.Create(args.file)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, stream)
	return errors.Join(err, f.Close())
}
