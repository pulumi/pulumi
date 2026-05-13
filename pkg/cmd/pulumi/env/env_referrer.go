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

package env

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cloud"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

func newEnvReferrerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "referrer",
		Short: "Inspect entities that reference an environment",
		Long:  "[EXPERIMENTAL] Inspect entities that reference an environment.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.AddCommand(newEnvReferrerListCmd())
	return cmd
}

// envReferrerListClient is the slice of the cloud API the list command needs.
// Defined locally so unit tests can stub it without touching the full HTTP
// client surface.
type envReferrerListClient interface {
	ListEnvironmentReferrers(
		ctx context.Context, org, project, env string,
		opts client.ListEnvironmentReferrersOptions,
	) (apitype.ListEnvironmentReferrersResponse, error)
}

// envReferrerListFactory resolves a cloud client and the effective org for the
// call. orgOverride wins when non-empty; otherwise the default org from the
// cloud context is used.
type envReferrerListFactory func(
	ctx context.Context, orgOverride string,
) (envReferrerListClient, string, error)

type envReferrerListArgs struct {
	org                    string
	count                  int
	allRevisions           bool
	latestStackVersionOnly bool
	continuationToken      string
	output                 string
}

func newEnvReferrerListCmd() *cobra.Command {
	return newEnvReferrerListCmdWith(nil)
}

// newEnvReferrerListCmdWith builds the `pulumi env referrer list` command.
// factory produces the cloud client and resolves the effective org; pass nil
// to use the production factory backed by [cloud.ResolveContext].
func newEnvReferrerListCmdWith(factory envReferrerListFactory) *cobra.Command {
	var args envReferrerListArgs

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List entities that reference an environment",
		Long: "[EXPERIMENTAL] List entities that reference an environment.\n" +
			"\n" +
			"Returns environments that import the target environment, Pulumi stacks\n" +
			"whose configuration imports it, and Insights accounts that use it.\n" +
			"Results are paginated; use --count and --continuation-token to page.\n" +
			"\n" +
			"Wraps the `ListEnvironmentReferrers` Pulumi Cloud REST endpoint.",
		Example: "  # List the referrers of an environment.\n" +
			"  pulumi env referrer list my-project my-env\n\n" +
			"  # Page through results.\n" +
			"  pulumi env referrer list my-project my-env --count 50\n\n" +
			"  # Emit JSON for scripting.\n" +
			"  pulumi env referrer list my-project my-env --output json",
		RunE: func(cmd *cobra.Command, posArgs []string) error {
			resolveFactory := factory
			if resolveFactory == nil {
				resolveFactory = defaultEnvReferrerListFactory
			}
			return runEnvReferrerList(cmd.Context(), cmd.OutOrStdout(), resolveFactory,
				posArgs[0], posArgs[1], args)
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{{Name: "project"}, {Name: "name"}},
		Required:  2,
	})

	cmd.Flags().StringVar(&args.org, "org", "",
		"The organization that owns the environment (defaults to the current default org)")
	cmd.Flags().IntVar(&args.count, "count", 0,
		"The maximum number of results to return (1-500)")
	cmd.Flags().BoolVar(&args.allRevisions, "all-revisions", false,
		"Include references across all revisions")
	cmd.Flags().BoolVar(&args.latestStackVersionOnly, "latest-stack-version-only", false,
		"Return only the latest stack version for each referring stack")
	cmd.Flags().StringVar(&args.continuationToken, "continuation-token", "",
		"The continuation token for paginated retrieval")
	cmd.Flags().StringVarP(&args.output, "output", "o", "default",
		"Output format. One of: default, json")

	return cmd
}

// runEnvReferrerList executes the list operation. ctx and out are decoupled
// from cobra so the function is straightforward to drive from tests.
func runEnvReferrerList(
	ctx context.Context, out io.Writer, factory envReferrerListFactory,
	project, name string, args envReferrerListArgs,
) error {
	// Validate --output before talking to the network so a typo doesn't burn
	// an API call.
	render, err := envReferrerListRenderer(args.output)
	if err != nil {
		return err
	}

	c, org, err := factory(ctx, args.org)
	if err != nil {
		return err
	}

	resp, err := c.ListEnvironmentReferrers(ctx, org, project, name,
		client.ListEnvironmentReferrersOptions{
			Count:                  args.count,
			AllRevisions:           args.allRevisions,
			LatestStackVersionOnly: args.latestStackVersionOnly,
			ContinuationToken:      args.continuationToken,
		})
	if err != nil {
		return fmt.Errorf("listing referrers for %s/%s/%s: %w", org, project, name, err)
	}
	return render(out, resp)
}

type envReferrerListRenderFunc func(io.Writer, apitype.ListEnvironmentReferrersResponse) error

// envReferrerListRenderer maps --output to the corresponding render function.
// Returns a caller-actionable error for unknown values.
func envReferrerListRenderer(format string) (envReferrerListRenderFunc, error) {
	switch format {
	case "", "default":
		return renderReferrerListText, nil
	case "json":
		return renderReferrerListJSON, nil
	default:
		return nil, fmt.Errorf(
			"invalid --output value %q (must be 'default' or 'json')", format)
	}
}

// renderReferrerListText writes a human-readable view of the response to w.
// The API returns referrers grouped by revision tag (e.g. "latest") or
// revision number; we emit each group under a heading and walk it in deterministic
// order so the output is stable across runs.
func renderReferrerListText(w io.Writer, r apitype.ListEnvironmentReferrersResponse) error {
	total := 0
	for _, group := range r.Referrers {
		total += len(group)
	}
	if total == 0 {
		fmt.Fprintln(w, "No referrers found for this environment.")
		return nil
	}

	keys := make([]string, 0, len(r.Referrers))
	for k := range r.Referrers {
		keys = append(keys, k)
	}
	// "latest" first, then numeric/lexicographic order, so the most useful
	// group is on top and the rest is stable.
	sort.Slice(keys, func(i, j int) bool {
		if keys[i] == "latest" {
			return true
		}
		if keys[j] == "latest" {
			return false
		}
		return keys[i] < keys[j]
	})

	for i, key := range keys {
		if i > 0 {
			fmt.Fprintln(w)
		}
		fmt.Fprintf(w, "Revision: %s\n", key)
		for _, ref := range r.Referrers[key] {
			switch {
			case ref.Environment != nil:
				e := ref.Environment
				fmt.Fprintf(w, "  environment  %s/%s  rev=%d\n", e.Project, e.Name, e.Revision)
			case ref.Stack != nil:
				s := ref.Stack
				fmt.Fprintf(w, "  stack        %s/%s  ver=%d\n", s.Project, s.Stack, s.Version)
			case ref.InsightsAccount != nil:
				fmt.Fprintf(w, "  insights     %s\n", ref.InsightsAccount.AccountName)
			}
		}
	}
	if r.ContinuationToken != "" {
		fmt.Fprintf(w, "\nNext page: --continuation-token=%s\n", r.ContinuationToken)
	}
	return nil
}

// renderReferrerListJSON writes the response as indented JSON. Indentation
// matches the rest of the cli/cloud commands so jq-style scripting feels
// consistent.
func renderReferrerListJSON(w io.Writer, r apitype.ListEnvironmentReferrersResponse) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// defaultEnvReferrerListFactory is the production wiring for
// envReferrerListFactory. It resolves the cloud context via
// cloud.ResolveContext and surfaces the *client.Client directly —
// *client.Client already satisfies envReferrerListClient through its
// ListEnvironmentReferrers method.
func defaultEnvReferrerListFactory(
	ctx context.Context, orgOverride string,
) (envReferrerListClient, string, error) {
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
