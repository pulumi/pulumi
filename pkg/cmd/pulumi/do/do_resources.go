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

package do

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/hashicorp/hcl/v2"
	hclv2syntax "github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/spf13/cobra"

	hclsyntax "github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	backendSecrets "github.com/pulumi/pulumi/pkg/v3/backend/secrets"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/autonames"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// filterReferencesByPCLUsage returns the subset of refs whose keys appear as top-level identifier
// roots (the leading name in a scope traversal, e.g. `myBucket` in `myBucket.arn`) in the given
// PCL source. It's used to trim the persisted snippet.References map down to only the identifiers
// the snippet's code actually consumes — auto-derived entries that aren't referenced would
// otherwise freeze stale URNs into the snapshot.
//
// On parse failure the refs map is returned unchanged rather than blanked: dropping an identifier
// the engine later needs is a hard failure at update time, so keeping extras is the safer default.
func filterReferencesByPCLUsage(refs map[string]string, src []byte, filename string) map[string]string {
	if len(refs) == 0 || len(src) == 0 {
		return refs
	}
	parser := hclsyntax.NewParser()
	if err := parser.ParseFile(bytes.NewReader(src), filename); err != nil {
		return refs
	}
	if parser.Diagnostics.HasErrors() || len(parser.Files) == 0 {
		return refs
	}

	used := map[string]struct{}{}
	visit := func(node hclv2syntax.Node) hcl.Diagnostics {
		if trav, ok := node.(*hclv2syntax.ScopeTraversalExpr); ok {
			if len(trav.Traversal) > 0 {
				if root, ok := trav.Traversal[0].(hcl.TraverseRoot); ok {
					used[root.Name] = struct{}{}
				}
			}
		}
		return nil
	}
	walkBody(parser.Files[0].Body, visit)

	out := make(map[string]string, len(used))
	for k, v := range refs {
		if _, ok := used[k]; ok {
			out[k] = v
		}
	}
	return out
}

func walkBody(body *hclv2syntax.Body, visit hclv2syntax.VisitFunc) {
	for _, attr := range body.Attributes {
		_ = hclv2syntax.VisitAll(attr.Expr, visit)
	}
	for _, block := range body.Blocks {
		walkBody(block.Body, visit)
	}
}

// newShowResourcesCommand builds the cobra command for `pulumi do show-resources`. It's executed
// as a fresh cobra command from the parent `do` command's RunE (see do.go) because the parent
// runs with DisableFlagParsing, so cobra wouldn't otherwise handle --help or arg validation for
// this subcommand.
func newShowResourcesCommand(ws pkgWorkspace.Context, lm cmdBackend.LoginManager) *cobra.Command {
	output := "text"
	cmd := &cobra.Command{
		Use:   "show-resources",
		Short: "Show the identifiers `pulumi do` will auto-assign to resources in the current stack",
		Long: "Show the identifiers `pulumi do` will auto-assign to resources in the current stack.\n\n" +
			"Each identifier can be used in a --input-file expression in place of the resource's " +
			"URN, in whatever expression syntax the chosen input format supports. Entries in " +
			"--resources-file take precedence over the auto-assigned identifiers shown here.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runShowResources(cmd, ws, lm, output)
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "text", "Output format (supported: text, json)")
	constrictor.AttachArguments(cmd, constrictor.NoArgs)
	// The parent `do` command overrides HelpFunc to run the dynamic-dispatch logic; cobra
	// inherits HelpFunc from the parent when the child doesn't set one, which would drag
	// show-resources' --help through provider loading. Pin it to the default here.
	cmd.SetHelpFunc(cmd.HelpFunc())
	return cmd
}

// runShowResources opens the currently-selected stack, computes the auto-name map and prints it
// (identifier -> URN). This is the discoverability surface for the auto-map that upsert/create
// silently merge into the user's --resources-file.
func runShowResources(cmd *cobra.Command, ws pkgWorkspace.Context, lm cmdBackend.LoginManager, output string) error {
	switch output {
	case "", "default", "text", "json":
	default:
		return fmt.Errorf("unsupported output format %q; supported values are text and json", output)
	}

	ctx := cmd.Context()
	base := diag.DefaultSink(cmd.OutOrStdout(), cmd.ErrOrStderr(), diag.FormatOptions{
		Color: cmdutil.GetGlobalColorization(),
	})
	sink := &forwardingSink{base: base}
	displayOpts := display.Options{Color: cmdutil.GetGlobalColorization()}
	stack, err := cmdStack.RequireStack(
		ctx, sink, ws, lm,
		"", cmdStack.LoadOnly, displayOpts, "",
	)
	if err != nil {
		return fmt.Errorf("load stack: %w", err)
	}
	snap, err := stack.Snapshot(ctx, backendSecrets.DefaultProvider)
	if err != nil {
		return fmt.Errorf("load stack snapshot: %w", err)
	}
	names := autonames.ResourceNames(snap)
	switch output {
	case "", "default", "text":
		return printShowResourcesText(cmd, names)
	case "json":
		return printShowResourcesJSON(cmd, names)
	}
	contract.Failf("unsupported output format %q", output)
	return nil
}

func printShowResourcesText(cmd *cobra.Command, names map[string]string) error {
	if len(names) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "(no resources)")
		return nil
	}
	idents := make([]string, 0, len(names))
	for k := range names {
		idents = append(idents, k)
	}
	sort.Strings(idents)
	rows := make([]cmdutil.TableRow, 0, len(idents))
	for _, k := range idents {
		rows = append(rows, cmdutil.TableRow{Columns: []string{k, names[k]}})
	}
	fmt.Fprint(cmd.OutOrStdout(), cmdutil.Table{
		Headers: []string{"NAME", "URN"},
		Rows:    rows,
	})
	return nil
}

func printShowResourcesJSON(cmd *cobra.Command, names map[string]string) error {
	if names == nil {
		names = map[string]string{}
	}
	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetIndent("", "  ")
	return encoder.Encode(names)
}
