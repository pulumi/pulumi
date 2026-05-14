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

// AI Generated - needs human review

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// policyGroupNewClient is the narrow subset of cloud-API operations the new
// command needs. It both creates the Policy Group and reads it back so the
// command can print the resulting structure (per BEST_PRACTICES §1).
type policyGroupNewClient interface {
	CreatePolicyGroup(ctx context.Context, orgName, name string) error
	GetPolicyGroup(
		ctx context.Context, orgName, policyGroup string,
	) (apitype.GetPolicyGroupResponse, error)
}

// policyGroupNewClientFactory resolves a cloud client and the organization the
// new Policy Group will be created in. orgFlag carries the raw value of
// `--org` (empty means "use the default org").
type policyGroupNewClientFactory func(
	ctx context.Context, orgFlag string,
) (policyGroupNewClient, string, error)

// policyGroupNewArgs collects the flag values for the new command.
type policyGroupNewArgs struct {
	org          string
	outputFormat outputflag.OutputFlag[policyGroupGetRenderFunc]
}

// newPolicyGroupNewCmd builds `pulumi policy group new` with the production
// client factory.
func newPolicyGroupNewCmd() *cobra.Command {
	return newPolicyGroupNewCmdWith(defaultPolicyGroupNewClientFactory)
}

func newPolicyGroupNewCmdWith(factory policyGroupNewClientFactory) *cobra.Command {
	contract.Assertf(factory != nil, "policyGroupNewClientFactory must not be nil")
	var args policyGroupNewArgs
	args.outputFormat = defaultPolicyGroupGetOutputFormat()

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "new <name>",
		Short:  "[EXPERIMENTAL] Create a new Policy Group",
		Long: "[EXPERIMENTAL] Create a new Policy Group.\n" +
			"\n" +
			"Creates a new Policy Group in the given organization. Policy Groups\n" +
			"define which Policy Packs are enforced on which stacks or cloud\n" +
			"accounts, with configurable enforcement levels per pack.\n" +
			"\n" +
			"On success the created group is fetched and rendered. Default output is a\n" +
			"human-readable summary; pass --output=json for the full response as JSON.",
		Example: "  # Create a Policy Group in the default organization\n" +
			"  pulumi policy group new prod-policies\n\n" +
			"  # Create a Policy Group in a specific organization and emit JSON\n" +
			"  pulumi policy group new prod-policies --org acme --output json",
		RunE: func(cmd *cobra.Command, posArgs []string) error {
			return runPolicyGroupNew(cmd.Context(), cmd.OutOrStdout(), factory, posArgs[0], args)
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "name"},
		},
		Required: 1,
	})

	cmd.Flags().StringVar(&args.org, "org", "", "The organization to create the Policy Group in")
	outputflag.VarP(cmd.Flags(), &args.outputFormat)

	return cmd
}

// defaultPolicyGroupNewClientFactory is the production wiring: resolve the
// cloud backend, pick the effective organization, and hand back the underlying
// *client.Client.
func defaultPolicyGroupNewClientFactory(
	ctx context.Context, orgFlag string,
) (policyGroupNewClient, string, error) {
	ws := pkgWorkspace.Instance
	opts := display.Options{Color: cmdutil.GetGlobalColorization()}

	be, err := cmdBackend.CurrentBackend(ctx, ws, cmdBackend.DefaultLoginManager, nil, opts)
	if err != nil {
		return nil, "", err
	}
	cloudBackend, ok := be.(httpstate.Backend)
	if !ok {
		return nil, "", errors.New(
			"creating a Policy Group requires the Pulumi Cloud backend; run `pulumi login`")
	}

	userName, orgs, _, err := cloudBackend.CurrentUser()
	if err != nil {
		return nil, "", err
	}

	org := orgFlag
	if org == "" {
		defaultOrg, err := cloudBackend.GetDefaultOrg(ctx)
		if err != nil {
			return nil, "", err
		}
		org = defaultOrg
	}
	if org == "" {
		org = userName
	}

	if !slices.Contains(orgs, org) && org != userName {
		return nil, "", fmt.Errorf("user %s is not a member of organization %s", userName, org)
	}

	return cloudBackend.Client(), org, nil
}

// runPolicyGroupNew is the cobra-decoupled command body so tests can drive it
// directly without spinning up the flag parser.
func runPolicyGroupNew(
	ctx context.Context, w io.Writer,
	factory policyGroupNewClientFactory, name string, args policyGroupNewArgs,
) error {
	c, org, err := factory(ctx, args.org)
	if err != nil {
		return err
	}

	if err := c.CreatePolicyGroup(ctx, org, name); err != nil {
		return fmt.Errorf("creating policy group: %w", err)
	}

	resp, err := c.GetPolicyGroup(ctx, org, name)
	if err != nil {
		return fmt.Errorf("reading policy group after create: %w", err)
	}

	return args.outputFormat.Get()(w, resp)
}
