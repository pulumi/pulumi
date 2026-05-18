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

package insights

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cloud"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// insightsResourceClient is the subset of cloud-API operations the get command
// needs. Defined inside this package so unit tests can stub it without touching
// the full HTTP client surface.
type insightsResourceClient interface {
	GetInsightsResource(
		ctx context.Context, org, account, resourceTypeAndId string,
	) (apitype.InsightsResourceWithVersion, error)
}

// clientFactory resolves the cloud client and the effective org for the call.
// orgOverride wins when non-empty; otherwise the default org from the cloud
// context is used. A helpful error is returned when the caller is not logged
// in or no org can be determined.
type clientFactory func(
	ctx context.Context, orgOverride string,
) (insightsResourceClient, string, error)

type insightsResourceGetArgs struct {
	org     string
	account string
	output  string
}

type insightsResourceGetCmd struct {
	clientFactory clientFactory
}

// newInsightsResourceGetCmd builds the `pulumi insights resource get` command.
// factory produces the cloud client and resolves the effective org; pass nil to
// use the production factory backed by [cloud.ResolveContext].
func newInsightsResourceGetCmd(factory clientFactory) *cobra.Command {
	if factory == nil {
		factory = defaultClientFactory
	}

	get := &insightsResourceGetCmd{clientFactory: factory}
	var args insightsResourceGetArgs

	cmd := &cobra.Command{
		Use:   "get",
		Short: "[EXPERIMENTAL] Get a single resource discovered by Pulumi Insights",
		Long: "[EXPERIMENTAL] Look up a single resource discovered by Pulumi Insights.\n" +
			"\n" +
			"The positional argument identifies the resource within an Insights account, in\n" +
			"the `<type>::<id>` form described by the Pulumi Cloud REST API (e.g.\n" +
			"`aws:s3/bucket:Bucket::my-bucket`). The account is selected with --account; the\n" +
			"organization defaults to the current default org and can be overridden with\n" +
			"--org.\n" +
			"\n" +
			"Wraps the `ReadResource` Pulumi Cloud REST endpoint.",
		Example: "  # Look up a resource in a specific Insights account.\n" +
			"  pulumi insights resource get --account prod-aws 'aws:s3/bucket:Bucket::my-bucket'\n\n" +
			"  # Override the organization.\n" +
			"  pulumi insights resource get --org acme --account prod-aws \\\n" +
			"      'aws:s3/bucket:Bucket::my-bucket'\n\n" +
			"  # Emit JSON for scripting.\n" +
			"  pulumi insights resource get --account prod-aws \\\n" +
			"      'aws:s3/bucket:Bucket::my-bucket' --output json",
		RunE: func(cmd *cobra.Command, posArgs []string) error {
			return get.Run(cmd.Context(), cmd.OutOrStdout(), posArgs[0], args)
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{{Name: "resource-type-and-id"}},
		Required:  1,
	})

	cmd.Flags().StringVar(&args.org, "org", "",
		"Organization that owns the Insights account (defaults to the current default org)")
	cmd.Flags().StringVar(&args.account, "account", "",
		"Insights account containing the resource")
	cmd.Flags().StringVar(&args.output, "output", "default",
		"Output format. One of: default, json")
	// MarkFlagRequired only errors when the flag isn't defined, which is a
	// programming bug — the immediate StringVar above guarantees it exists.
	_ = cmd.MarkFlagRequired("account")

	return cmd
}

// Run executes the get operation. ctx and out are decoupled from cobra so the
// function is straightforward to drive from tests.
func (c *insightsResourceGetCmd) Run(
	ctx context.Context, out io.Writer, resourceTypeAndID string, args insightsResourceGetArgs,
) error {
	if args.account == "" {
		// `MarkFlagRequired` covers the cobra path, but Run can be called
		// directly from tests; guard so failures here are obvious.
		return errors.New("--account is required")
	}

	// Validate --output before talking to the network so a typo doesn't burn an
	// API call.
	render, err := renderer(args.output)
	if err != nil {
		return err
	}

	client, org, err := c.clientFactory(ctx, args.org)
	if err != nil {
		return err
	}

	resource, err := client.GetInsightsResource(ctx, org, args.account, resourceTypeAndID)
	if err != nil {
		return fmt.Errorf("reading insights resource: %w", err)
	}

	return render(out, resource)
}

// renderer maps --output to the corresponding render function. Returns a
// caller-actionable error for unknown values.
func renderer(format string) (func(io.Writer, apitype.InsightsResourceWithVersion) error, error) {
	switch format {
	case "", "default":
		return renderResourceText, nil
	case "json":
		return renderResourceJSON, nil
	default:
		return nil, fmt.Errorf("invalid --output value %q (must be 'default' or 'json')", format)
	}
}

// renderResourceText writes a human-readable key/value view of the resource to w.
// We use a flat layout rather than the table widget because the response is a
// single record — a table would be visually heavier without adding information.
func renderResourceText(w io.Writer, r apitype.InsightsResourceWithVersion) error {
	fmt.Fprintf(w, "Account:      %s\n", r.Account)
	fmt.Fprintf(w, "Type:         %s\n", r.Type)
	fmt.Fprintf(w, "ID:           %s\n", r.ID)
	fmt.Fprintf(w, "Version:      %d\n", r.Version)
	fmt.Fprintf(w, "Modified:     %s\n", r.Modified.UTC().Format(time.RFC3339))
	if r.PolicyState != "" {
		fmt.Fprintf(w, "Policy state: %s\n", r.PolicyState)
	}
	if len(r.State) > 0 {
		var pretty bytes.Buffer
		// We render `state` as indented JSON rather than a recursive
		// bulleted/key-value tree on purpose:
		//
		//   - `state` is schemaless from the CLI's perspective — its shape
		//     depends on which cloud provider Insights scanned, so we can't
		//     pre-format around known fields.
		//   - JSON preserves type distinctions a tree would muddle (`true`
		//     vs `"true"`, `[]` vs `{}`, null vs missing) and avoids ad-hoc
		//     escaping rules for strings that contain colons or newlines,
		//     both of which are common in cloud state.
		//   - It matches `pulumi stack`'s `stringifyOutput`, which falls
		//     back to JSON for any non-scalar output.
		//
		// On malformed JSON (shouldn't happen — the server promises
		// `application/json`) we fall back to the raw bytes so the user
		// still sees what the service returned.
		if err := json.Indent(&pretty, r.State, "", "  "); err == nil {
			fmt.Fprintf(w, "State:\n%s\n", pretty.String())
		} else {
			fmt.Fprintf(w, "State:        %s\n", string(r.State))
		}
	}
	return nil
}

// renderResourceJSON writes the resource as indented JSON. Indentation matches
// the rest of the cli/cloud commands so jq-style scripting feels consistent.
func renderResourceJSON(w io.Writer, r apitype.InsightsResourceWithVersion) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// defaultClientFactory is the production wiring for clientFactory. It resolves
// the cloud context via cloud.ResolveContext and surfaces the *client.Client
// directly — *client.Client already satisfies insightsResourceClient through
// its GetInsightsResource method.
func defaultClientFactory(
	ctx context.Context, orgOverride string,
) (insightsResourceClient, string, error) {
	resolved, err := cloud.ResolveContext(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("resolving cloud context: %w", err)
	}
	if !resolved.LoggedIn {
		return nil, "", errors.New("not logged in to Pulumi Cloud; run `pulumi login` first")
	}

	org := orgOverride
	if org == "" {
		org = resolved.OrgName
	}
	if org == "" {
		return nil, "", errors.New(
			"no organization available; pass --org or set a default with `pulumi org set-default`")
	}

	return resolved.Client, org, nil
}
